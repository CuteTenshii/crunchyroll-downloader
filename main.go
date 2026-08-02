package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPlayback4294Retries        = 2
	defaultPlayback4294Backoff        = 8 * time.Second
	maxPlayback4294Retries            = 5
	maxPlayback4294Backoff            = time.Minute
	defaultIndexWindow                = 25
	defaultIndexTerminalRecheckWindow = 3
	defaultIndexCircuitLimit          = 3
	defaultIndexCooldown              = 30 * time.Minute
)

var (
	token                      = ""
	audioLang                  = flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated for multiple (e.g. \"ja-JP,en-US\"). First is the default track")
	subtitlesLang              = flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated for multiple (e.g. \"en-US,es-419\"). First is the default track")
	ccLang                     = flag.String("cc-lang", "", "Closed caption language(s), comma-separated for multiple (e.g. \"en-US\"). Downloaded in addition to --subs-lang, not instead of it")
	videoQuality               = flag.String("video-quality", "1080p", "Video quality")
	audioQuality               = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber               = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRtFile                  = flag.String("etp-rt-file", "", "Path to a 0600 regular file containing the etp_rt cookie")
	debug                      = flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
	index                      = flag.Bool("index", false, "Build a metadata catalog of all episodes in a series (no download). Requires a /series/ URL")
	indexSubs                  = flag.Bool("index-subs", false, "Like --index, but also download subtitle transcripts for every episode. Resumable")
	indexDelay                 = flag.Int("index-delay", 3, "Seconds to wait between subtitle acquisition attempts")
	indexPriority              = flag.String("index-priority-ids", "", "Episode provider IDs to process first in --index-subs mode, comma-separated")
	indexWindow                = flag.Int("index-window", defaultIndexWindow, "Maximum provider identities attempted in one resumable index run (1-100)")
	indexTerminalRecheckWindow = flag.Int("index-terminal-recheck-window", defaultIndexTerminalRecheckWindow, "Maximum missing-locale/permanent rows sparsely rechecked after a source version or catalog snapshot change (0-25)")
	indexCircuitLimit          = flag.Int("index-circuit-4294-limit", defaultIndexCircuitLimit, "Consecutive provider 4294 responses before the global playback circuit opens")
	indexCircuitCooldown       = flag.Duration("index-circuit-cooldown", defaultIndexCooldown, "Global cooldown after the playback circuit opens")
	indexSummaryPath           = flag.String("index-run-summary", "", "Machine-readable terminal run summary path (default: <index>.run-summary.json)")
	playback4294Retries        = flag.Int("playback-4294-retries", defaultPlayback4294Retries, "Additional retries for playback provider error 4294")
	playback4294Backoff        = flag.Duration("playback-4294-backoff", defaultPlayback4294Backoff, "Initial backoff for playback provider error 4294")
	getProcessEpisodeInfo      = getEpisodeInfo
)

const maxETPRTBytes int64 = 16 << 10

// CredentialFileError makes unsafe credential-file rejections actionable
// without exposing the cookie itself.
type CredentialFileError struct {
	Path    string
	Problem string
}

func (e *CredentialFileError) Error() string {
	return fmt.Sprintf("unsafe etp_rt credential file %s: %s", e.Path, e.Problem)
}

func readETPRTFile(path string) (string, error) {
	entryInfo, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("stat etp_rt credential file: %w", err)
	}
	if err := validateCredentialFileInfo(path, entryInfo); err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open etp_rt credential file: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat opened etp_rt credential file: %w", err)
	}
	if err := validateOpenedCredentialFile(path, entryInfo, openedInfo); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(file, maxETPRTBytes+1))
	if err != nil {
		return "", fmt.Errorf("read etp_rt credential file: %w", err)
	}
	if int64(len(data)) > maxETPRTBytes {
		return "", &CredentialFileError{Path: path, Problem: "size is outside the permitted range"}
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", &CredentialFileError{Path: path, Problem: "is empty"}
	}
	return secret, nil
}

func validateCredentialFileInfo(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return &CredentialFileError{Path: path, Problem: "must not be a symlink"}
	}
	if !info.Mode().IsRegular() {
		return &CredentialFileError{Path: path, Problem: "must be a regular file"}
	}
	if !isPrivateCredentialMode(info.Mode()) {
		return &CredentialFileError{Path: path, Problem: "permissions must be 0600"}
	}
	if info.Size() <= 0 || info.Size() > maxETPRTBytes {
		return &CredentialFileError{Path: path, Problem: "size is outside the permitted range"}
	}
	return nil
}

func validateOpenedCredentialFile(path string, entryInfo, openedInfo os.FileInfo) error {
	if err := validateCredentialFileInfo(path, openedInfo); err != nil {
		return err
	}
	if !os.SameFile(entryInfo, openedInfo) {
		return &CredentialFileError{Path: path, Problem: "changed after validation"}
	}
	return nil
}

func loadETPRT(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("provide --etp-rt-file with a 0600 regular file")
	}
	return readETPRTFile(filepath.Clean(filePath))
}

func validatePlaybackRetryConfig(retries int, backoff time.Duration) error {
	if retries < 0 || retries > maxPlayback4294Retries {
		return fmt.Errorf("--playback-4294-retries must be between 0 and %d", maxPlayback4294Retries)
	}
	if backoff <= 0 || backoff > maxPlayback4294Backoff {
		return fmt.Errorf("--playback-4294-backoff must be greater than 0 and at most %s", maxPlayback4294Backoff)
	}
	return nil
}

func validateIndexRunConfig(window, circuitLimit int, cooldown time.Duration) error {
	if window < 1 || window > 100 {
		return fmt.Errorf("--index-window must be between 1 and 100")
	}
	if circuitLimit < 1 || circuitLimit > 10 {
		return fmt.Errorf("--index-circuit-4294-limit must be between 1 and 10")
	}
	if cooldown < time.Minute || cooldown > 24*time.Hour {
		return fmt.Errorf("--index-circuit-cooldown must be between 1m and 24h")
	}
	return nil
}

func validateIndexTerminalRecheckWindow(window int) error {
	if window < 0 || window > 25 {
		return fmt.Errorf("--index-terminal-recheck-window must be between 0 and 25")
	}
	return nil
}

func validateBatchIndexSummaryPath(urlCount int, fetchSubs bool, summaryPath string) error {
	if fetchSubs && urlCount > 1 && strings.TrimSpace(summaryPath) != "" {
		return errors.New("--index-run-summary cannot be shared by multiple --file URLs; omit it to use per-catalog summary paths")
	}
	return nil
}

func beginBatchIndexCatalogMetrics(fetchSubs bool) {
	if fetchSubs {
		resetProviderCallMetrics()
	}
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

func processUrl(url string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			switch value := recovered.(type) {
			case error:
				err = fmt.Errorf("process %s: %w", url, value)
			default:
				err = fmt.Errorf("process %s: %v", url, value)
			}
		}
	}()

	parts := strings.Split(url, "/")
	if len(parts) < 5 {
		return fmt.Errorf("Invalid URL format: %s", url)
	}
	contentType := parts[3]
	contentId := parts[4]
	if len(contentId) < 9 || len(contentId) > 14 {
		return fmt.Errorf("Invalid URL format: %s", url)
	}
	if contentType != "watch" && contentType != "series" {
		return fmt.Errorf("Invalid URL (must be /watch/ or /series/): %s", url)
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

	// Index mode: build a metadata catalog (optionally with subtitles) and exit
	// without downloading any video. Only meaningful for /series/ URLs.
	if *index || *indexSubs {
		if contentType != "series" {
			return errors.New("--index/--index-subs requires a /series/ URL")
		}
		return writeIndex(url, contentId, primaryAudio, primarySubs, *indexSubs)
	}

	if contentType == "watch" {
		info := getProcessEpisodeInfo(contentId)
		if err := downloadEpisode(contentId, info, audioLangs, subsLangs, ccLangs, videoQuality, audioQuality); err != nil {
			return err
		}
	} else {
		seasons := getSeasons(contentId, primaryAudio, primarySubs)
		if len(seasons) == 0 {
			return errors.New("no seasons found")
		}

		if *seasonNumber != 0 {
			var seasonId string
			for _, season := range seasons {
				if season.SeasonNumber == *seasonNumber {
					seasonId = season.ID
					break
				}
			}
			if seasonId == "" {
				return fmt.Errorf("This anime has no season %v!", *seasonNumber)
			}

			episodes := getSeasonEpisodes(seasonId, primaryAudio, primarySubs)
			if err := downloadSeason(videoQuality, audioQuality, audioLangs, subsLangs, ccLangs, episodes); err != nil {
				return err
			}
		} else {
			fmt.Println("No season number specified, downloading all seasons...")

			for _, season := range seasons {
				episodes := getSeasonEpisodes(season.ID, primaryAudio, primarySubs)
				if err := downloadSeason(videoQuality, audioQuality, audioLangs, subsLangs, ccLangs, episodes); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func run(url, urlsFile string) error {
	if url == "" && urlsFile == "" {
		return errors.New("provide --url or --file")
	}
	if err := validatePlaybackRetryConfig(*playback4294Retries, *playback4294Backoff); err != nil {
		return err
	}
	if (*index || *indexSubs) && *indexSubs {
		if err := validateIndexRunConfig(*indexWindow, *indexCircuitLimit, *indexCircuitCooldown); err != nil {
			return err
		}
		if err := validateIndexTerminalRecheckWindow(*indexTerminalRecheckWindow); err != nil {
			return err
		}
		resetProviderCallMetrics()
	}

	etpRT, err := loadETPRT(*etpRtFile)
	if err != nil {
		return fmt.Errorf("Authentication setup failed: %w", err)
	}
	setETPRT(etpRT)
	if err := refreshAccessToken(); err != nil {
		return fmt.Errorf("Authentication failed: %w", err)
	}

	if urlsFile != "" {
		file, err := os.Open(urlsFile)
		if err != nil {
			return fmt.Errorf("Failed to open URLs file: %w", err)
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
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read URLs file: %w", err)
		}
		if err := validateBatchIndexSummaryPath(len(urls), *indexSubs, *indexSummaryPath); err != nil {
			return err
		}

		fmt.Printf("Found %d URLs to download\n\n", len(urls))
		failed := false
		for i, u := range urls {
			beginBatchIndexCatalogMetrics(*indexSubs)
			fmt.Printf("=== [%d/%d] %s ===\n", i+1, len(urls), u)
			if err := processUrl(u); err != nil {
				fmt.Printf("! %s\n", err)
				failed = true
			}
			fmt.Println()
		}
		if failed {
			return errors.New("one or more URLs failed")
		}
		return nil
	}

	return processUrl(url)
}

func main() {
	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("file", "", "Path to a text file with one URL per line")
	flag.Parse()

	if *url == "" && *urlsFile == "" {
		flag.Usage()
		fmt.Println("provide --url or --file")
		os.Exit(1)
	}
	if err := run(*url, *urlsFile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
