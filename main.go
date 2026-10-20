package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	token         = ""
	audioLang     = flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated (e.g. \"ja-JP,en-US\"). Add ALL for every available dub; explicit languages lead, the first available track is default, and ALL alone prefers ja-JP")
	subtitlesLang = flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated (e.g. \"fr-FR,ALL\"). Add ALL for every available subtitle; explicit languages lead, the first available track is default, and ALL alone prefers en-US")
	ccLang        = flag.String("cc-lang", "", "Closed caption language(s), comma-separated (e.g. \"en-US,ALL\"). Add ALL for every available caption; downloaded in addition to --subs-lang")
	videoQuality  = flag.String("video-quality", "1080p", "Video quality")
	audioQuality  = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber  = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRt         = flag.String("etp-rt", "", "The \"etp_rt\" cookie value of your account")
	debug         = flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
	downloadDelay = flag.Duration("download-delay", 0, "Minimum delay between episode downloads, to help avoid Crunchyroll's rate limiting (e.g. \"30s\", \"2m\")")
	output        = flag.String("output-dir", ".", "Target base path for downloading")
	tempDir       = flag.String("temp-dir", "", "Directory for temporary download files (video/audio/subtitles before merging). Defaults to $TMPDIR or the OS default temp directory; set this if that default is too small or slow for multi-GB downloads")
)

// backoff spaces out consecutive episode downloads by *downloadDelay. It is
// initialized in main() once flags are parsed, and is a no-op (including when
// left nil) whenever no delay is configured.
var backoff *downloadBackoff

// configureTempDir points os.CreateTemp (used for every decrypted video,
// audio and subtitle file during a download) at dir when set, so a download
// isn't limited to whatever the OS default temp directory is. Some setups
// default to a small or RAM-backed temp dir (e.g. a tmpfs /tmp), which a
// season's worth of 1080p episodes downloading concurrently can easily
// exceed, surfacing as "disk quota exceeded" mid-download. Leaves TMPDIR
// alone when dir is empty, so the OS/user default still applies.
func configureTempDir(dir string) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0777); err != nil {
		fmt.Printf("! Couldn't create -temp-dir %q, falling back to the system default temp dir: %v\n", dir, err)
		return
	}
	_ = os.Setenv("TMPDIR", dir)
}

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

// preferredRequestLocale provides a real locale for the season API even when
// ALL is requested; track selection still happens against each episode later.
func preferredRequestLocale(langs []string, fallback string) string {
	for _, locale := range langs {
		if !strings.EqualFold(locale, "all") {
			return locale
		}
	}
	return fallback
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
	// first explicit request or the usual default for ALL alone. Track
	// availability is resolved from each episode's dub versions later.
	primaryAudio := preferredRequestLocale(audioLangs, "ja-JP")
	primarySubs := preferredRequestLocale(subsLangs, "en-US")

	if contentType == "watch" {
		info := getEpisodeInfo(contentId)
		downloadEpisodeWithRetry(contentId, info, audioLangs, subsLangs, ccLangs, videoQuality, audioQuality, *output)
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
			downloadSeason(videoQuality, audioQuality, audioLangs, subsLangs, ccLangs, episodes, *output)
		} else {
			print("No season number specified, downloading all seasons...\n")

			for _, season := range seasons {
				episodes := getSeasonEpisodes(season.ID, primaryAudio, primarySubs)
				downloadSeason(videoQuality, audioQuality, audioLangs, subsLangs, ccLangs, episodes, *output)
			}
		}
	}
}

func main() {
	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("file", "", "Path to a text file with one URL per line")
	flag.Parse()

	configureTempDir(*tempDir)

	if *url == "" && *urlsFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *etpRt == "" {
		fmt.Println("You must specify the \"-etp-rt\" option!\n- Open Crunchyroll on your browser and log in.\n- Open developer tools (Ctrl+Shift+I), go to \"Application\", and then \"Cookies\".\n- The value of the \"ept_rt\" cookie is what you need to input into this option.")
		os.Exit(1)
	}

	// Checked up front so a missing ffmpeg fails before anything is downloaded,
	// not at merge time after every track has been fetched.
	ffmpeg, err := findFFmpeg()
	if err != nil {
		fmt.Println("FFmpeg wasn't found. Install it and add it to your PATH, or put the ffmpeg executable in the same folder as this program.\nDownload it from https://www.ffmpeg.org/download.html")
		os.Exit(1)
	}
	ffmpegPath = ffmpeg

	token, err = GetAccessToken(*etpRt)
	if errors.Is(err, errAuthRejected) {
		fmt.Printf("Couldn't log in: %s\nYour \"-etp-rt\" cookie is wrong or expired. Copy a fresh one from your browser (see the README).\n", err)
		os.Exit(1)
	}
	if err != nil {
		fmt.Printf("Couldn't log in: %s\n", err)
		os.Exit(1)
	}
	backoff = newDownloadBackoff(*downloadDelay)

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
