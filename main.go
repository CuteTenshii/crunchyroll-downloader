package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
)

var token = ""

func main() {
	url := flag.String("url", "", "URL of the episode/season to download")
	audioLang := flag.String("audio-lang", "jp-JP", "Audio language")
	//videoLang := flag.String("video-lang", "en", "Video language")
	subtitlesLang := flag.String("subtitles-lang", "en-US", "Subtitles language")
	videoQuality := flag.String("video-quality", "1080p", "Video quality")
	audioQuality := flag.String("audio-quality", "192k", "Audio quality")
	etpRt := flag.String("etp-rt", "", "Idk what this means. This is the cookie value on your browser tho")
	flag.Parse()

	if *url == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *etpRt == "" {
		fmt.Println("You must specify -etp-rt:\n- Open Crunchyroll on your browser, log in.\n- Open developer tools (Ctrl+Shift+I), go to \"Application\", and then \"Cookies\".\n- The value of the \"ept_rt\" cookie is what you need to input into this option.")
		os.Exit(1)
	}

	contentId := strings.Split(*url, "/")[4]
	if len(contentId) != 9 {
		log.Println("Invalid URL format, please paste a link like this: https://www.crunchyroll.com/watch/GWDU82Z05/water-hashira-giyu-tomiokas-pain")
		os.Exit(1)
	}

	// Fetch Crunchyroll access token
	token = getAccessToken(*etpRt)
	fmt.Println("Got token")

	// Fetch some things
	info := getEpisodeInfo(contentId)
	if info.EpisodeMetadata.AudioLocale != *audioLang {
		// Run though info.EpisodeMetadata.Versions to find the correct episode GUID
		correctGuidI := slices.IndexFunc(info.EpisodeMetadata.Versions, func(v *DubVersion) bool {
			return v.AudioLocale == *audioLang
		})
		correctGuid := info.EpisodeMetadata.Versions[correctGuidI]
		if correctGuid == nil {
			log.Println("Invalid audio locale. Please put the locale in the \"ja-JP\", \"en-US\"... format.")
			os.Exit(1)
		}
		contentId = (*correctGuid).GUID
	}
	episode := getEpisode(contentId)

	manifest := parseManifest(episode.ManifestUrl)
	pssh := getPssh(manifest)
	if pssh == nil {
		panic("pssh not found")
	}
	videoSet := findSet(manifest.Period[0].AdaptationSets, "video/mp4")
	audioSet := findSet(manifest.Period[0].AdaptationSets, "audio/mp4")

	// Get Widevine license
	getLicense(*pssh, contentId, episode.Token)

	// Download subtitles
	subtitles := episode.Subtitles[*subtitlesLang]
	if subtitles != nil {
		fmt.Printf("Downloading subtitles for language: %s...", languageNames[*subtitlesLang])
		downloadSubs(subtitles.Url)
		fmt.Println("Downloaded subtitles!")
	}

	// Download video
	baseUrl, representationId := getBaseUrl(manifest, "video/mp4", *videoQuality)
	downloadParts(baseUrl, representationId, videoSet)

	// Download audio
	audioBaseUrl, audioRepresentationId := getBaseUrl(manifest, "audio/mp4", *audioQuality)
	downloadParts(audioBaseUrl, audioRepresentationId, audioSet)

	mergeEverything(subtitlesLang, info)
}
