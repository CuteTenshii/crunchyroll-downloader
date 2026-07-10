package testutil

import (
	"crunchyroll-downloader/internal/api"
	"github.com/unki2aut/go-mpd"
)

func EpisodeInfo() *api.EpisodeInfo {
	return &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}
}

func EpisodeInfoWithVersions(locales ...string) *api.EpisodeInfo {
	info := EpisodeInfo()
	for i, locale := range locales {
		info.EpisodeMetadata.Versions = append(info.EpisodeMetadata.Versions, &api.DubVersion{
			AudioLocale: locale,
			GUID:        locale + "-guid",
		})
		if info.EpisodeMetadata.AudioLocale == "" && i == 0 {
			info.EpisodeMetadata.AudioLocale = locale
		}
	}
	return info
}

func SeasonEpisode(episodeNumber int, seriesTitle string, seasonNumber int) api.SeasonEpisode {
	return api.SeasonEpisode{
		ID:            "episode-N",
		SeriesTitle:   seriesTitle,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: episodeNumber,
		AudioLocale:   "ja-JP",
		Title:         "Test Episode N",
	}
}

func DummyMPD() *mpd.MPD {
	height := uint64(1080)
	videoBandwidth := uint64(5000000)
	audioBandwidth := uint64(192000)
	videoRepID := "video/1080p"
	audioRepID := "audio/192k"
	initPattern := "init-$RepresentationID$.mp4"
	mediaPattern := "seg-$Number%05d$-$RepresentationID$.m4s"
	d := uint64(1)

	return &mpd.MPD{
		Period: []*mpd.Period{
			{
				AdaptationSets: []*mpd.AdaptationSet{
					{
						Representations: []mpd.Representation{
							{
								ID:     &videoRepID,
								Height: &height,
								Bandwidth: &videoBandwidth,
								BaseURL: []*mpd.BaseURL{
									{Value: "https://example.com/video/1080p/"},
								},
							},
						},
						SegmentTemplate: &mpd.SegmentTemplate{
							Initialization: &initPattern,
							Media:          &mediaPattern,
							SegmentTimeline: &mpd.SegmentTimeline{
								S: []*mpd.SegmentTimelineS{
									{D: d, R: nil},
								},
							},
						},
					},
					{
						Representations: []mpd.Representation{
							{
								ID:     &audioRepID,
								Bandwidth: &audioBandwidth,
								BaseURL: []*mpd.BaseURL{
									{Value: "https://example.com/audio/192k/"},
								},
							},
						},
						SegmentTemplate: &mpd.SegmentTemplate{
							Initialization: &initPattern,
							Media:          &mediaPattern,
							SegmentTimeline: &mpd.SegmentTimeline{
								S: []*mpd.SegmentTimelineS{
									{D: d, R: nil},
								},
							},
						},
					},
				},
			},
		},
	}
}
