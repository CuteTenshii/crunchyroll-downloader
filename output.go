package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// mediaTrack pairs a downloaded temporary file with the locale it represents.
type mediaTrack struct {
	file   string
	locale string
	format string // subtitle source format, e.g. "ass" or "vtt". Unused for audio tracks.
	isCC   bool   // true if this track is a closed caption rather than a regular subtitle
}

// trackTitle returns a human-readable track name for a locale, falling back to
// the raw locale when it isn't in the known list.
func trackTitle(locale string) string {
	if name, ok := languageNames[locale]; ok {
		return name
	}
	return locale
}

// mergeEverything merges the video, all audio tracks and all subtitle tracks
// into a single MKV container.
func mergeEverything(videoFile string, audioTracks, subTracks []mediaTrack, outputFile string, info EpisodeInfo) {
	args := []string{"-i", videoFile}
	for _, audio := range audioTracks {
		args = append(args, "-i", audio.file)
	}
	for _, sub := range subTracks {
		args = append(args, "-i", sub.file)
	}

	// Map every input stream explicitly; without this ffmpeg keeps only one
	// stream of each type.
	args = append(args, "-map", "0:v:0")
	for i := range audioTracks {
		args = append(args, "-map", fmt.Sprintf("%d:a:0", 1+i))
	}
	for j := range subTracks {
		args = append(args, "-map", fmt.Sprintf("%d", 1+len(audioTracks)+j))
	}

	args = append(args, "-c:v", "copy", "-c:a", "copy")
	// WebVTT cue-positioning metadata (e.g. "line:90% align:center") isn't
	// rendered correctly by most players when copied as-is into an MKV, so
	// VTT subtitle/caption tracks are transcoded to SRT (which has no
	// positioning syntax) instead of stream-copied. ASS tracks keep "copy"
	// to preserve their styling.
	for j, sub := range subTracks {
		if sub.format == "vtt" {
			args = append(args, fmt.Sprintf("-c:s:%d", j), "srt")
		} else {
			args = append(args, fmt.Sprintf("-c:s:%d", j), "copy")
		}
	}

	for i, audio := range audioTracks {
		args = append(args,
			fmt.Sprintf("-metadata:s:a:%d", i), "language="+languageCodes[audio.locale],
			fmt.Sprintf("-metadata:s:a:%d", i), "title="+trackTitle(audio.locale),
		)
	}
	for j, sub := range subTracks {
		title := trackTitle(sub.locale)
		if sub.isCC {
			title += " [CC]"
		}
		args = append(args,
			fmt.Sprintf("-metadata:s:s:%d", j), "language="+languageCodes[sub.locale],
			fmt.Sprintf("-metadata:s:s:%d", j), "title="+title,
		)
	}
	// Mark only the first audio track and the first non-CC subtitle track as
	// default. Disposition must be set on every track: each downloaded audio
	// file is a standalone default stream, so the non-primary ones must be
	// explicitly cleared. Closed captions are never marked default; they're
	// an opt-in extra a viewer toggles deliberately.
	for i := range audioTracks {
		disposition := "0"
		if i == 0 {
			disposition = "default"
		}
		args = append(args, fmt.Sprintf("-disposition:a:%d", i), disposition)
	}
	defaultSubSet := false
	for j, sub := range subTracks {
		disposition := "0"
		if !sub.isCC && !defaultSubSet {
			disposition = "default"
			defaultSubSet = true
		}
		args = append(args, fmt.Sprintf("-disposition:s:%d", j), disposition)
	}

	args = append(args,
		"-metadata:g", "title="+fmt.Sprintf("S%02vE%02v - %s", info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.Title),
		"-metadata:g", "show="+info.EpisodeMetadata.SeriesTitle,
		"-metadata:g", "track="+fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber),
		"-metadata:g", "season_number="+fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber),
		outputFile,
	)

	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(outputFile)
		panic(fmt.Sprintf("ffmpeg failed: %s\n%s", err, stderr.String()))
	}

	// Remove temporary files
	_ = os.Remove(videoFile)
	for _, audio := range audioTracks {
		_ = os.Remove(audio.file)
	}
	for _, sub := range subTracks {
		_ = os.Remove(sub.file)
	}

	fmt.Printf("\nDownload finished! Output file: %s\n\n", outputFile)
}
