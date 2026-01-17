package main

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
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
)

const appVersion = "1.1.0"

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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isKittyTerminal checks if we're running in Kitty terminal
func isKittyTerminal() bool {
	return os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != ""
}

// isImageCapableTerminal checks if the terminal supports image display
func isImageCapableTerminal() bool {
	// Check for Kitty
	if isKittyTerminal() {
		return true
	}
	
	// Check for iTerm2
	if strings.Contains(os.Getenv("TERM_PROGRAM"), "iTerm") {
		return true
	}
	
	// Check for WezTerm
	if os.Getenv("TERM_PROGRAM") == "WezTerm" {
		return true
	}
	
	return false
}

// displayKittyImageDirect displays an image directly to the terminal, bypassing TUI
func displayKittyImageDirect(imagePath string) {
	if !isKittyTerminal() {
		return
	}

	// Use kitten icat to display the image on the left with specific positioning
	cmd := exec.Command("kitten", "icat", 
		"--place", "20x10@0x0", // 20 columns x 10 rows at position 0,0 (top-left)
		"--engine", "builtin",
		imagePath,
	)
	
	// Allow output to show the image
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	
	if err != nil {
		// Try without positioning if place fails
		cmd = exec.Command("kitten", "icat", 
			"--align", "left",
			imagePath,
		)
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

// clearKittyImages clears all images from the terminal
func clearKittyImages() {
	if !isKittyTerminal() {
		return
	}

	// Use kitten icat --clear to remove all images
	cmd := exec.Command("kitten", "icat", "--clear")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// displayKittyImage displays an image using kitten icat
func displayKittyImage(imagePath string, width, height int) string {
	if !isKittyTerminal() {
		return ""
	}

	// Use kitten icat with stream transfer mode to get the escape sequences
	// This should work better with TUI applications
	cmd := exec.Command("kitten", "icat", 
		"--transfer-mode", "stream",
		"--align", "left",
		imagePath,
	)
	
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	
	return string(output)
}

// displayITermImage displays an image using iTerm2's image protocol
func displayITermImage(imagePath string) string {
	if !strings.Contains(os.Getenv("TERM_PROGRAM"), "iTerm") {
		return ""
	}

	// Read the image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return ""
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(imageData)

	// iTerm2 image protocol: \033]1337;File=inline=1:<base64_data>\007
	itermSequence := fmt.Sprintf("\033]1337;File=inline=1:%s\007", encoded)
	
	return itermSequence
}

// displayTerminalImage displays an image using the appropriate terminal protocol
func displayTerminalImage(imagePath string, width, height int) string {
	termProgram := os.Getenv("TERM_PROGRAM")
	
	if isKittyTerminal() || termProgram == "kiro" {
		// Try Kitty protocol for both Kitty and Kiro terminals
		return displayKittyImage(imagePath, width, height)
	} else if strings.Contains(termProgram, "iTerm") {
		return displayITermImage(imagePath)
	}
	return ""
}

// resizeImage resizes an image to fit within the specified dimensions while maintaining aspect ratio
func resizeImage(inputPath, outputPath string, maxWidth, maxHeight int) error {
	// Use ffmpeg first (more reliable for various formats)
	cmd := exec.Command("ffmpeg", 
		"-i", inputPath,
		"-vf", fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease", maxWidth, maxHeight),
		"-q:v", "2", // High quality
		"-y", // Overwrite output file
		outputPath,
	)
	
	// Suppress ffmpeg output
	cmd.Stderr = nil
	cmd.Stdout = nil
	
	err := cmd.Run()
	if err != nil {
		// Fallback to ImageMagick if ffmpeg fails
		cmd = exec.Command("convert", inputPath, 
			"-resize", fmt.Sprintf("%dx%d>", maxWidth, maxHeight),
			"-quality", "95", // High quality
			outputPath,
		)
		cmd.Stderr = nil
		cmd.Stdout = nil
		return cmd.Run()
	}
	
	return nil
}

// convertImageToASCII converts an image to colored ASCII art with improved quality
func convertImageToASCII(imagePath string, width, height int) string {
	file, err := os.Open(imagePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Decode image
	var img image.Image
	if strings.HasSuffix(strings.ToLower(imagePath), ".jpg") || strings.HasSuffix(strings.ToLower(imagePath), ".jpeg") {
		img, err = jpeg.Decode(file)
	} else if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
		img, err = png.Decode(file)
	} else {
		// Try to decode as any supported format
		img, _, err = image.Decode(file)
	}
	
	if err != nil {
		return ""
	}

	bounds := img.Bounds()
	imgWidth := bounds.Max.X - bounds.Min.X
	imgHeight := bounds.Max.Y - bounds.Min.Y

	// Calculate scaling factors
	scaleX := float64(imgWidth) / float64(width)
	scaleY := float64(imgHeight) / float64(height)

	// Enhanced ASCII characters with better gradation
	chars := []rune{' ', '░', '▒', '▓', '█'}
	
	var result strings.Builder
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Sample pixel from original image
			srcX := int(float64(x) * scaleX)
			srcY := int(float64(y) * scaleY)
			
			if srcX >= imgWidth {
				srcX = imgWidth - 1
			}
			if srcY >= imgHeight {
				srcY = imgHeight - 1
			}
			
			pixel := img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY)
			r, g, b, _ := pixel.RGBA()
			
			// Convert to 8-bit RGB values
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			
			// Convert to grayscale for character selection
			gray := (r*299 + g*587 + b*114) / 1000
			
			// Map to character index
			charIndex := int(float64(gray) / 65535.0 * float64(len(chars)-1))
			if charIndex >= len(chars) {
				charIndex = len(chars) - 1
			}
			
			// Create colored character using ANSI escape codes
			char := chars[charIndex]
			if char != ' ' {
				// Use RGB color for the character
				coloredChar := fmt.Sprintf("\033[38;2;%d;%d;%dm%c\033[0m", r8, g8, b8, char)
				result.WriteString(coloredChar)
			} else {
				result.WriteRune(char)
			}
		}
		if y < height-1 {
			result.WriteRune('\n')
		}
	}
	
	return result.String()
}

// downloadAndCacheThumb downloads and caches a thumbnail for display
func (m *model) downloadAndCacheThumb(url, path string) error {
	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return nil // File already exists
	}
	
	return m.downloadThumb(url, path)
}

func searchSongs(query string, filter searchFilter) tea.Cmd {
	return searchYTMusic(query, filter)
}

func fetchAlbumTracks(browseID string) tea.Cmd {
	return fetchYTMusicAlbumTracks(browseID)
}

func (m *model) runDownloadConvert() {
	// Validate track ID before attempting download
	if m.selected.id == "" || len(m.selected.id) < 10 {
		m.program.Send(errMsg(fmt.Errorf("cannot download this track - invalid track ID")))
		return
	}

	client := youtube.Client{}
	track, err := client.GetVideo(m.selected.id) // GetVideo works for music tracks too
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	m.program.Send(metadataFetchedMsg{
		id:     m.selected.id,
		title:  track.Title,
		author: track.Author,
	})

	formats := track.Formats.Type("audio")
	if len(formats) == 0 {
		m.program.Send(errMsg(fmt.Errorf("no audio format found")))
		return
	}
	format := &formats[0]

	tempAudio := "temp_audio"
	tempThumb := "temp_thumb.jpg"
	finalName := strings.ReplaceAll(track.Title, "/", "_") + ".mp3"

	err = m.downloadFile(client, format, track, tempAudio, func(p float64) {
		m.program.Send(downloadProgressMsg(p))
	})
	if err != nil {
		m.program.Send(errMsg(err))
		return
	}

	m.program.Send(convertMsg{})
	err = m.downloadThumb(m.selected.thumb, tempThumb)
	if err != nil {
		// Silently continue if thumb download fails
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
		"-metadata", "title=" + track.Title,
		"-metadata", "artist=" + track.Author,
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

func (m *model) runDownloadAlbum() {
	if len(m.albumTracks) == 0 {
		m.program.Send(errMsg(fmt.Errorf("no tracks found in album")))
		return
	}

	// Clean up album name for folder creation
	albumName := m.currentAlbum.title
	// Remove year from title if present
	if strings.Contains(albumName, "(") && strings.Contains(albumName, ")") {
		parts := strings.Split(albumName, "(")
		albumName = strings.TrimSpace(parts[0])
	}
	// Remove "Topic" and other suffixes
	albumName = strings.TrimSuffix(albumName, " - Topic")
	albumName = strings.TrimSuffix(albumName, "Topic")
	albumName = strings.TrimSpace(albumName)
	
	// Create safe folder name
	albumDir := strings.ReplaceAll(albumName, "/", "_")
	albumDir = strings.ReplaceAll(albumDir, "\\", "_")
	albumDir = strings.ReplaceAll(albumDir, ":", "_")
	albumDir = strings.ReplaceAll(albumDir, "*", "_")
	albumDir = strings.ReplaceAll(albumDir, "?", "_")
	albumDir = strings.ReplaceAll(albumDir, "\"", "_")
	albumDir = strings.ReplaceAll(albumDir, "<", "_")
	albumDir = strings.ReplaceAll(albumDir, ">", "_")
	albumDir = strings.ReplaceAll(albumDir, "|", "_")
	
	err := os.MkdirAll(albumDir, 0755)
	if err != nil {
		m.program.Send(errMsg(fmt.Errorf("failed to create album directory: %v", err)))
		return
	}

	totalTracks := len(m.albumTracks)
	client := youtube.Client{}

	// Download album cover if available
	albumThumb := "temp_album_thumb.jpg"
	if m.currentAlbum.thumb != "" {
		err = m.downloadThumb(m.currentAlbum.thumb, albumThumb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading album thumb: %v\n", err)
		}
	}

	// Download each track
	for i, track := range m.albumTracks {
		// Skip tracks with invalid IDs
		if track.id == "" || len(track.id) < 10 {
			continue
		}

		m.program.Send(albumTrackProgressMsg{
			current: i + 1,
			total:   totalTracks,
			title:   track.title,
		})

		// Get track details
		trackDetails, err := client.GetVideo(track.id)
		if err != nil {
			continue
		}

		formats := trackDetails.Formats.Type("audio")
		if len(formats) == 0 {
			continue
		}
		format := &formats[0]

		tempAudio := fmt.Sprintf("temp_audio_%d", i)
		safeTitle := strings.ReplaceAll(trackDetails.Title, "/", "_")
		safeTitle = strings.ReplaceAll(safeTitle, "\\", "_")
		safeTitle = strings.ReplaceAll(safeTitle, ":", "_")
		finalName := fmt.Sprintf("%s/%02d - %s.mp3", albumDir, i+1, safeTitle)

		err = m.downloadFile(client, format, trackDetails, tempAudio, func(p float64) {
			// Calculate overall album progress: (completed tracks + current track progress) / total tracks
			overallProgress := (float64(i) + p) / float64(totalTracks)
			m.program.Send(downloadProgressMsg(overallProgress))
		})
		if err != nil {
			os.Remove(tempAudio)
			continue
		}

		// Convert to MP3 with metadata
		args := []string{
			"-y",
			"-i", tempAudio,
		}
		
		// Add album cover if available
		if m.currentAlbum.thumb != "" {
			args = append(args, "-i", albumThumb, "-map", "0:0", "-map", "1:0")
		} else {
			args = append(args, "-map", "0:0")
		}
		
		args = append(args,
			"-c:a", "libmp3lame",
			"-q:a", "2",
			"-id3v2_version", "3",
		)
		
		// Add album cover metadata if available
		if m.currentAlbum.thumb != "" {
			args = append(args,
				"-metadata:s:v", "title=\"Album cover\"",
				"-metadata:s:v", "comment=\"Cover (Front)\"",
			)
		}
		
		args = append(args,
			"-metadata", "title=" + trackDetails.Title,
			"-metadata", "artist=" + trackDetails.Author,
			"-metadata", "album=" + albumName,
			"-metadata", "track=" + fmt.Sprintf("%d/%d", i+1, totalTracks),
			finalName,
		)

		cmd := exec.Command("ffmpeg", args...)
		if err := cmd.Run(); err != nil {
			os.Remove(tempAudio)
			continue
		}

		os.Remove(tempAudio)
	}

	// Clean up album thumb
	if m.currentAlbum.thumb != "" {
		os.Remove(albumThumb)
	}
	
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
						// For albums, try to fetch tracks using the album title and artist
						m.currentAlbum = item
						m.state = stateSearching
						
						// Use enhanced album track search
						return m, tea.Batch(m.spinner.Tick, searchAlbumWithTracks(item.title, item.author))
					} else {
						// Check if track has valid ID before downloading
						if item.id == "" || len(item.id) < 10 {
							return m, nil // Do nothing for invalid tracks
						}
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
						// Download the entire album
						m.selected = m.currentAlbum
						m.state = stateDownloadingAlbum
						go m.runDownloadAlbum()
						return m, nil
					}
					// Download individual track from album
					m.stopPlayback() // Cleanup any existing playback first
					// Find the original track (without tree prefix) from albumTracks
					for _, origTrack := range m.albumTracks {
						if origTrack.id == item.id {
							// Check if track has valid ID before downloading
							if origTrack.id == "" || len(origTrack.id) < 10 {
								return m, nil // Do nothing for invalid tracks
							}
							m.selected = origTrack
							m.state = stateDownloading
							go m.runDownloadConvert()
							return m, nil
						}
					}
				}
			}
		case "p":
			if m.state == stateSelecting {
				item, ok := m.list.SelectedItem().(songItem)
				if ok {
					// Don't allow playing albums directly - only individual tracks
					if item.isAlbum {
						return m, nil // Do nothing for albums
					}
					
					// Check if track has valid ID
					if item.id == "" || len(item.id) < 10 {
						return m, nil // Do nothing for invalid tracks
					}
					
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
							// Check if track has valid ID
							if origTrack.id == "" || len(origTrack.id) < 10 {
								return m, nil // Do nothing for invalid tracks
							}
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

	case imageReadyMsg:
		// When image is ready, just store the path - don't display immediately
		// Let the View function handle the display timing
		if m.state == statePlaying {
			m.playback.kittyImage = msg.imagePath
		}
		return m, nil

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
		
		// Add album header with download instruction
		albumHeader := songItem{
			id:      m.currentAlbum.id,
			title:   fmt.Sprintf("📀 %s (Press ENTER to download full album)", m.currentAlbum.title),
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
				helpStyle.Render("\n  ENTER: Browse Album/Download Song  •  P: Play Song  •  Q: Quit"),
			),
		)
	case stateViewingAlbumTracks:
		return docStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				m.albumTrackList.View(),
				helpStyle.Render("\n  ENTER: Download (Album header = Full Album, Track = Single)  •  P: Play Track  •  Q: Back  •  ESC: Back"),
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
		// Create clean content
		mainContent := fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			titleStyle.Render("Now Playing: " + m.playback.playingSong),
			m.renderLyrics(),
			helpStyle.Render("SPACE: Play/Pause  •  S: Stop  •  Q: Exit"),
		)

		// Check if we have ASCII art album cover
		if m.playback.albumCover != "" {
			// Display ASCII art album cover on the left
			coverStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 1)
			
			styledCover := coverStyle.Render(m.playback.albumCover)
			
			// Add info about the ASCII art
			asciiInfo := helpStyle.Render("🎨  Colorized ASCII album art")
			
			// Join cover and main content horizontally
			s = lipgloss.JoinHorizontal(
				lipgloss.Top,
				lipgloss.JoinVertical(lipgloss.Left, styledCover, asciiInfo),
				"  ", // Spacing
				mainContent,
			)
		} else {
			// No cover available, show main content only
			s = fmt.Sprintf("\n  %s", mainContent)
		}
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
