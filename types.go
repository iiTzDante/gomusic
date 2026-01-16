package main

import (
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
	statePlaying
	stateError
)

type songItem struct {
	id     string
	title  string
	author string
	thumb  string
}

func (i songItem) Title() string       { return i.title }
func (i songItem) Description() string { return i.author }
func (i songItem) FilterValue() string { return i.title }

type model struct {
	state     state
	textInput textinput.Model
	list      list.Model
	progress  progress.Model
	spinner   spinner.Model
	err       error
	fileName  string
	quitting  bool
	width     int
	height    int
	selected  songItem
	program   *tea.Program

	// Player state
	playingSong string
	isPaused    bool
	player      any // *beep.Ctrl when !noplayback
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
type stopMsg struct{}
