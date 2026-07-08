package mux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"crunchyroll-downloader/internal/api"
	loc "crunchyroll-downloader/internal/locale"
)

type MediaTrack struct {
	File   string
	Locale string
}

func TrackTitle(code string) string {
	if name, ok := loc.LanguageNames[code]; ok {
		return name
	}
	return code
}

func MergeEverything(videoFile string, audioTracks, subTracks []MediaTrack, outputFile string, info *api.EpisodeInfo) {
	args := []string{"-i", videoFile}
	for _, audio := range audioTracks {
		args = append(args, "-i", audio.File)
	}
	for _, sub := range subTracks {
		args = append(args, "-i", sub.File)
	}

	args = append(args, "-map", "0:v:0")
	for i := range audioTracks {
		args = append(args, "-map", fmt.Sprintf("%d:a:0", 1+i))
	}
	for j := range subTracks {
		args = append(args, "-map", fmt.Sprintf("%d", 1+len(audioTracks)+j))
	}

	args = append(args, "-c:v", "copy", "-c:a", "copy")
	if len(subTracks) > 0 {
		args = append(args, "-c:s", "copy")
	}

	for i, audio := range audioTracks {
		args = append(args,
			fmt.Sprintf("-metadata:s:a:%d", i), "language="+loc.LanguageCodes[audio.Locale],
			fmt.Sprintf("-metadata:s:a:%d", i), "title="+TrackTitle(audio.Locale),
		)
	}
	for j, sub := range subTracks {
		args = append(args,
			fmt.Sprintf("-metadata:s:s:%d", j), "language="+loc.LanguageCodes[sub.Locale],
			fmt.Sprintf("-metadata:s:s:%d", j), "title="+TrackTitle(sub.Locale),
		)
	}

	for i := range audioTracks {
		disposition := "0"
		if i == 0 {
			disposition = "default"
		}
		args = append(args, fmt.Sprintf("-disposition:a:%d", i), disposition)
	}
	for j := range subTracks {
		disposition := "0"
		if j == 0 {
			disposition = "default"
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

	_ = os.Remove(videoFile)
	for _, audio := range audioTracks {
		_ = os.Remove(audio.File)
	}
	for _, sub := range subTracks {
		_ = os.Remove(sub.File)
	}

	fmt.Printf("\nDownload finished! Output file: %s\n\n", outputFile)
}
