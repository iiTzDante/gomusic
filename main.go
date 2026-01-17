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

func searchSongs(query string, filter searchFilter) tea.Cmd {
	return func() tea.Msg {
		var items []songItem

		// Search for videos (songs) if filter allows
		if filter == filterAll || filter == filterSongs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Silently recover from panics in video search
					}
				}()
				// Prioritize music searches - try music-specific queries first
				searchQueries := []string{
					query + " music",
					query + " song",
					query + " audio",
					query,
				}
				
				for _, searchQuery := range searchQueries {
					search := ytsearch.VideoSearch(searchQuery)
					result, err := search.Next()
					if err == nil && len(result.Videos) > 0 {
						for _, v := range result.Videos {
							// Prefer music-related results
							titleLower := strings.ToLower(v.Title)
							
							// Skip if it's clearly not music (optional filter)
							if strings.Contains(titleLower, "tutorial") || 
							   strings.Contains(titleLower, "how to") ||
							   strings.Contains(titleLower, "review") {
								continue
							}
							
							thumb := ""
							if len(v.Thumbnails) > 0 {
								thumb = v.Thumbnails[0].URL
							}
							items = append(items, songItem{
								id:        v.ID,
								title:     v.Title,
								author:    v.Channel.Title,
								thumb:     thumb,
								isAlbum:   false,
								trackCount: 0,
							})
						}
						// If we found results, don't try other queries
						if len(items) > 0 {
							break
						}
					}
				}
			}()
		}

		// Search for playlists (albums) if filter allows
		if filter == filterAll || filter == filterAlbums {
			// Try multiple query variations to find playlists
			// Prioritize music-specific searches to target YouTube Music content
			queryVariations := []string{
				query + " music album",
				query + " album music",
				query + " music playlist",
				query + " official album",
				query + " full album music",
				query + " album",
				query + " playlist",
				query + " full album",
			}
			
			seenPlaylistIDs := make(map[string]bool)
			foundPlaylists := false
			
			for _, searchQuery := range queryVariations {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Silently recover from panics in playlist search
						}
					}()
					playlistSearch := ytsearch.PlaylistSearch(searchQuery)
					playlistResult, err := playlistSearch.Next()
					if err == nil && playlistResult != nil && len(playlistResult.Playlists) > 0 {
						foundPlaylists = true
						for _, p := range playlistResult.Playlists {
							// Validate playlist ID - must start with "PL" or be a valid playlist format
							// Video IDs are typically 11 chars and don't start with PL
							if !isValidPlaylistID(p.ID) {
								continue // Skip invalid playlist IDs
							}
							
							// Skip if we've already added this playlist
							if seenPlaylistIDs[p.ID] {
								continue
							}
							seenPlaylistIDs[p.ID] = true
							
							thumb := ""
							if len(p.Thumbnails) > 0 {
								thumb = p.Thumbnails[0].URL
							}
							items = append(items, songItem{
								id:         p.ID,
								title:      p.Title,
								author:     p.Channel.Title,
								thumb:      thumb,
								isAlbum:    true,
								trackCount: p.VideoCount,
							})
						}
					}
				}()
				
				// If we found some results, don't try more variations
				if foundPlaylists {
					break
				}
			}
			
			// Fallback: If no playlists found, search for videos with "full album" or "playlist" in title
			// and treat them as potential albums (they might be playlist links)
			if !foundPlaylists {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Silently recover from panics
						}
					}()
					// Search for videos that might be full albums
					albumVideoSearch := ytsearch.VideoSearch(query + " full album")
					albumVideoResult, err := albumVideoSearch.Next()
					if err == nil && albumVideoResult != nil {
						for _, v := range albumVideoResult.Videos {
							titleLower := strings.ToLower(v.Title)
							// Check if title suggests it's a full album or playlist
							if strings.Contains(titleLower, "full album") || 
							   strings.Contains(titleLower, "complete album") ||
							   strings.Contains(titleLower, "playlist") {
								thumb := ""
								if len(v.Thumbnails) > 0 {
									thumb = v.Thumbnails[0].URL
								}
								// Note: These are videos, not playlists, so trackCount will be 0
								// But we'll mark them as albums so users can download them
								items = append(items, songItem{
									id:         v.ID,
									title:      v.Title,
									author:     v.Channel.Title,
									thumb:      thumb,
									isAlbum:    true,
									trackCount: 0, // Unknown for video-based "albums"
								})
							}
						}
					}
				}()
			}
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

// isValidPlaylistID checks if an ID is a valid YouTube playlist ID
// Playlist IDs typically start with "PL" and are longer than video IDs
func isValidPlaylistID(id string) bool {
	if id == "" {
		return false
	}
	// Playlist IDs usually start with "PL" or "RD" (radio playlists) or "OL" (official playlists)
	// Video IDs are 11 characters and don't start with these prefixes
	if strings.HasPrefix(id, "PL") || strings.HasPrefix(id, "RD") || strings.HasPrefix(id, "OL") {
		return true
	}
	// If it's longer than 11 chars, it might be a playlist ID
	if len(id) > 11 {
		return true
	}
	// If it's exactly 11 chars and doesn't start with PL/RD/OL, it's likely a video ID
	if len(id) == 11 {
		return false
	}
	// For other lengths, be more permissive but check if it looks like a playlist
	return len(id) >= 13 // Playlist IDs are usually at least 13 characters
}

func fetchAlbumTracks(playlistID string) tea.Cmd {
	return func() tea.Msg {
		// Validate playlist ID first
		if !isValidPlaylistID(playlistID) {
			return errMsg(fmt.Errorf("invalid playlist ID: %s (expected playlist ID starting with PL, RD, or OL)", playlistID))
		}
		
		client := youtube.Client{}
		// GetPlaylist expects a URL, so construct it from the playlist ID
		// Handle both URL and ID formats
		var playlistURL string
		if strings.HasPrefix(playlistID, "http") {
			playlistURL = playlistID
		} else {
			// It's a playlist ID
			playlistURL = "https://www.youtube.com/playlist?list=" + playlistID
		}
		
		playlist, err := client.GetPlaylist(playlistURL)
		if err != nil {
			return errMsg(fmt.Errorf("failed to fetch playlist: %v (URL: %s)", err, playlistURL))
		}

		var tracks []songItem
		for _, entry := range playlist.Videos {
			thumb := ""
			if len(entry.Thumbnails) > 0 {
				thumb = entry.Thumbnails[0].URL
			}
			tracks = append(tracks, songItem{
				id:         entry.ID,
				title:      entry.Title,
				author:     entry.Author,
				thumb:      thumb,
				isAlbum:    false,
				trackCount: 0,
			})
		}

		return albumTracksFetchedMsg(tracks)
	}
}

func (m *model) runDownloadAlbum() {
	client := youtube.Client{}
	
	// Fetch all tracks from the playlist
	// GetPlaylist expects a URL, so construct it from the playlist ID
	playlistURL := "https://www.youtube.com/playlist?list=" + m.selected.id
	playlist, err := client.GetPlaylist(playlistURL)
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	totalTracks := len(playlist.Videos)
	if totalTracks == 0 {
		m.program.Send(errMsg(fmt.Errorf("no tracks found in album")))
		return
	}

	// Create album directory
	albumDir := strings.ReplaceAll(m.selected.title, "/", "_")
	albumDir = strings.ReplaceAll(albumDir, "\\", "_")
	err = os.MkdirAll(albumDir, 0755)
	if err != nil {
		m.program.Send(errMsg(fmt.Errorf("failed to create album directory: %v", err)))
		return
	}

	// Download album cover
	albumThumb := "temp_album_thumb.jpg"
	err = m.downloadThumb(m.selected.thumb, albumThumb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading album thumb: %v\n", err)
	}

		// Download each track
		for i, entry := range playlist.Videos {
			m.program.Send(albumTrackProgressMsg{
				current: i + 1,
				total:   totalTracks,
				title:   entry.Title,
			})

		// Get video details
		videoDetails, err := client.GetVideo(entry.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting video %s: %v\n", entry.ID, err)
			continue
		}

		formats := videoDetails.Formats.Type("audio")
		if len(formats) == 0 {
			fmt.Fprintf(os.Stderr, "No audio format found for %s\n", entry.Title)
			continue
		}
		format := &formats[0]

		tempAudio := fmt.Sprintf("temp_audio_%d", i)
		finalName := fmt.Sprintf("%s/%02d - %s.mp3", albumDir, i+1, strings.ReplaceAll(videoDetails.Title, "/", "_"))

		err = m.downloadFile(client, format, videoDetails, tempAudio, func(p float64) {
			// Progress for individual track
			m.program.Send(downloadProgressMsg(p))
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", entry.Title, err)
			os.Remove(tempAudio)
			continue
		}

		// Convert to MP3 with metadata
		args := []string{
			"-y",
			"-i", tempAudio,
			"-i", albumThumb,
			"-map", "0:0",
			"-map", "1:0",
			"-c:a", "libmp3lame",
			"-q:a", "2",
			"-id3v2_version", "3",
			"-metadata:s:v", "title=\"Album cover\"",
			"-metadata:s:v", "comment=\"Cover (Front)\"",
			"-metadata", "title=" + videoDetails.Title,
			"-metadata", "artist=" + videoDetails.Author,
			"-metadata", "album=" + m.selected.title,
			"-metadata", "track=" + fmt.Sprintf("%d/%d", i+1, totalTracks),
			finalName,
		}

		cmd := exec.Command("ffmpeg", args...)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "FFmpeg failed for %s: %v\n", entry.Title, err)
			os.Remove(tempAudio)
			continue
		}

		os.Remove(tempAudio)
	}

	os.Remove(albumThumb)
	m.program.Send(doneMsg(fmt.Sprintf("Album: %s (%d tracks)", albumDir, totalTracks)))
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
				m.state = stateViewingAlbumTracks
				return m, nil
			}
			if m.state == stateViewingAlbumTracks {
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
				return m, tea.Batch(m.spinner.Tick, searchSongs(m.textInput.Value(), m.searchFilter))
			}
			if m.state == stateSelecting {
				item, ok := m.list.SelectedItem().(songItem)
				if ok {
					m.selected = item
					if item.isAlbum {
						// View album tracks instead of immediately downloading
						m.currentAlbum = item
						m.state = stateSearching
						return m, tea.Batch(m.spinner.Tick, fetchAlbumTracks(item.id))
					} else {
						m.state = stateDownloading
						go m.runDownloadConvert()
					}
					return m, nil
				}
			}
			if m.state == stateViewingAlbumTracks {
				item, ok := m.albumTrackList.SelectedItem().(songItem)
				if ok {
					// Skip if album header is selected
					if item.isAlbum {
						return m, nil
					}
					m.stopPlayback() // Cleanup any existing playback first
					// Find the original track (without tree prefix) from albumTracks
					for _, origTrack := range m.albumTracks {
						if origTrack.id == item.id {
							m.selected = origTrack
							m.state = stateLoading
							go m.runInternalPlayback(origTrack)
							return m, m.spinner.Tick
						}
					}
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
			if m.state == stateViewingAlbumTracks {
				item, ok := m.albumTrackList.SelectedItem().(songItem)
				if ok {
					// Skip if album header is selected
					if item.isAlbum {
						return m, nil
					}
					m.stopPlayback() // Cleanup any existing playback first
					// Find the original track (without tree prefix) from albumTracks
					for _, origTrack := range m.albumTracks {
						if origTrack.id == item.id {
							m.selected = origTrack
							m.state = stateLoading
							go m.runInternalPlayback(origTrack)
							return m, m.spinner.Tick
						}
					}
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
			if m.state == stateViewingAlbumTracks {
				m.state = stateSelecting
				return m, nil
			}
			if m.state == stateSelecting {
				m.state = stateInput
				return m, nil
			}
		case "1":
			if m.state == stateInput {
				m.searchFilter = filterAll
				return m, nil
			}
		case "2":
			if m.state == stateInput {
				m.searchFilter = filterSongs
				return m, nil
			}
		case "3":
			if m.state == stateInput {
				m.searchFilter = filterAlbums
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
		m.list.Title = "Select Song or Album"
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
		if m.state == statePlaying {
			// Only return to album tracks view if we have a valid album track list
			// Check if list is initialized (width > 0) and has tracks
			if len(m.albumTracks) > 0 && m.albumTrackList.Width() > 0 {
				m.state = stateViewingAlbumTracks
			} else {
				// Fallback to selecting state if album track list is not valid
				m.state = stateSelecting
				m.list.ResetSelected()
			}
		} else {
			m.state = stateSelecting
			m.list.ResetSelected()
		}
		return m, nil

	case albumTracksFetchedMsg:
		m.albumTracks = msg
		// Create list of tracks for viewing with tree structure
		var trackItems []list.Item
		
		// Add album header
		albumHeader := songItem{
			id:      m.currentAlbum.id,
			title:   fmt.Sprintf("📀 %s", m.currentAlbum.title),
			author:  m.currentAlbum.author,
			isAlbum: true,
		}
		trackItems = append(trackItems, albumHeader)
		
		// Add tracks with tree view formatting
		for i, track := range msg {
			// Create a copy for display with tree structure
			displayTrack := track
			// Use tree characters for visual hierarchy
			if i == len(msg)-1 {
				// Last track
				displayTrack.title = fmt.Sprintf("└── %02d. %s", i+1, track.title)
			} else {
				// Middle tracks
				displayTrack.title = fmt.Sprintf("├── %02d. %s", i+1, track.title)
			}
			trackItems = append(trackItems, displayTrack)
		}
		
		m.albumTrackList = list.New(trackItems, list.NewDefaultDelegate(), m.width-4, m.height-8)
		m.albumTrackList.Title = fmt.Sprintf("Album: %s (%d tracks)", m.currentAlbum.title, len(msg))
		m.state = stateViewingAlbumTracks
		return m, nil

	case albumTrackProgressMsg:
		m.albumProgress.current = msg.current
		m.albumProgress.total = msg.total
		m.albumProgress.title = msg.title
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
		if m.state == stateViewingAlbumTracks {
			m.albumTrackList.SetSize(msg.Width-4, msg.Height-8)
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

	if m.state == stateViewingAlbumTracks {
		// Safety check: ensure album track list is valid before updating
		// Check if list is properly initialized by checking its width (initialized lists have width > 0)
		if m.albumTrackList.Width() == 0 {
			// If list is invalid, recreate it from albumTracks
			if len(m.albumTracks) > 0 {
				var trackItems []list.Item
				albumHeader := songItem{
					id:      m.currentAlbum.id,
					title:   fmt.Sprintf("📀 %s", m.currentAlbum.title),
					author:  m.currentAlbum.author,
					isAlbum: true,
				}
				trackItems = append(trackItems, albumHeader)
				
				for i, track := range m.albumTracks {
					displayTrack := track
					if i == len(m.albumTracks)-1 {
						displayTrack.title = fmt.Sprintf("└── %02d. %s", i+1, track.title)
					} else {
						displayTrack.title = fmt.Sprintf("├── %02d. %s", i+1, track.title)
					}
					trackItems = append(trackItems, displayTrack)
				}
				m.albumTrackList = list.New(trackItems, list.NewDefaultDelegate(), m.width-4, m.height-8)
				m.albumTrackList.Title = fmt.Sprintf("Album: %s (%d tracks)", m.currentAlbum.title, len(m.albumTracks))
			} else {
				// No tracks available, go back to selecting
				m.state = stateSelecting
				return m, nil
			}
		}
		// Safely update the list with panic recovery
		var cmd tea.Cmd
		func() {
			defer func() {
				if r := recover(); r != nil {
					// If update panics, recreate the list
					if len(m.albumTracks) > 0 {
						var trackItems []list.Item
						albumHeader := songItem{
							id:      m.currentAlbum.id,
							title:   fmt.Sprintf("📀 %s", m.currentAlbum.title),
							author:  m.currentAlbum.author,
							isAlbum: true,
						}
						trackItems = append(trackItems, albumHeader)
						
						for i, track := range m.albumTracks {
							displayTrack := track
							if i == len(m.albumTracks)-1 {
								displayTrack.title = fmt.Sprintf("└── %02d. %s", i+1, track.title)
							} else {
								displayTrack.title = fmt.Sprintf("├── %02d. %s", i+1, track.title)
							}
							trackItems = append(trackItems, displayTrack)
						}
						m.albumTrackList = list.New(trackItems, list.NewDefaultDelegate(), m.width-4, m.height-8)
						m.albumTrackList.Title = fmt.Sprintf("Album: %s (%d tracks)", m.currentAlbum.title, len(m.albumTracks))
					}
				}
			}()
			m.albumTrackList, cmd = m.albumTrackList.Update(msg)
		}()
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
		filterText := "All"
		switch m.searchFilter {
		case filterSongs:
			filterText = "Songs Only"
		case filterAlbums:
			filterText = "Albums Only"
		}
		s = fmt.Sprintf("\n  %s\n\n  %s\n\n  %s\n\n  %s",
			titleStyle.Render("GoMusic Search"),
			m.textInput.View(),
			helpStyle.Render(fmt.Sprintf("Filter: %s  •  1: All  2: Songs  3: Albums", filterText)),
			helpStyle.Render("Enter song name, artist, or album"),
		)
	case stateSearching:
		s = fmt.Sprintf("\n  %s Searching YouTube Music...\n", m.spinner.View())
	case stateSelecting:
		return docStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				m.list.View(),
				helpStyle.Render("\n  ENTER: View Album/Download  •  P: Play Integrated  •  Q: Quit"),
			),
		)
	case stateViewingAlbumTracks:
		return docStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				m.albumTrackList.View(),
				helpStyle.Render("\n  ENTER: Play Track  •  P: Play Track  •  Q: Back to Albums  •  ESC: Back"),
			),
		)
	case stateDownloading:
		s = fmt.Sprintf("\n  %s\n\n  %s\n\n  %s",
			titleStyle.Render("Downloading: "+m.selected.title),
			m.progress.View(),
			helpStyle.Render("Selected: "+m.selected.author),
		)
	case stateDownloadingAlbum:
		trackInfo := fmt.Sprintf("Track %d/%d: %s", m.albumProgress.current, m.albumProgress.total, m.albumProgress.title)
		s = fmt.Sprintf("\n  %s\n\n  %s\n\n  %s\n\n  %s",
			titleStyle.Render("Downloading Album: "+m.selected.title),
			m.progress.View(),
			statusStyle.Render(trackInfo),
			helpStyle.Render("Downloading all tracks from album..."),
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
		state:        stateInput,
		textInput:    ti,
		spinner:      s,
		progress:     p,
		playback:     &playbackState{},
		searchFilter: filterAll,
	}

	program := tea.NewProgram(m)
	m.program = program

	initSpeaker()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running GoMusic: %v\n", err)
		os.Exit(1)
	}
}
