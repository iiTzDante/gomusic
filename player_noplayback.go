//go:build noplayback

package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Stub implementations for noplayback builds
func initSpeaker() {
	// No-op for noplayback builds
}

func (m *model) runInternalPlayback(item songItem) {
	// For noplayback builds, just show a message and process album cover
	m.playback.playingSong = fmt.Sprintf("%s - %s", item.title, item.author)
	m.playback.isPaused = false
	m.playback.lyrics = nil
	m.playback.currentLyricIndex = -1
	m.playback.albumCover = ""
	m.playback.coverPath = ""
	m.playback.kittyImage = ""
	m.playback.resizedCoverPath = ""

	m.program.Send(playMsg{title: item.title, author: item.author})

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
}

func (m *model) togglePause() {
	// No-op for noplayback builds
	m.playback.isPaused = !m.playback.isPaused
}

func (m *model) stopPlayback() {
	// Clear images from terminal
	clearKittyImages()
	
	// Clean up cover files
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
	// No-op for noplayback builds
}

func (m *model) seekBackward() {
	// No-op for noplayback builds
}

func (m *model) getCurrentPlaybackPosition() (time.Duration, bool) {
	// No-op for noplayback builds - always return false
	return 0, false
}
