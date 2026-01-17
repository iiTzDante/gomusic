package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Types ---

type state int

const (
	stateInput state = iota
	stateSearching
	stateSelecting
	stateDownloading
	stateConverting
	stateFinished
	stateLoading
	statePlaying
	stateError
	stateDownloadingAlbum
	stateViewingAlbumTracks
)

type LyricLine struct {
	Timestamp time.Duration
	Text      string
}

type searchFilter int

const (
	filterAll searchFilter = iota
	filterSongs
	filterAlbums
)

type songItem struct {
	id         string
	title      string
	author     string
	thumb      string
	lyrics     []LyricLine
	isAlbum    bool
	trackCount int // For albums, number of tracks
}

func (i songItem) Title() string {
	if i.isAlbum {
		return "📀 " + i.title
	}
	// For tree view, check if title already has indentation
	if strings.HasPrefix(i.title, "  ") || strings.HasPrefix(i.title, "│  ") {
		return i.title
	}
	return i.title
}
func (i songItem) Description() string {
	if i.isAlbum {
		if i.trackCount > 0 {
			return fmt.Sprintf("%s (Album • %d tracks)", i.author, i.trackCount)
		}
		return i.author + " (Album)"
	}
	return i.author
}
func (i songItem) FilterValue() string { return i.title }

type playbackState struct {
	playingSong       string
	isPaused          bool
	player            any // *beep.Ctrl when !noplayback
	cmd               any // *exec.Cmd to kill the stream
	lyrics            []LyricLine
	currentLyricIndex int
	albumCover        string // ASCII art representation of album cover
	coverPath         string // Path to cached cover image
	kittyImage        string // Kitty graphics protocol sequence for actual image
	resizedCoverPath  string // Path to resized cover for Kitty display
}

type model struct {
	state        state
	textInput    textinput.Model
	list         list.Model
	progress     progress.Model
	spinner      spinner.Model
	err          error
	fileName     string
	quitting     bool
	width        int
	height       int
	selected     songItem
	program      *tea.Program
	searchFilter searchFilter // Current search filter

	// Album download state
	albumTracks   []songItem
	albumProgress struct {
		current int
		total   int
		title   string
	}
	// Album viewing state
	currentAlbum   songItem   // The album being viewed
	albumTrackList list.Model // List of tracks in the album

	// Shared playback state (pointer ensures updates are seen by all receivers)
	playback *playbackState
}

// --- Messages ---

type searchResultsMsg []songItem
type errMsg error
type downloadProgressMsg float64
type convertMsg struct{}
type doneMsg string
type metadataFetchedMsg struct {
	id     string
	title  string
	author string
}
type playMsg struct {
	title  string
	author string
}
type lyricsFetchedMsg []LyricLine
type noLyricsMsg struct{}
type lyricTickMsg time.Time
type stopMsg struct{}
type albumTracksFetchedMsg []songItem
type albumTrackProgressMsg struct {
	current int
	total   int
	title   string
}

type imageReadyMsg struct {
	imagePath string
}
