package engine

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CLIOptions holds per-run selection options (languages, season, index mode,
// credential path) that are not part of RuntimeConfig quality/retry settings.
type CLIOptions struct {
	AudioLang     string
	SubtitlesLang string
	CCLang        string
	SeasonNumber  int
	Index         bool
	IndexSubs     bool
	EtpRtFile     string
}

// DefaultCLIOptions returns the same language/season defaults the CLI flags used.
func DefaultCLIOptions() CLIOptions {
	return CLIOptions{
		AudioLang:     "ja-JP",
		SubtitlesLang: "en-US",
	}
}

// activeCLI is process-wide CLI selection state. Configure sets it before Run.
var activeCLI = DefaultCLIOptions()

// Configure applies runtime and CLI options for the next Run/processUrl call.
func Configure(cfg RuntimeConfig, cli CLIOptions) {
	activeConfig = cfg
	activeCLI = cli
}

var getProcessEpisodeInfo = getEpisodeInfo

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

// isValidContentID accepts Crunchyroll watch/series provider IDs.
// Movies and some catalog rows use longer IDs than classic episodes.
func isValidContentID(id string) bool {
	if len(id) < 6 || len(id) > 32 {
		return false
	}
	for _, r := range id {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
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

	// Strip query/fragment so pasted browser URLs still parse.
	if i := strings.IndexAny(url, "?#"); i >= 0 {
		url = url[:i]
	}
	url = strings.TrimRight(url, "/")

	parts := strings.Split(url, "/")
	if len(parts) < 5 {
		return fmt.Errorf("Invalid URL format: %s", url)
	}
	contentType := parts[3]
	contentId := parts[4]
	// Crunchyroll IDs vary by content type: classic episode IDs are ~9 chars
	// (e.g. GWDU82Z05), locale-tagged IDs ~14 (GE00198973JAJP), and movies can
	// be 16+ (GMEE00374450JAJP). Accept a broad alphanumeric range rather than
	// a hard 14-char cap that rejects movies.
	if !isValidContentID(contentId) {
		return fmt.Errorf("Invalid URL format (bad content id %q): %s", contentId, url)
	}
	if contentType != "watch" && contentType != "series" {
		return fmt.Errorf("Invalid URL (must be /watch/ or /series/): %s", url)
	}

	audioLangs := parseLangs(activeCLI.AudioLang)
	if len(audioLangs) == 0 {
		audioLangs = []string{"ja-JP"}
	}
	subsLangs := parseLangs(activeCLI.SubtitlesLang)
	ccLangs := parseLangs(activeCLI.CCLang)

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
	if activeCLI.Index || activeCLI.IndexSubs {
		if contentType != "series" {
			return errors.New("--index/--index-subs requires a /series/ URL")
		}
		return writeIndex(url, contentId, primaryAudio, primarySubs, activeCLI.IndexSubs)
	}

	videoQuality := activeConfig.VideoQuality
	audioQuality := activeConfig.AudioQuality

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

		if activeCLI.SeasonNumber != 0 {
			var seasonId string
			for _, season := range seasons {
				if season.SeasonNumber == activeCLI.SeasonNumber {
					seasonId = season.ID
					break
				}
			}
			if seasonId == "" {
				return fmt.Errorf("This anime has no season %v!", activeCLI.SeasonNumber)
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

// Run authenticates and processes a single URL or a batch file of URLs.
// Configure must be called first (or defaults apply).
func Run(url, urlsFile string) error {
	if url == "" && urlsFile == "" {
		return errors.New("provide --url or --file")
	}
	if err := validatePlaybackRetryConfig(activeConfig.Playback4294Retries, activeConfig.Playback4294Backoff); err != nil {
		return err
	}
	if (activeCLI.Index || activeCLI.IndexSubs) && activeCLI.IndexSubs {
		if err := validateIndexRunConfig(activeConfig.IndexWindow, activeConfig.IndexCircuitLimit, activeConfig.IndexCircuitCooldown); err != nil {
			return err
		}
		if err := validateIndexTerminalRecheckWindow(activeConfig.IndexTerminalRecheckWindow); err != nil {
			return err
		}
		resetProviderCallMetrics()
	}

	etpRT, err := loadETPRT(activeCLI.EtpRtFile)
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
		if err := validateBatchIndexSummaryPath(len(urls), activeCLI.IndexSubs, activeConfig.IndexSummaryPath); err != nil {
			return err
		}

		fmt.Printf("Found %d URLs to download\n\n", len(urls))
		failed := false
		for i, u := range urls {
			beginBatchIndexCatalogMetrics(activeCLI.IndexSubs)
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
