package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"crunchyroll-downloader/internal/engine"
)

func main() {
	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("file", "", "Path to a text file with one URL per line")
	audioLang := flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated for multiple (e.g. \"ja-JP,en-US\"). First is the default track")
	subtitlesLang := flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated for multiple (e.g. \"en-US,es-419\"). First is the default track")
	ccLang := flag.String("cc-lang", "", "Closed caption language(s), comma-separated for multiple (e.g. \"en-US\"). Downloaded in addition to --subs-lang, not instead of it")
	videoQuality := flag.String("video-quality", "1080p", "Video quality")
	audioQuality := flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber := flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRtFile := flag.String("etp-rt-file", "", "Path to a 0600 regular file containing the etp_rt cookie")
	debug := flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
	index := flag.Bool("index", false, "Build a metadata catalog of all episodes in a series (no download). Requires a /series/ URL")
	indexSubs := flag.Bool("index-subs", false, "Like --index, but also download subtitle transcripts for every episode. Resumable")
	indexDelay := flag.Int("index-delay", 3, "Seconds to wait between subtitle acquisition attempts")
	indexPriority := flag.String("index-priority-ids", "", "Episode provider IDs to process first in --index-subs mode, comma-separated")
	indexWindow := flag.Int("index-window", 25, "Maximum provider identities attempted in one resumable index run (1-100)")
	indexTerminalRecheckWindow := flag.Int("index-terminal-recheck-window", 3, "Maximum missing-locale/permanent rows sparsely rechecked after a source version or catalog snapshot change (0-25)")
	indexCircuitLimit := flag.Int("index-circuit-4294-limit", 3, "Consecutive provider 4294 responses before the global playback circuit opens")
	indexCircuitCooldown := flag.Duration("index-circuit-cooldown", 30*time.Minute, "Global cooldown after the playback circuit opens")
	indexSummaryPath := flag.String("index-run-summary", "", "Machine-readable terminal run summary path (default: <index>.run-summary.json)")
	playback4294Retries := flag.Int("playback-4294-retries", 2, "Additional retries for playback provider error 4294")
	playback4294Backoff := flag.Duration("playback-4294-backoff", 8*time.Second, "Initial backoff for playback provider error 4294")
	flag.Parse()

	if *url == "" && *urlsFile == "" {
		flag.Usage()
		fmt.Println("provide --url or --file")
		os.Exit(1)
	}

	cfg := engine.RuntimeConfig{
		VideoQuality:               *videoQuality,
		AudioQuality:               *audioQuality,
		DebugManifest:              *debug,
		Playback4294Retries:        *playback4294Retries,
		Playback4294Backoff:        *playback4294Backoff,
		IndexDelaySeconds:          *indexDelay,
		IndexWindow:                *indexWindow,
		IndexTerminalRecheckWindow: *indexTerminalRecheckWindow,
		IndexCircuitLimit:          *indexCircuitLimit,
		IndexCircuitCooldown:       *indexCircuitCooldown,
		IndexPriorityIDs:           *indexPriority,
		IndexSummaryPath:           *indexSummaryPath,
	}
	cli := engine.CLIOptions{
		AudioLang:     *audioLang,
		SubtitlesLang: *subtitlesLang,
		CCLang:        *ccLang,
		SeasonNumber:  *seasonNumber,
		Index:         *index,
		IndexSubs:     *indexSubs,
		EtpRtFile:     *etpRtFile,
	}
	engine.Configure(cfg, cli)

	if err := engine.Run(*url, *urlsFile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
