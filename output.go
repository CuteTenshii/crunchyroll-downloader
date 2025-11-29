package main

import (
	"fmt"
	"os"
	"os/exec"
)

// mergeEverything merges audio, video and subtitles in a single MKV container
func mergeEverything(subtitlesLang *string, info EpisodeInfo) {
	args := []string{
		"-i", "video.mp4", "-i", "audio.mp3", "-c:v", "copy", "-c:a", "copy", "-disposition:s:0", "default",
		"-metadata:g", "title=" + fmt.Sprintf("S%vE%v - %s", info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.Title),
		"-metadata:g", "show=" + info.EpisodeMetadata.SeriesTitle,
		"-metadata:g", "track=" + fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber),
		"-metadata:g", "season_number=" + fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber),
	}

	if _, err := os.Stat("subs.ass"); err == nil {
		args = append(args,
			"-i", "subs.ass",
			"-c:s", "ass", // keep subtitles as ASS
			"-metadata:s:s:0", fmt.Sprintf("title=%s", languageNames[*subtitlesLang]),
		)
	}
	args = append(args, "output.mkv")

	cmd := exec.Command("ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}
