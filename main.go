package main

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kkdai/youtube/v2"
	"github.com/raitonoberu/ytsearch"
)

const appVersion = "1.0.31"

// --- Styles ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#F456D3")).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

// --- Logic ---

func searchSongs(query string) tea.Cmd {
	return func() tea.Msg {
		search := ytsearch.VideoSearch(query + " audio")
		result, err := search.Next()
		if err != nil {
			return errMsg(err)
		}

		var items []songItem
		for _, v := range result.Videos {
			thumb := ""
			if len(v.Thumbnails) > 0 {
				thumb = v.Thumbnails[0].URL
			}
			items = append(items, songItem{
				id:     v.ID,
				title:  v.Title,
				author: v.Channel.Title,
				thumb:  thumb,
			})
		}
		return searchResultsMsg(items)
	}
}

func (m *model) runDownloadConvert() {
	client := youtube.Client{}
	video, err := client.GetVideo(m.selected.id)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	m.program.Send(metadataFetchedMsg{
		id:     m.selected.id,
		title:  video.Title,
		author: video.Author,
	})

	formats := video.Formats.Type("audio")
	if len(formats) == 0 {
		m.program.Send(errMsg(fmt.Errorf("no audio format found")))
		return
	}
	format := &formats[0]

	tempAudio := "temp_audio"
	tempThumb := "temp_thumb.jpg"
	finalName := strings.ReplaceAll(video.Title, "/", "_") + ".mp3"

	err = m.downloadFile(client, format, video, tempAudio, func(p float64) {
		m.program.Send(downloadProgressMsg(p))
	})
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	m.program.Send(convertMsg{})
	err = m.downloadThumb(m.selected.thumb, tempThumb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading thumb: %v\n", err)
	}

	args := []string{
		"-y",
		"-i", tempAudio,
		"-i", tempThumb,
		"-map", "0:0",
		"-map", "1:0",
		"-c:a", "libmp3lame",
		"-q:a", "2",
		"-id3v2_version", "3",
		"-metadata:s:v", "title=\"Album cover\"",
		"-metadata:s:v", "comment=\"Cover (Front)\"",
		"-metadata", "title=" + video.Title,
		"-metadata", "artist=" + video.Author,
		finalName,
	}

	cmd := exec.Command("ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		m.program.Send(errMsg(fmt.Errorf("FFmpeg failed: %v", err)))
		return
	}

	os.Remove(tempAudio)
	os.Remove(tempThumb)

	m.program.Send(doneMsg(finalName))
}

func (m *model) downloadFile(client youtube.Client, format *youtube.Format, video *youtube.Video, path string, onProgress func(float64)) error {
	stream, size, err := client.GetStream(video, format)
	if err != nil {
		return err
	}
	defer stream.Close()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			file.Write(buf[:n])
			downloaded += int64(n)
			if size > 0 {
				onProgress(float64(downloaded) / float64(size))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *model) downloadThumb(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

// --- Bubble Tea Methods ---

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "q":
			if m.state == statePlaying {
				m.stopPlayback()
				m.state = stateSelecting
				m.list.ResetSelected()
				return m, nil
			}
			if m.state == stateSelecting {
				m.state = stateInput
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if m.state == stateInput {
				m.state = stateSearching
				return m, tea.Batch(m.spinner.Tick, searchSongs(m.textInput.Value()))
			}
			if m.state == stateSelecting {
				item, ok := m.list.SelectedItem().(songItem)
				if ok {
					m.selected = item
					m.state = stateDownloading
					go m.runDownloadConvert()
					return m, nil
				}
			}
		case "p":
			if m.state == stateSelecting {
				item, ok := m.list.SelectedItem().(songItem)
				if ok {
					m.stopPlayback() // Cleanup any existing playback first
					m.selected = item
					m.state = stateLoading
					go m.runInternalPlayback(item)
					return m, m.spinner.Tick
				}
			}
		case " ":
			if m.state == statePlaying {
				m.togglePause()
				return m, nil
			}
		case "s":
			if m.state == statePlaying {
				m.stopPlayback()
				return m, nil
			}
		case "esc":
			if m.state == stateSelecting {
				m.state = stateInput
				return m, nil
			}
		case "right":
			if m.state == statePlaying {
				m.seekForward()
			}
		case "left":
			if m.state == statePlaying {
				m.seekBackward()
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case lyricTickMsg:
		if m.state == statePlaying {
			m.updateLyrics()
			return m, tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
				return lyricTickMsg(t)
			})
		}
		return m, nil

	case searchResultsMsg:
		m.state = stateSelecting
		var items []list.Item
		for _, v := range msg {
			items = append(items, v)
		}
		m.list = list.New(items, list.NewDefaultDelegate(), m.width-4, m.height-8)
		m.list.Title = "Select Song"
		return m, nil

	case errMsg:
		m.err = msg
		m.state = stateError
		return m, nil

	case metadataFetchedMsg:
		if m.selected.id == msg.id {
			m.selected.title = msg.title
			m.selected.author = msg.author
		}
		return m, nil

	case downloadProgressMsg:
		cmd := m.progress.SetPercent(float64(msg))
		return m, cmd

	case convertMsg:
		m.state = stateConverting
		return m, nil

	case doneMsg:
		m.fileName = string(msg)
		m.state = stateFinished
		return m, tea.Batch(
			tea.Printf("\n  %s %s\n", statusStyle.Render("Saved:"), m.fileName),
			tea.Quit,
		)

	case playMsg:
		m.playback.playingSong = fmt.Sprintf("%s - %s", msg.title, msg.author)
		m.state = statePlaying
		return m, tea.Batch(
			m.spinner.Tick,
			tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
				return lyricTickMsg(t)
			}),
		)

	case lyricsFetchedMsg:
		m.playback.lyrics = msg
		return m, nil

	case noLyricsMsg:
		m.playback.lyrics = []LyricLine{{Timestamp: 0, Text: "[No synced lyrics found]"}}
		return m, nil

	case stopMsg:
		m.state = stateSelecting
		m.list.ResetSelected()
		return m, nil

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if m2, ok := newModel.(progress.Model); ok {
			m.progress = m2
		}
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateSelecting {
			m.list.SetSize(msg.Width-4, msg.Height-8)
		}
		m.progress.Width = msg.Width - 4
	}

	if m.state == stateInput {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	if m.state == stateSelecting {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "\n  Goodbye! 🎧\n\n"
	}

	var s string

	switch m.state {
	case stateInput:
		s = fmt.Sprintf("\n  %s\n\n  %s\n\n  %s",
			titleStyle.Render("GoMusic Search"),
			m.textInput.View(),
			helpStyle.Render("Enter song name, artist, or album"),
		)
	case stateSearching:
		s = fmt.Sprintf("\n  %s Searching YouTube Music...\n", m.spinner.View())
	case stateSelecting:
		return docStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				m.list.View(),
				helpStyle.Render("\n  ENTER: Download & Convert  •  P: Play Integrated  •  Q: Quit"),
			),
		)
	case stateDownloading:
		s = fmt.Sprintf("\n  %s\n\n  %s\n\n  %s",
			titleStyle.Render("Downloading: "+m.selected.title),
			m.progress.View(),
			helpStyle.Render("Selected: "+m.selected.author),
		)
	case stateConverting:
		s = fmt.Sprintf("\n  %s %s\n\n  %s",
			m.spinner.View(),
			titleStyle.Render("Encoding & Tagging..."),
			helpStyle.Render("Using FFmpeg to embed cover art and ID3 tags"),
		)
	case stateFinished:
		s = fmt.Sprintf("\n  %s\n", titleStyle.Render("Success! Enjoy your music."))
	case stateLoading:
		s = fmt.Sprintf("\n  %s %s\n", m.spinner.View(), titleStyle.Render("Preparing stream..."))
	case statePlaying:
		// Simple animated wave visualizer
		t := float64(time.Now().UnixNano()/1e7) / 10.0
		wave := ""
		width := 60
		chars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
		for i := 0; i < width; i++ {
			p := float64(i) / float64(width)
			h := math.Sin(p*math.Pi*2+t)*2.5 + math.Sin(p*math.Pi*4+t*1.5)*1.5 + math.Sin(p*math.Pi*8+t*2.5)*0.8
			if m.playback.isPaused {
				h = 0
			}
			idx := int((h + 4.8) / 9.6 * 8)
			if idx < 0 {
				idx = 0
			}
			if idx > 7 {
				idx = 7
			}
			wave += chars[idx]
		}
		s = fmt.Sprintf(
			"\n  %s\n\n  %s\n\n%s\n\n  %s\n\n  %s",
			titleStyle.Render("Now Playing"),
			statusStyle.Render(m.playback.playingSong),
			m.renderLyrics(),
			lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render(wave),
			helpStyle.Render("SPACE: Play/Pause  •  S: Stop  •  Q: Exit"),
		)
	case stateError:
		s = fmt.Sprintf("\n  %s\n\n  %v\n",
			errorStyle.Render("Error"),
			m.err,
		)
	}

	return s
}

func (m *model) updateLyrics() {
	if len(m.playback.lyrics) == 0 {
		return
	}

	currentTime, ok := m.getCurrentPlaybackPosition()
	if !ok {
		return
	}

	// Find the current lyric index
	newIdx := -1
	for i, l := range m.playback.lyrics {
		if l.Timestamp <= currentTime {
			newIdx = i
		} else {
			break
		}
	}
	m.playback.currentLyricIndex = newIdx
}

func (m *model) renderLyrics() string {
	if m.playback.lyrics == nil {
		if m.playback.playingSong != "" {
			return "\n  " + helpStyle.Render("Searching for lyrics...")
		}
		return ""
	}

	if len(m.playback.lyrics) == 1 && m.playback.lyrics[0].Text == "[No synced lyrics found]" {
		return "\n  " + helpStyle.Render("No synced lyrics found for this track.")
	}

	idx := m.playback.currentLyricIndex
	var lines []string

	// If no lyrics have started yet or no lyrics found
	if idx < 0 || len(m.playback.lyrics) == 0 {
		return ""
	}

	// If we've finished all lyrics, keep showing the last few lines
	if idx >= len(m.playback.lyrics) {
		idx = len(m.playback.lyrics) - 1
	}

	// Show 3 lines: previous, current (highlighted), next
	for i := idx - 1; i <= idx+1; i++ {
		if i < 0 || i >= len(m.playback.lyrics) {
			lines = append(lines, "")
			continue
		}

		text := m.playback.lyrics[i].Text
		if i == idx {
			lines = append(lines, "  "+lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FFFF")).
				Bold(true).
				Render("> "+text))
		} else {
			lines = append(lines, "    "+helpStyle.Render(text))
		}
	}

	return strings.Join(lines, "\n")
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-v" {
		fmt.Printf("gomusic version %s\n", appVersion)
		return
	}

	ti := textinput.New()
	ti.Placeholder = "Song title..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	p := progress.New(progress.WithDefaultGradient())

	m := &model{
		state:     stateInput,
		textInput: ti,
		spinner:   s,
		progress:  p,
		playback:  &playbackState{},
	}

	program := tea.NewProgram(m)
	m.program = program

	initSpeaker()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running GoMusic: %v\n", err)
		os.Exit(1)
	}
}
