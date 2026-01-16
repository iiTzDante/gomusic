//go:build !noplayback

package main

import (
	"fmt"
	"io"
	"time"

	"os/exec"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/kkdai/youtube/v2"
)

func initSpeaker() {
	sr := beep.SampleRate(44100)
	speaker.Init(sr, sr.N(time.Second/10))
}

func (m *model) runInternalPlayback(item songItem) {
	client := youtube.Client{}
	video, err := client.GetVideo(item.id)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	formats := video.Formats.Type("audio")
	if len(formats) == 0 {
		m.program.Send(errMsg(fmt.Errorf("no audio format found")))
		return
	}
	format := &formats[0]

	streamURL, err := client.GetStreamURL(video, format)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	// Force 44.1kHz stereo to match the speaker exactly and avoid pitch/speed issues
	// Use -probesize and -analyzeduration to minimize start delay
	cmd := exec.Command("ffmpeg",
		"-probesize", "32",
		"-analyzeduration", "0",
		"-i", streamURL,
		"-vn", "-c:a", "libmp3lame",
		"-ar", "44100",
		"-ac", "2",
		"-f", "mp3",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	if err := cmd.Start(); err != nil {
		m.program.Send(errMsg(err))
		return
	}

	// Store cmd so we can kill it
	m.playback.cmd = cmd

	streamer, _, err := mp3.Decode(io.NopCloser(stdout))
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}
	defer streamer.Close()

	// Fetch lyrics in background
	go func() {
		durSeconds := int(video.Duration.Seconds())
		lyrics, err := fetchLyrics(video.Title, video.Author, durSeconds)
		if err != nil || len(lyrics) == 0 {
			m.program.Send(noLyricsMsg{})
		} else {
			m.program.Send(lyricsFetchedMsg(lyrics))
		}
	}()

	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
	m.playback.player = ctrl
	m.playback.playingSong = video.Title
	m.playback.isPaused = false
	m.playback.lyrics = nil
	m.playback.currentLyricIndex = -1

	m.program.Send(playMsg{title: video.Title, author: video.Author})

	done := make(chan bool)
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to finish or the process to be killed
	go func() {
		cmd.Wait()
	}()

	<-done
	m.program.Send(stopMsg{})
}

func (m *model) togglePause() {
	if ctrl, ok := m.playback.player.(*beep.Ctrl); ok && ctrl != nil {
		m.playback.isPaused = !m.playback.isPaused
		ctrl.Paused = m.playback.isPaused
	}
}

func (m *model) stopPlayback() {
	// 1. Kill the ffmpeg process first
	if cmd, ok := m.playback.cmd.(*exec.Cmd); ok && cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		m.playback.cmd = nil
	}

	// 2. Stop the audio engine
	if ctrl, ok := m.playback.player.(*beep.Ctrl); ok && ctrl != nil {
		ctrl.Paused = true
		m.playback.player = nil
	}
	m.playback.playingSong = ""
}

func (m *model) seekForward() {
	if ctrl, ok := m.playback.player.(*beep.Ctrl); ok && ctrl != nil {
		if seeker, ok := ctrl.Streamer.(beep.StreamSeeker); ok {
			speaker.Lock()
			newPos := seeker.Position() + 5*44100
			if newPos >= seeker.Len() {
				newPos = seeker.Len() - 1
			}
			seeker.Seek(newPos)
			speaker.Unlock()
		}
	}
}

func (m *model) seekBackward() {
	if ctrl, ok := m.playback.player.(*beep.Ctrl); ok && ctrl != nil {
		if seeker, ok := ctrl.Streamer.(beep.StreamSeeker); ok {
			speaker.Lock()
			newPos := seeker.Position() - 5*44100
			if newPos < 0 {
				newPos = 0
			}
			seeker.Seek(newPos)
			speaker.Unlock()
		}
	}
}

// Get current playback position for lyrics synchronization
func (m *model) getCurrentPlaybackPosition() (time.Duration, bool) {
	if m.playback.player == nil {
		return 0, false
	}

	ctrl, ok := m.playback.player.(*beep.Ctrl)
	if !ok || ctrl == nil {
		return 0, false
	}

	seeker, ok := ctrl.Streamer.(beep.StreamSeeker)
	if !ok {
		return 0, false
	}

	// Use speaker lock to safely read position without interfering with playback
	speaker.Lock()
	pos := seeker.Position()
	speaker.Unlock()

	currentTime := time.Duration(float64(pos) / 44100.0 * float64(time.Second))
	return currentTime, true
}
