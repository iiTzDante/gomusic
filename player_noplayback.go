//go:build noplayback

package main

import (
	"time"
)

// Stub implementations for noplayback builds
func initSpeaker() {
	// No-op for noplayback builds
}

func (m *model) runInternalPlayback(item songItem) {
	// No-op for noplayback builds
	m.program.Send(errMsg(nil))
}

func (m *model) togglePause() {
	// No-op for noplayback builds
}

func (m *model) stopPlayback() {
	// No-op for noplayback builds
}

func (m *model) seekForward() {
	// No-op for noplayback builds
}

func (m *model) seekBackward() {
	// No-op for noplayback builds
}

func (m *model) getCurrentPlaybackPosition() (time.Duration, bool) {
	// No-op for noplayback builds - always return false
	return 0, false
}
