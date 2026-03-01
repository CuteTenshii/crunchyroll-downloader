package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
)

var (
	token         = ""
	audioLang     = flag.String("audio-lang", "ja-JP", "Audio language")
	subtitlesLang = flag.String("subs-lang", "en-US", "Subtitles language")
	videoQuality  = flag.String("video-quality", "1080p", "Video quality")
	audioQuality  = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber  = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRt         = flag.String("etp-rt", "", "The \"etp_rt\" cookie value of your account")
)

func processUrl(url string) {
	contentType := strings.Split(url, "/")[3]
	contentId := strings.Split(url, "/")[4]
	if len(contentId) != 9 && len(contentId) != 14 {
		fmt.Printf("Invalid URL format: %s\n", url)
		return
	}
	if contentType != "watch" && contentType != "series" {
		fmt.Printf("Invalid URL (must be /watch/ or /series/): %s\n", url)
		return
	}

	if contentType == "watch" {
		info := getEpisodeInfo(contentId)
		if info.EpisodeMetadata.AudioLocale != *audioLang {
			correctGuidI := slices.IndexFunc(info.EpisodeMetadata.Versions, func(v *DubVersion) bool {
				return v.AudioLocale == *audioLang
			})

			if correctGuidI == -1 {
				print("! Invalid audio locale. Please put the locale in the \"ja-JP\", \"en-US\"... format.\n")
				return
			}
			correctGuid := info.EpisodeMetadata.Versions[correctGuidI]
			contentId = (*correctGuid).GUID
		}

		downloadEpisode(contentId, videoQuality, audioQuality, subtitlesLang, info)
	} else {
		seasons := getSeasons(contentId)

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

			episodes := getSeasonEpisodes(seasonId)
			downloadSeason(videoQuality, audioQuality, subtitlesLang, episodes)
		} else {
			print("No season number specified, downloading all seasons...\n")

			for _, season := range seasons {
				episodes := getSeasonEpisodes(season.ID)
				downloadSeason(videoQuality, audioQuality, subtitlesLang, episodes)
			}
		}
	}
}

func main() {
	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("urls", "", "Path to a text file with one URL per line")
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
