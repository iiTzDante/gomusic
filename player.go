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

	// Use ffmpeg to transcode the stream URL directly to mp3 and pipe to stdout
	// -re is not needed here as we want to buffer as fast as possible
	cmd := exec.Command("ffmpeg", "-i", streamURL, "-vn", "-c:a", "libmp3lame", "-f", "mp3", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	if err := cmd.Start(); err != nil {
		m.program.Send(errMsg(err))
		return
	}

	streamer, _, err := mp3.Decode(io.NopCloser(stdout))
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}
	defer streamer.Close()

	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
	m.playback.player = ctrl
	m.playback.playingSong = video.Title
	m.playback.isPaused = false

	m.program.Send(playMsg{title: video.Title, author: video.Author})

	done := make(chan bool)
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to finish or be stopped
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
	if ctrl, ok := m.playback.player.(*beep.Ctrl); ok && ctrl != nil {
		ctrl.Paused = true
		m.playback.player = nil
		m.playback.playingSong = ""
	}
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
