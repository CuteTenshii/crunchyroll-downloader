package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	token         = ""
	audioLang     = flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated for multiple (e.g. \"ja-JP,en-US\"). First is the default track")
	subtitlesLang = flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated for multiple (e.g. \"en-US,es-419\"). First is the default track")
	ccLang        = flag.String("cc-lang", "", "Closed caption language(s), comma-separated for multiple (e.g. \"en-US\"). Downloaded in addition to --subs-lang, not instead of it")
	videoQuality  = flag.String("video-quality", "1080p", "Video quality")
	audioQuality  = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber  = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRt         = flag.String("etp-rt", "", "The \"etp_rt\" cookie value of your account")
	debug         = flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
)

// parseLangs splits a comma-separated locale list, trimming spaces and dropping
// empties.
func parseLangs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseUrl extracts the content type ("watch" or "series") and content ID from a
// Crunchyroll URL, tolerating an optional locale prefix such as "/fr/". The
// content type and ID may appear at any position after the host.
func parseUrl(url string) (contentType, contentId string) {
	parts := strings.Split(strings.TrimRight(url, "/"), "/")
	for i, p := range parts {
		if (p == "watch" || p == "series") && i+1 < len(parts) {
			return p, parts[i+1]
		}
	}
	return "", ""
}

func processUrl(url string) {
	contentType, contentId := parseUrl(url)
	if contentType == "" || contentId == "" {
		fmt.Printf("Invalid URL (must be /watch/ or /series/): %s\n", url)
		return
	}

	audioLangs := parseLangs(*audioLang)
	if len(audioLangs) == 0 {
		audioLangs = []string{"ja-JP"}
	}
	subsLangs := parseLangs(*subtitlesLang)
	ccLangs := parseLangs(*ccLang)

	// The season/series API endpoints take a single preferred locale; use the
	// primary (first) requested one. All dub versions are still listed per
	// episode, so the other languages remain resolvable.
	primaryAudio := audioLangs[0]
	primarySubs := "en-US"
	if len(subsLangs) > 0 {
		primarySubs = subsLangs[0]
	}

	if contentType == "watch" {
		info := getEpisodeInfo(contentId)
		downloadEpisode(contentId, info, audioLangs, subsLangs, ccLangs, videoQuality, audioQuality)
	} else {
		seasons := getSeasons(contentId, primaryAudio, primarySubs)

		if *seasonNumber != 0 {
			var seasonId string
			for _, season := range seasons {
				if season.SeasonNumber == *seasonNumber {
					seasonId = season.ID
					break
				}
			}
			if seasonId == "" {
				fmt.Printf("This anime has no season %v!\n", *seasonNumber)
				return
			}

			episodes := getSeasonEpisodes(seasonId, primaryAudio, primarySubs)
			downloadSeason(videoQuality, audioQuality, audioLangs, subsLangs, ccLangs, episodes)
		} else {
			print("No season number specified, downloading all seasons...\n")

			for _, season := range seasons {
				episodes := getSeasonEpisodes(season.ID, primaryAudio, primarySubs)
				downloadSeason(videoQuality, audioQuality, audioLangs, subsLangs, ccLangs, episodes)
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

	if *etpRt == "" {
		fmt.Println("You must specify the \"-etp-rt\" option!\n- Open Crunchyroll on your browser and log in.\n- Open developer tools (Ctrl+Shift+I), go to \"Application\", and then \"Cookies\".\n- The value of the \"ept_rt\" cookie is what you need to input into this option.")
		os.Exit(1)
	}

	token = GetAccessToken(*etpRt)

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
			processUrl(u)
			fmt.Println()
		}
	} else {
		processUrl(*url)
	}
}
