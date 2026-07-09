package api

type Episode struct {
	ManifestURL string                `json:"url"`
	Subtitles   map[string]*Subtitle  `json:"subtitles"`
	Token       string                `json:"token"`
	Error       any                   `json:"error"`
}

type Subtitle struct {
	Language string `json:"language"`
	URL      string `json:"url"`
}

type EpisodeMetadataResponse struct {
	Data []EpisodeInfo `json:"data"`
}

type EpisodeInfo struct {
	EpisodeMetadata EpisodeMetadata `json:"episode_metadata"`
	Title           string          `json:"title"`
}

type EpisodeMetadata struct {
	AudioLocale        string       `json:"audio_locale"`
	EpisodeNumber      int          `json:"episode_number"`
	SeasonNumber       int          `json:"season_number"`
	SeriesTitle        string       `json:"series_title"`
	AvailabilityStarts string       `json:"availability_starts"`
	Versions           []*DubVersion `json:"versions"`
}

type DubVersion struct {
	AudioLocale string `json:"audio_locale"`
	GUID        string `json:"guid"`
}

type SeasonEpisodes struct {
	Data []SeasonEpisode `json:"data"`
}

type SeasonEpisode struct {
	ID                 string        `json:"id"`
	Versions           []*DubVersion `json:"versions"`
	SeasonNumber       int           `json:"season_number"`
	EpisodeNumber      int           `json:"episode_number"`
	SeriesTitle        string        `json:"series_title"`
	AudioLocale        string        `json:"audio_locale"`
	Title              string        `json:"title"`
	AvailabilityStarts string        `json:"availability_starts"`
}

type Seasons struct {
	Data []Season `json:"data"`
}

type Season struct {
	ID           string `json:"id"`
	SeasonNumber int    `json:"season_number"`
}

type CrunchyrollTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type CrunchyrollWidevineLicenseResponse struct {
	License string `json:"license"`
}
