# GoMusic

A fast terminal UI for downloading music from YouTube with automatic MP3 conversion and metadata tagging.

## Features

- 🔍 Fast YouTube Music search (5x concurrent workers)
- 🎵 Audio-only downloads (prioritizes music over music videos)
- 🎧 **Playback preview** (`p` key) - requires building from source with ALSA
- 📝 Automatic ID3 tagging (title, artist, album art)
- 🎨 Beautiful TUI with Bubble Tea
- ⚡ High-quality MP3 conversion with FFmpeg
- 🖼️ Embedded album artwork

## Installation

```bash
go install github.com/iiTzDante/gomusic@latest
```

Or build from source:

```bash
git clone https://github.com/iiTzDante/gomusic
cd gomusic
go build -o gomusic main.go
```

## Requirements

- Go 1.19 or later
- FFmpeg (for MP3 conversion and tagging)
- Chrome/Chromium (for YouTube scraping via Rod)
- **ALSA** (Linux only, for playback feature when building from source)

> **Note**: Pre-built release binaries are download-only and do not include playback functionality. To use the integrated playback feature (`p` key), build from source on Linux with ALSA libraries installed (`libasound2-dev` on Debian/Ubuntu).

## Usage

```bash
gomusic
``` 

Then:
1. Enter song name, artist, or album
2. Select from search results
3. Wait for download and conversion
4. Find your MP3 in the current directory

## How It Works

1. Searches YouTube Music using headless browser
2. Fetches video metadata (title, artist, thumbnail)
3. Downloads best available audio format
4. Converts to MP3 with FFmpeg
5. Embeds ID3 tags and cover art

## Search Optimization

The app appends " audio" to search queries to prioritize:
- Official audio tracks
- YouTube Music "Topic" channels
- Audio-only uploads over music videos

## Dependencies

- [kkdai/youtube](https://github.com/kkdai/youtube) - YouTube video fetching
- [PChaparro/go-youtube-scraper](https://github.com/PChaparro/go-youtube-scraper) - YouTube search
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) - TUI components

## Environment Variables

- `ROD_LOG=false` - Silences browser logs (automatically set)

## License

MIT
