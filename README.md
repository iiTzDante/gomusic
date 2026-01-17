# GoMusic

A lightning-fast terminal UI for downloading and streaming music from YouTube Music with automatic MP3 conversion and metadata tagging.

## Features

- ⚡ **YouTube Music Integration**: Direct access to YouTube Music's curated catalog with high-quality metadata.
- 🎧 **Zero-Wait Streaming**: Start listening immediately with real-time audio streaming.
- 📀 **Full Album Support**: Browse and download complete albums with proper track numbering.
- 🎵 **High-Quality Audio**: Automatic download and conversion to high-bitrate MP3.
- 🎨 **Modern TUI**: Beautiful interface with rhythmic visualizers and smooth animations.
- 📝 **Smart Metadata**: Automatically embeds title, artist, album, and high-res cover art into MP3s.
- 🏹 **Enhanced Controls**: Responsive seeking, pause/resume, and smart navigation.
- 🎼 **Synced Lyrics**: Real-time lyric display with automatic synchronization.

## Installation

### Arch Linux (AUR)
```bash
yay -S gomusic
```

### From Source
```bash
git clone https://github.com/iiTzDante/gomusic
cd gomusic
go build -o gomusic .
```

## Requirements

- **Go 1.22+** (for building from source)
- **FFmpeg** (essential for transcoding and streaming)
- **ALSA** (Linux only, required for integrated playback)

## Controls

### Main Search
| Key | Action |
|-----|--------|
| `Enter` | Search or Browse Album/Download Song |
| `p` | Instant Playback Preview (Songs only) |
| `1` / `2` / `3` | Filter: All / Songs / Albums |
| `q` | Quit |

### Album View
| Key | Action |
|-----|--------|
| `Enter` | Download Full Album (header) / Download Single Track |
| `p` | Play Individual Track |
| `q` / `Esc` | Back to Search |

### Playback
| Key | Action |
|-----|--------|
| `Space` | Pause / Resume |
| `Left` / `Right` | Seek Backward / Forward (5s) |
| `s` | Stop Playback |
| `q` | Exit Playback |

## How It Works

1.  **YouTube Music Search**: Uses dedicated YouTube Music API for accurate music discovery.
2.  **Smart Album Detection**: Automatically finds and organizes album tracks with proper metadata.
3.  **Instant Stream**: Pipes direct audio streams through FFmpeg for immediate playback.
4.  **Intelligent Download**: Creates organized folders with clean names (removes "Topic" suffixes).
5.  **Rich Metadata**: Embeds complete ID3 tags including album art and track numbers.

## Dependencies

- [kkdai/youtube](https://github.com/kkdai/youtube) - Stream/Video engine
- [raitonoberu/ytmusic](https://github.com/raitonoberu/ytmusic) - YouTube Music API
- [faiface/beep](https://github.com/faiface/beep) - Audio processing
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework

## License

MIT
