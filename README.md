# GoMusic

A lightning-fast terminal UI for downloading and streaming music from YouTube with automatic MP3 conversion and metadata tagging.

## Features

- ⚡ **Instant Search**: Lightweight, sub-second search without overhead.
- 🎧 **Zero-Wait Streaming**: Start listening immediately with real-time audio streaming.
- 🎵 **High-Quality Audio**: Automatic download and conversion to high-bitrate MP3.
- 🎨 **Modern TUI**: Beautiful interface with rhythmic visualizers and smooth animations.
- 📝 **Auto-Tagging**: Automatically embeds title, artist, and high-res album art into MP3s.
- 🏹 **Enhanced Controls**: Responsive seeking, pause/resume, and smart navigation.

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

| Key | Action |
|-----|--------|
| `Enter` | Search or Start Download |
| `p` | Instant Playback Preview |
| `Space` | Pause / Resume |
| `Left` / `Right` | Seek Backward / Forward (5s) |
| `s` | Stop Playback (Return to list) |
| `q` | Return to list (Playing) / Quit (Menu) |
| `Ctrl+C` | Force Quit |

## How It Works

1.  **Fast Scrape**: Uses a lightweight HTTP scraper to find tracks instantly.
2.  **Instant Stream**: Pipes a direct audio stream through FFmpeg for immediate playback.
3.  **High-Speed Download**: Parallelized downloading and transcoding for local saves.
4.  **Metadata Injection**: Fetches and embeds ID3 tags and cover art on the fly.

## Dependencies

- [kkdai/youtube](https://github.com/kkdai/youtube) - Stream/Video engine
- [raitonoberu/ytsearch](https://github.com/raitonoberu/ytsearch) - High-speed search
- [faiface/beep](https://github.com/faiface/beep) - Audio processing
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework

## License

MIT
