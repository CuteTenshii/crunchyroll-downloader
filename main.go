package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/config"
	"crunchyroll-downloader/internal/download"
	"crunchyroll-downloader/internal/drm"
)

var (
	audioLang     = flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated for multiple (e.g. \"ja-JP,en-US\"). First is the default track")
	subtitlesLang = flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated for multiple (e.g. \"en-US,es-419\"). First is the default track")
	videoQuality  = flag.String("video-quality", "1080p", "Video quality")
	audioQuality  = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber  = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRt         = flag.String("etp-rt", "", "The \"etp_rt\" cookie value of your account")
	debug         = flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
	workers       = flag.Int("workers", 10, "Number of concurrent segment download workers")
	outputDir     = flag.String("output-dir", "", "Custom output directory for downloads")
	widevineDev   = flag.String("widevine-device", "", "Path to .wvd file or directory with client_id.bin + private_key.pem")
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

// validateOutputDir checks that the specified output directory exists and is a
// directory. Returns an error message, or empty string if valid.
func validateOutputDir(dir string) string {
	if dir == "" {
		return "" // empty dir means use CWD default — valid
	}
	if fi, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Sprintf("Output directory %s does not exist. Create it first or omit --output-dir to use the current directory.", dir)
	} else if err != nil {
		return fmt.Sprintf("Error accessing output directory %s: %v", dir, err)
	} else if !fi.IsDir() {
		return fmt.Sprintf("Output directory %s is not a directory.", dir)
	}
	return ""
}

// invalidURL holds a URL that failed validation and the reason.
type invalidURL struct {
	URL   string
	Error string
}

// validateURL checks that the URL has a /watch/ or /series/ path with a
// content ID between 9 and 14 characters.
func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return fmt.Errorf("URL path must contain content type and ID")
	}

	contentType := parts[0]
	contentID := parts[1]

	if contentType != "watch" && contentType != "series" {
		return fmt.Errorf("URL must be /watch/ or /series/")
	}

	if len(contentID) < 9 || len(contentID) > 14 {
		return fmt.Errorf("content ID length must be 9-14 characters (got %d)", len(contentID))
	}

	return nil
}

// validateAllURLs validates all URLs upfront and returns any that failed.
func validateAllURLs(urls []string) []invalidURL {
	var invalid []invalidURL
	for _, u := range urls {
		if err := validateURL(u); err != nil {
			invalid = append(invalid, invalidURL{URL: u, Error: err.Error()})
		}
	}
	return invalid
}

func processURL(ctx context.Context, client *api.Client, rawURL string, outputDir string, audioLangs, subsLangs []string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		fmt.Printf("Invalid URL: %v\n", err)
		return
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		fmt.Printf("Invalid URL format: %s\n", rawURL)
		return
	}
	contentType := parts[0]
	contentID := parts[1]
	if len(contentID) < 9 || len(contentID) > 14 {
		fmt.Printf("Invalid URL format: %s\n", rawURL)
		return
	}
	if contentType != "watch" && contentType != "series" {
		fmt.Printf("Invalid URL (must be /watch/ or /series/): %s\n", rawURL)
		return
	}

	if len(audioLangs) == 0 {
		audioLangs = []string{"ja-JP"}
	}

	primaryAudio := audioLangs[0]
	primarySubs := "en-US"
	if len(subsLangs) > 0 {
		primarySubs = subsLangs[0]
	}

	if contentType == "watch" {
		info, err := client.GetEpisodeInfo(ctx, contentID)
		if err != nil {
			fmt.Printf("Error fetching episode info: %v\n", err)
			return
		}
		if err := download.Episode(ctx, client, contentID, info, audioLangs, subsLangs, videoQuality, audioQuality, *workers, outputDir); err != nil {
			fmt.Printf("Error downloading episode: %v\n", err)
		}
	} else {
		seasons, err := client.GetSeasons(ctx, contentID, primaryAudio, primarySubs)
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

			episodes, err := client.GetSeasonEpisodes(ctx, seasonID, primaryAudio, primarySubs)
			if err != nil {
				fmt.Printf("Error fetching episodes: %v\n", err)
				return
			}
			if err := download.Season(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, *workers, outputDir); err != nil {
				fmt.Printf("Season completed with errors: %v\n", err)
			}
		} else {
			fmt.Print("No season number specified, downloading all seasons...\n")

			for _, season := range seasons {
				episodes, err := client.GetSeasonEpisodes(ctx, season.ID, primaryAudio, primarySubs)
				if err != nil {
					fmt.Printf("Error fetching episodes for season %v: %v\n", season.SeasonNumber, err)
					continue
				}
				if err := download.Season(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, *workers, outputDir); err != nil {
					fmt.Printf("Season %v completed with errors: %v\n", season.SeasonNumber, err)
				}
			}
		}
	}
}

// checkFFmpeg validates that FFmpeg is available on the system PATH
// and can be executed. Returns an actionable error if not.
func checkFFmpeg() error {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("FFmpeg not found: install FFmpeg and ensure it is on $PATH. See https://ffmpeg.org/download.html")
	}

	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg found at %s but failed to run: %w", path, err)
	}

	return nil
}

// isAllNilConfig returns true if all pointer fields in cfg are nil,
// indicating the config file did not exist or was empty.
func isAllNilConfig(cfg *config.Config) bool {
	return cfg.AudioLang == nil &&
		cfg.SubsLang == nil &&
		cfg.VideoQuality == nil &&
		cfg.AudioQuality == nil &&
		cfg.Workers == nil &&
		cfg.OutputDir == nil &&
		cfg.EtpRt == nil &&
		cfg.WidevineDevice == nil
}

// resolveString resolves a string value through the precedence hierarchy:
// explicit CLI flag > env var > config value > default value.
// The explicitFlags map should be built via flag.Visit().
func resolveString(explicitFlags map[string]bool, flagName string, flagVal string, envName string, configVal *string, defaultVal string) string {
	if explicitFlags[flagName] {
		return flagVal
	}
	if v, ok := os.LookupEnv(envName); ok && v != "" {
		return v
	}
	if configVal != nil && *configVal != "" {
		return *configVal
	}
	return defaultVal
}

// resolveEtpRt resolves the etp_rt value through the precedence hierarchy:
// explicit --etp-rt CLI flag > CRUNCHYROLL_ETP_RT env var > config file value.
func resolveEtpRt(explicitFlags map[string]bool, flagVal string, configVal *string) string {
	if explicitFlags["etp-rt"] && flagVal != "" {
		return flagVal
	}
	if v, ok := os.LookupEnv("CRUNCHYROLL_ETP_RT"); ok && v != "" {
		return v
	}
	if configVal != nil && *configVal != "" {
		return *configVal
	}
	return ""
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("file", "", "Path to a text file with one URL per line")
	flag.Parse()

	if *url == "" && *urlsFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Track explicitly-set flags via flag.Visit()
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// Resolve config path
	cfgPath, err := config.ConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot determine config path: %v — skipping config file\n", err)
	}

	// Load config (if config path was resolved)
	cfg := &config.Config{}
	if err == nil {
		cfg, err = config.Load(cfgPath)
		if err != nil {
			// Invalid JSON: warn and continue with defaults
			fmt.Fprintf(os.Stderr, "Warning: %v — using defaults\n", err)
			cfg = &config.Config{}
		} else if isAllNilConfig(cfg) {
			// Config file does not exist: create skeleton
			if err := config.WriteSkeleton(cfgPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create default config: %v\n", err)
			} else {
				fmt.Printf("Created default config at %s\n", cfgPath)
			}
		}
	}

	// Resolve precedence: CLI flag > env var > config file > default
	resolvedEtpRt := resolveEtpRt(explicitFlags, *etpRt, cfg.EtpRt)
	resolvedOutputDir := resolveString(explicitFlags, "output-dir", *outputDir, "", cfg.OutputDir, "")

	// Validate FFmpeg availability before any download (D-18, D-19)
	if err := checkFFmpeg(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Parse language flags once after config resolution (QOL-08, D-22)
	audioLangs := parseLangs(*audioLang)
	subsLangs := parseLangs(*subtitlesLang)

	// Resolve Widevine device path through precedence and set it before
	// any API client call, ensuring sync.Once uses the correct path.
	resolvedWidevineDev := resolveString(explicitFlags, "widevine-device", *widevineDev, "WIDEVINE_DEVICE_PATH", cfg.WidevineDevice, "")
	if resolvedWidevineDev != "" {
		drm.SetWidevinePath(resolvedWidevineDev)
	}

	client, err := api.NewWithContext(ctx, resolvedEtpRt)
	if err != nil {
		fmt.Printf("Failed to initialize API client: %v\n", err)
		os.Exit(1)
	}
	client.Debug = *debug
	// Validate output directory exists if specified (D-11)
	if errMsg := validateOutputDir(resolvedOutputDir); errMsg != "" {
		fmt.Println(errMsg)
		os.Exit(1)
	}

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

		// Validate all URLs upfront before any downloads (D-17)
		if invalid := validateAllURLs(urls); len(invalid) > 0 {
			fmt.Println("Invalid URLs found:")
			for _, inv := range invalid {
				fmt.Printf("  %s — %s\n", inv.URL, inv.Error)
			}
			os.Exit(1)
		}

		fmt.Printf("Found %d URLs to download\n\n", len(urls))
		for i, u := range urls {
			fmt.Printf("=== [%d/%d] %s ===\n", i+1, len(urls), u)
			processURL(ctx, client, u, resolvedOutputDir, audioLangs, subsLangs)
			fmt.Println()
		}
	} else {
		processURL(ctx, client, *url, resolvedOutputDir, audioLangs, subsLangs)
	}
}
