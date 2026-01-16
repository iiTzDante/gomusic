//go:build !noplayback

package main

import (
	"fmt"
	"io"
	"os"
	"time"

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

	stream, _, err := client.GetStream(video, format)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	tmp, err := os.CreateTemp("", "gomusic-stream-*.mp3")
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}
	defer os.Remove(tmp.Name())

	_, err = io.Copy(tmp, stream)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}
	tmp.Seek(0, 0)

	streamer, _, err := mp3.Decode(tmp)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}
	defer streamer.Close()

	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
	m.player = ctrl
	m.program.Send(playMsg{title: video.Title, author: video.Author})

	done := make(chan bool)
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		done <- true
	})))

	<-done
	m.program.Send(stopMsg{})
}

func (m *model) togglePause() {
	if ctrl, ok := m.player.(*beep.Ctrl); ok && ctrl != nil {
		m.isPaused = !m.isPaused
		ctrl.Paused = m.isPaused
	}
}

func (m *model) stopPlayback() {
	if ctrl, ok := m.player.(*beep.Ctrl); ok && ctrl != nil {
		ctrl.Paused = true
		m.player = nil
	}
}
