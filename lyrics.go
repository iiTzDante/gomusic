package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LRCLIB API response structure
type lrclibResponse struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	Duration     float64 `json:"duration"`
	LrcLibID     int     `json:"id"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

func fetchLyrics(title, artist string, duration int) ([]LyricLine, error) {
	// Search for lyrics using LRCLIB API - optimized order

	cleanedTitle := cleanString(title)
	cleanedArtist := cleanArtist(artist)

	// Strategy 1: Search endpoint first (broader, usually faster)
	searchQuery := cleanedArtist + " " + cleanedTitle
	lyrics, err := trySearch(searchQuery)
	if err == nil {
		return lyrics, nil
	}

	// Strategy 2: If title has " - ", try splitting it
	if strings.Contains(title, " - ") {
		parts := strings.SplitN(title, " - ", 2)
		newArtist := cleanArtist(parts[0])
		newTitle := cleanString(parts[1])

		lyrics, err = trySearch(newArtist + " " + newTitle)
		if err == nil {
			return lyrics, nil
		}
	}

	// Strategy 3: Exact get without duration (last resort)
	lyrics, err = tryFetch(cleanedTitle, cleanedArtist, 0)
	if err == nil {
		return lyrics, nil
	}

	return nil, fmt.Errorf("lyrics not found")
}

func tryFetch(title, artist string, duration int) ([]LyricLine, error) {
	baseURL := "https://lrclib.net/api/get"
	params := url.Values{}
	params.Add("artist_name", artist)
	params.Add("track_name", title)
	if duration > 0 {
		params.Add("duration", strconv.Itoa(duration))
	}

	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var lrclib lrclibResponse
	if err := json.NewDecoder(resp.Body).Decode(&lrclib); err != nil {
		return nil, err
	}

	if lrclib.SyncedLyrics == "" {
		return nil, fmt.Errorf("no synced lyrics")
	}

	return parseLRC(lrclib.SyncedLyrics), nil
}

func trySearch(query string) ([]LyricLine, error) {
	baseURL := "https://lrclib.net/api/search"
	params := url.Values{}
	params.Add("q", query)

	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var results []lrclibResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	for _, res := range results {
		if res.SyncedLyrics != "" {
			return parseLRC(res.SyncedLyrics), nil
		}
	}

	return nil, fmt.Errorf("no synced lyrics in search")
}

func cleanString(s string) string {
	// 1. Remove anything in square brackets or parentheses
	reBrackets := regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)`)
	s = reBrackets.ReplaceAllString(s, "")

	// 2. Remove common suffixes (case insensitive)
	suffixes := []string{
		"official music video", "official video", "official audio",
		"music video", "lyric video", "lyrics", "official",
		"video", "audio", "full song", "hd", "4k", "720p", "1080p",
	}
	for _, suffix := range suffixes {
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(suffix))
		s = re.ReplaceAllString(s, "")
	}

	// 3. Remove "by Artist" patterns if they exist
	reBy := regexp.MustCompile(`(?i)\s+by\s+.*$`)
	s = reBy.ReplaceAllString(s, "")

	// 4. Cleanup dashes and pipes
	s = strings.ReplaceAll(s, "ft.", "")
	s = strings.ReplaceAll(s, "feat.", "")

	return strings.TrimSpace(s)
}

func cleanArtist(s string) string {
	s = strings.TrimSuffix(s, " - Topic")
	s = strings.TrimSuffix(s, "VEVO")
	s = strings.TrimSuffix(s, "Vevo")
	return strings.TrimSpace(s)
}

func parseLRC(lrcText string) []LyricLine {
	var lines []LyricLine
	// Regex to match [mm:ss.xx] text
	re := regexp.MustCompile(`\[(\d+):(\d+\.\d+)\](.*)`)

	scanner := strings.Split(lrcText, "\n")
	for _, text := range scanner {
		matches := re.FindStringSubmatch(text)
		if len(matches) == 4 {
			min, _ := strconv.Atoi(matches[1])
			sec, _ := strconv.ParseFloat(matches[2], 64)
			lyric := strings.TrimSpace(matches[3])

			duration := time.Duration(min)*time.Minute + time.Duration(sec*float64(time.Second))
			lines = append(lines, LyricLine{
				Timestamp: duration,
				Text:      lyric,
			})
		}
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Timestamp < lines[j].Timestamp
	})

	return lines
}
