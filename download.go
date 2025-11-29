package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/unki2aut/go-mpd"
)

func buildUrl(base, representationId, file string, partNum *int64) string {
	if partNum != nil {
		// $Number%05d$
		file = strings.ReplaceAll(file, "$Number%05d$", fmt.Sprintf("%05d", *partNum))
	}
	return strings.ReplaceAll(file, "$RepresentationID$", base+representationId)
}

var parts []byte

func downloadPart(url string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	parts = append(parts, body...)
}

func getFilename(set *mpd.AdaptationSet) string {
	if set.MimeType == "video/mp4" {
		return "video.mp4"
	} else if set.MimeType == "audio/mp4" {
		return "audio.mp3"
	}
	return ""
}

func downloadParts(baseUrl, representationId *string, set *mpd.AdaptationSet) {
	initUrl := buildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Initialization, nil)
	downloadPart(initUrl)

	timeline := expandTimeline(set.SegmentTemplate.SegmentTimeline.S, 1)
	for i, item := range timeline {
		url := buildUrl(*baseUrl, *representationId, *set.SegmentTemplate.Media, &item)
		downloadPart(url)
		fmt.Printf("\rDownloaded %v of %v segments (%s)", i+1, len(timeline), humanSize(int64(len(parts))))
	}

	fmt.Println("\nFinished downloading!")

	// Write to a file
	file, err := os.Create(getFilename(set))
	if err != nil {
		panic(err)
	}
	decrypted, err := decryptPart(parts)
	if err != nil {
		panic(err)
	}
	file.Write(decrypted)
	file.Close()

	// Empty parts
	parts = []byte{}
}

func downloadSubs(url string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	// Write to a file
	file, err := os.Create("subs.ass")
	if err != nil {
		panic(err)
	}
	file.Write(body)
	file.Close()
}
