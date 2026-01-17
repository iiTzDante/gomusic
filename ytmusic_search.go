package main

import (
	"fmt"
	"strings"

	"github.com/raitonoberu/ytmusic"
	tea "github.com/charmbracelet/bubbletea"
)

// searchYTMusic performs a YouTube Music search using the dedicated library
func searchYTMusic(query string, filter searchFilter) tea.Cmd {
	return func() tea.Msg {
		var items []songItem

		// Perform search based on filter
		switch filter {
		case filterAll:
			// Search everything
			searchClient := ytmusic.Search(query)
			result, err := searchClient.Next()
			if err != nil {
				return errMsg(fmt.Errorf("YouTube Music search failed: %v", err))
			}
			items = append(items, convertYTMusicResults(result)...)

		case filterSongs:
			// Search only tracks
			searchClient := ytmusic.TrackSearch(query)
			result, err := searchClient.Next()
			if err != nil {
				return errMsg(fmt.Errorf("YouTube Music track search failed: %v", err))
			}
			for _, track := range result.Tracks {
				// Only add tracks with valid IDs
				if len(track.VideoID) >= 10 {
					items = append(items, convertYTMusicTrack(track))
				} else {
					// Skip tracks with invalid IDs silently
				}
			}

		case filterAlbums:
			// Search only albums
			searchClient := ytmusic.AlbumSearch(query)
			result, err := searchClient.Next()
			if err != nil {
				return errMsg(fmt.Errorf("YouTube Music album search failed: %v", err))
			}
			for _, album := range result.Albums {
				items = append(items, convertYTMusicAlbum(album))
			}
		}

		return searchResultsMsg(items)
	}
}

// convertYTMusicResults converts the general search results to songItems
func convertYTMusicResults(result *ytmusic.SearchResult) []songItem {
	var items []songItem

	// Add tracks
	for _, track := range result.Tracks {
		// Only add tracks with valid IDs
		if len(track.VideoID) >= 10 {
			items = append(items, convertYTMusicTrack(track))
		} else {
			// Skip tracks with invalid IDs silently
		}
	}

	// Add albums
	for _, album := range result.Albums {
		items = append(items, convertYTMusicAlbum(album))
	}

	// Add playlists as albums
	for _, playlist := range result.Playlists {
		items = append(items, convertYTMusicPlaylist(playlist))
	}

	return items
}

// convertYTMusicTrack converts a YouTube Music track to songItem
func convertYTMusicTrack(track *ytmusic.TrackItem) songItem {
	// Get the best thumbnail
	thumb := getBestThumbnail(track.Thumbnails)

	// Combine artists into a single string
	artistStr := strings.Join(getArtistNames(track.Artists), ", ")

	// Validate VideoID length - YouTube video IDs should be 11 characters
	videoID := track.VideoID
	title := track.Title
	if len(videoID) < 10 {
		// If VideoID is too short, we can't use this track for playback/download
		// Mark it visually in the title
		title = "⚠️ " + title + " (Not available for playback)"
		videoID = "" // Mark as invalid
	}

	return songItem{
		id:         videoID, // YouTube Music uses VideoID internally for tracks
		title:      title,
		author:     artistStr,
		thumb:      thumb,
		isAlbum:    false,
		trackCount: 0,
	}
}

// convertYTMusicAlbum converts a YouTube Music album to songItem
func convertYTMusicAlbum(album *ytmusic.AlbumItem) songItem {
	// Get the best thumbnail
	thumb := getBestThumbnail(album.Thumbnails)

	// Combine artists into a single string
	artistStr := strings.Join(getArtistNames(album.Artists), ", ")

	// Add album type and year info to the title if available
	title := album.Title
	if album.Year != "" {
		title = fmt.Sprintf("%s (%s)", title, album.Year)
	}

	return songItem{
		id:         album.BrowseID,
		title:      title,
		author:     artistStr,
		thumb:      thumb,
		isAlbum:    true,
		trackCount: 0, // We'll try to get this when browsing the album
	}
}

// convertYTMusicPlaylist converts a YouTube Music playlist to songItem
func convertYTMusicPlaylist(playlist *ytmusic.PlaylistItem) songItem {
	// Get the best thumbnail
	thumb := getBestThumbnail(playlist.Thumbnails)

	return songItem{
		id:         playlist.BrowseID,
		title:      playlist.Title,
		author:     playlist.Author,
		thumb:      thumb,
		isAlbum:    true, // Treat playlists as albums
		trackCount: 0,    // Parse from ItemCount if needed
	}
}

// Helper function to get artist names from the artists slice
func getArtistNames(artists []ytmusic.Artist) []string {
	var names []string
	for _, artist := range artists {
		// Clean up artist name
		cleanName := cleanArtistName(artist.Name)
		names = append(names, cleanName)
	}
	return names
}

// Helper function to clean up artist names
func cleanArtistName(name string) string {
	// Remove common suffixes
	name = strings.TrimSuffix(name, " - Topic")
	name = strings.TrimSuffix(name, "Topic")
	name = strings.TrimSuffix(name, "VEVO")
	name = strings.TrimSuffix(name, "Vevo")
	name = strings.TrimSuffix(name, " Official")
	return strings.TrimSpace(name)
}

// Helper function to get the best thumbnail URL
func getBestThumbnail(thumbnails []ytmusic.Thumbnail) string {
	if len(thumbnails) == 0 {
		return ""
	}
	// Return the largest available thumbnail (last in the slice)
	return thumbnails[len(thumbnails)-1].URL
}

// fetchYTMusicAlbumTracks fetches tracks from a YouTube Music album
func fetchYTMusicAlbumTracks(browseID string) tea.Cmd {
	return func() tea.Msg {
		// Strategy 1: Try to find tracks by searching for the album
		// We'll need to get the album info first, then search for tracks from that album
		
		// Since we don't have direct album browsing, we'll use a workaround:
		// Search for tracks and filter by the album ID/name
		
		// For now, let's try to get a watch playlist from any track in the album
		// This is a limitation of the current library - it doesn't support direct album browsing
		
		// Alternative approach: Search for the album name and get tracks
		return searchAlbumTracksByBrowseID(browseID)
	}
}

// searchAlbumTracksByBrowseID attempts to find album tracks using various strategies
func searchAlbumTracksByBrowseID(browseID string) tea.Msg {
	// Strategy 1: If we have stored album info, search for tracks from that album
	// This is a workaround since the library doesn't support direct album track listing
	
	// For now, we'll return a helpful error message suggesting the user search for individual tracks
	return errMsg(fmt.Errorf("album track browsing requires additional implementation - try searching for individual songs from this album instead"))
}

// Enhanced album search that also finds tracks within albums
func searchAlbumWithTracks(albumTitle, artistName string) tea.Cmd {
	return func() tea.Msg {
		// Clean up the album title (remove emoji and extra formatting)
		cleanTitle := strings.TrimPrefix(albumTitle, "📀 ")
		cleanTitle = strings.TrimSpace(cleanTitle)
		
		var tracks []songItem
		albumNameLower := strings.ToLower(cleanTitle)
		artistNameLower := strings.ToLower(artistName)
		
		// Strategy 1: Search for tracks with album and artist
		searchQueries := []string{
			fmt.Sprintf("%s %s", cleanTitle, artistName),
			fmt.Sprintf("%s album %s", artistName, cleanTitle),
			fmt.Sprintf("\"%s\" \"%s\"", cleanTitle, artistName), // Exact match
			cleanTitle, // Just the album name
		}
		
		for _, searchQuery := range searchQueries {
			searchClient := ytmusic.TrackSearch(searchQuery)
			result, err := searchClient.Next()
			if err != nil {
				continue // Try next query
			}
			
			for _, track := range result.Tracks {
				// Filter tracks that belong to the specified album
				trackAlbumLower := strings.ToLower(track.Album.Name)
				trackArtistLower := strings.ToLower(strings.Join(getArtistNames(track.Artists), " "))
				
				// Check if the track's album matches our target album
				albumMatch := strings.Contains(trackAlbumLower, albumNameLower) || 
							 strings.Contains(albumNameLower, trackAlbumLower) ||
							 trackAlbumLower == albumNameLower
				
				// Also check if artist matches
				artistMatch := strings.Contains(trackArtistLower, artistNameLower) ||
							  strings.Contains(artistNameLower, trackArtistLower)
				
				if albumMatch && artistMatch {
					// Avoid duplicates and invalid tracks
					isDuplicate := false
					for _, existingTrack := range tracks {
						if existingTrack.id == track.VideoID { // YouTube Music track identifier
							isDuplicate = true
							break
						}
					}
					// Only add tracks with valid IDs
					if !isDuplicate && len(track.VideoID) >= 10 {
						tracks = append(tracks, convertYTMusicTrack(track))
					}
				}
			}
			
			// If we found tracks, we can stop searching
			if len(tracks) > 0 {
				break
			}
		}

		// Strategy 2: If we didn't find tracks by album matching, try getting a watch playlist
		// from the first track we found in any of our searches
		if len(tracks) == 0 {
			for _, searchQuery := range searchQueries {
				searchClient := ytmusic.TrackSearch(searchQuery)
				result, err := searchClient.Next()
				if err != nil || len(result.Tracks) == 0 {
					continue
				}
				
				// Try to get related tracks using GetWatchPlaylist
				watchTracks, err := ytmusic.GetWatchPlaylist(result.Tracks[0].VideoID) // Get related tracks
				if err == nil && len(watchTracks) > 0 {
					for _, track := range watchTracks {
						// Filter for tracks from the same album or artist
						trackAlbumLower := strings.ToLower(track.Album.Name)
						trackArtistLower := strings.ToLower(strings.Join(getArtistNames(track.Artists), " "))
						
						albumMatch := strings.Contains(trackAlbumLower, albumNameLower) || 
									 strings.Contains(albumNameLower, trackAlbumLower)
						artistMatch := strings.Contains(trackArtistLower, artistNameLower) ||
									  strings.Contains(artistNameLower, trackArtistLower)
						
						if albumMatch || (artistMatch && len(tracks) < 10) { // Be more lenient for artist matches
							// Avoid duplicates and invalid tracks
							isDuplicate := false
							for _, existingTrack := range tracks {
								if existingTrack.id == track.VideoID { // YouTube Music track identifier
									isDuplicate = true
									break
								}
							}
							// Only add tracks with valid IDs
							if !isDuplicate && len(track.VideoID) >= 10 {
								tracks = append(tracks, convertYTMusicTrack(track))
							}
						}
					}
					
					if len(tracks) > 0 {
						break // Found some tracks, stop searching
					}
				}
			}
		}

		if len(tracks) == 0 {
			return errMsg(fmt.Errorf("no tracks found for album: %s by %s - try searching for individual songs", cleanTitle, artistName))
		}

		return albumTracksFetchedMsg(tracks)
	}
}