//go:build noplayback

package main

func initSpeaker() {
	// No-op
}

func (m *model) runInternalPlayback(item songItem) {
	// No-op
}

func (m *model) togglePause() {
	// No-op
}

func (m *model) stopPlayback() {
	// No-op
}
