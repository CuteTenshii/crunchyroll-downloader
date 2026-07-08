package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/download"
)

var (
	audioLang     = flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated for multiple (e.g. \"ja-JP,en-US\"). First is the default track")
	subtitlesLang = flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated for multiple (e.g. \"en-US,es-419\"). First is the default track")
	videoQuality  = flag.String("video-quality", "1080p", "Video quality")
	audioQuality  = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber  = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRt         = flag.String("etp-rt", "", "The \"etp_rt\" cookie value of your account")
	debug         = flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
)

func parseLangs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func processURL(client *api.Client, url string) {
	parts := strings.Split(url, "/")
	if len(parts) < 5 {
		fmt.Printf("Invalid URL format: %s\n", url)
		return
	}
	contentType := parts[3]
	contentID := parts[4]
	if len(contentID) < 9 || len(contentID) > 14 {
		fmt.Printf("Invalid URL format: %s\n", url)
		return
	}
	if contentType != "watch" && contentType != "series" {
		fmt.Printf("Invalid URL (must be /watch/ or /series/): %s\n", url)
		return
	}

	audioLangs := parseLangs(*audioLang)
	if len(audioLangs) == 0 {
		audioLangs = []string{"ja-JP"}
	}
	subsLangs := parseLangs(*subtitlesLang)

	primaryAudio := audioLangs[0]
	primarySubs := "en-US"
	if len(subsLangs) > 0 {
		primarySubs = subsLangs[0]
	}

	if contentType == "watch" {
		info, err := client.GetEpisodeInfo(contentID)
		if err != nil {
			fmt.Printf("Error fetching episode info: %v\n", err)
			return
		}
		if err := download.Episode(client, contentID, info, audioLangs, subsLangs, videoQuality, audioQuality); err != nil {
			fmt.Printf("Error downloading episode: %v\n", err)
		}
	} else {
		seasons, err := client.GetSeasons(contentID, primaryAudio, primarySubs)
		if err != nil {
			fmt.Printf("Error fetching seasons: %v\n", err)
			return
		}

		if *seasonNumber != 0 {
			var seasonID string
			for _, season := range seasons {
				if season.SeasonNumber == *seasonNumber {
					seasonID = season.ID
					break
				}
			}
			if seasonID == "" {
				fmt.Printf("This anime has no season %v!\n", *seasonNumber)
				return
			}

			episodes, err := client.GetSeasonEpisodes(seasonID, primaryAudio, primarySubs)
			if err != nil {
				fmt.Printf("Error fetching episodes: %v\n", err)
				return
			}
			if err := download.Season(client, videoQuality, audioQuality, audioLangs, subsLangs, episodes); err != nil {
				fmt.Printf("Season completed with errors: %v\n", err)
			}
		} else {
			fmt.Print("No season number specified, downloading all seasons...\n")

			for _, season := range seasons {
				episodes, err := client.GetSeasonEpisodes(season.ID, primaryAudio, primarySubs)
				if err != nil {
					fmt.Printf("Error fetching episodes for season %v: %v\n", season.SeasonNumber, err)
					continue
				}
				if err := download.Season(client, videoQuality, audioQuality, audioLangs, subsLangs, episodes); err != nil {
					fmt.Printf("Season %v completed with errors: %v\n", season.SeasonNumber, err)
				}
			}
		}
	}
}

func main() {
	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("file", "", "Path to a text file with one URL per line")
	flag.Parse()

	if *url == "" && *urlsFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	client, err := api.New(*etpRt)
	if err != nil {
		fmt.Printf("Failed to initialize API client: %v\n", err)
		os.Exit(1)
	}
	client.Debug = *debug

	if *urlsFile != "" {
		file, err := os.Open(*urlsFile)
		if err != nil {
			fmt.Printf("Failed to open URLs file: %s\n", err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var urls []string
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && strings.HasPrefix(line, "http") {
				urls = append(urls, line)
			}
		}

		fmt.Printf("Found %d URLs to download\n\n", len(urls))
		for i, u := range urls {
			fmt.Printf("=== [%d/%d] %s ===\n", i+1, len(urls), u)
			processURL(client, u)
			fmt.Println()
		}
	} else {
		processURL(client, *url)
	}
}
