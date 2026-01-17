//go:build !noplayback

package main

import (
	"fmt"
	"io"
	"os"
	"sync"
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
	// Validate track ID before attempting playback
	if item.id == "" || len(item.id) < 10 {
		m.program.Send(errMsg(fmt.Errorf("cannot play this track - invalid track ID")))
		return
	}

	client := youtube.Client{}
	track, err := client.GetVideo(item.id) // GetVideo works for music tracks
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	formats := track.Formats.Type("audio")
	if len(formats) == 0 {
		m.program.Send(errMsg(fmt.Errorf("no audio format found")))
		return
	}
	format := &formats[0]

	streamURL, err := client.GetStreamURL(track, format)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	// Use reconnect flags to handle network fluctuations
	// Add user agent to prevent YouTube from throttling or closing the connection
	cmd := exec.Command("ffmpeg",
		"-user_agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"-reconnect", "1",
		"-reconnect_at_eof", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-probesize", "5000000",
		"-analyzeduration", "5000000",
		"-i", streamURL,
		"-loglevel", "error",
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

	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
	m.playback.player = ctrl
	m.playback.playingSong = track.Title
	m.playback.isPaused = false
	m.playback.lyrics = nil
	m.playback.currentLyricIndex = -1
	m.playback.albumCover = ""
	m.playback.coverPath = ""
	m.playback.kittyImage = ""
	m.playback.resizedCoverPath = ""

	m.program.Send(playMsg{title: track.Title, author: track.Author})

	// Use WaitGroup to fetch image and lyrics concurrently
	var wg sync.WaitGroup
	
	// Fetch album cover in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		if item.thumb != "" {
			coverPath := fmt.Sprintf("temp_cover_%s.jpg", item.id)
			err := m.downloadAndCacheThumb(item.thumb, coverPath)
			if err == nil {
				// Always generate ASCII art for stable display
				asciiArt := convertImageToASCII(coverPath, 40, 20) // Large colorized ASCII art
				if asciiArt != "" {
					m.playback.albumCover = asciiArt
					m.playback.coverPath = coverPath
				}
				
				// Also try terminal image display if supported
				if isImageCapableTerminal() {
					// Resize image for better display (200x200 pixels max)
					resizedPath := fmt.Sprintf("temp_cover_resized_%s.jpg", item.id)
					err := resizeImage(coverPath, resizedPath, 200, 200)
					if err == nil {
						// Store paths and notify TUI that image is ready
						m.playback.resizedCoverPath = resizedPath
						m.playback.kittyImage = "ready" // Signal that image is ready
						m.program.Send(imageReadyMsg{imagePath: resizedPath})
					}
				}
			}
		}
	}()

	// Fetch lyrics in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		durSeconds := int(track.Duration.Seconds())
		lyrics, err := fetchLyrics(track.Title, track.Author, durSeconds)
		if err != nil || len(lyrics) == 0 {
			m.program.Send(noLyricsMsg{})
		} else {
			m.program.Send(lyricsFetchedMsg(lyrics))
		}
	}()

	// Don't wait for image/lyrics to complete - let them load in background

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
	
	// 3. Clear images from terminal
	clearKittyImages()
	
	// 4. Clean up cover files
	if m.playback.coverPath != "" {
		os.Remove(m.playback.coverPath)
		m.playback.coverPath = ""
	}
	if m.playback.resizedCoverPath != "" {
		os.Remove(m.playback.resizedCoverPath)
		m.playback.resizedCoverPath = ""
	}
	
	m.playback.playingSong = ""
	m.playback.albumCover = ""
	m.playback.kittyImage = ""
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
