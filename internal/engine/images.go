package engine

// CRImage is one size entry under images.thumbnail / poster_* from CMS JSON.
type CRImage struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Source string `json:"source"`
	Type   string `json:"type"`
}

// CRImages is the Crunchyroll CMS image map.
//
// CMS returns nested arrays: each key is [][]CRImage (groups of size variants),
// e.g. "thumbnail": [ [ {w:120,...}, {w:320,...}, ... ] ].
// Unmarshaling into []CRImage fails with:
// "cannot unmarshal array into Go struct field ... of type engine.CRImage".
type CRImages struct {
	Thumbnail  [][]CRImage `json:"thumbnail"`
	PosterTall [][]CRImage `json:"poster_tall"`
	PosterWide [][]CRImage `json:"poster_wide"`
}

// flattenImageGroups joins nested CMS image groups into a single slice.
func flattenImageGroups(groups [][]CRImage) []CRImage {
	if len(groups) == 0 {
		return nil
	}
	var out []CRImage
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

// pickImageURL chooses a reasonably sized image for UI thumbs.
// Prefers the largest width that is still <= maxWidth; falls back to smallest
// if all are larger (or the only available). Empty if no sources.
//
// Images are loaded by the WebView from Crunchyroll's static CDN — this does
// not open playback streams and does not add CMS API calls.
func pickImageURL(images []CRImage, maxWidth int) string {
	if len(images) == 0 {
		return ""
	}
	if maxWidth <= 0 {
		maxWidth = 320
	}
	bestUnder := -1
	bestUnderW := -1
	smallest := 0
	for i, img := range images {
		if img.Source == "" {
			continue
		}
		if images[smallest].Source == "" || (img.Width > 0 && (images[smallest].Width == 0 || img.Width < images[smallest].Width)) {
			smallest = i
		}
		if img.Width > 0 && img.Width <= maxWidth && img.Width > bestUnderW {
			bestUnder = i
			bestUnderW = img.Width
		}
	}
	if bestUnder >= 0 {
		return images[bestUnder].Source
	}
	if images[smallest].Source != "" {
		return images[smallest].Source
	}
	// Last resort: first non-empty
	for _, img := range images {
		if img.Source != "" {
			return img.Source
		}
	}
	return ""
}

func pickImageURLFromGroups(groups [][]CRImage, maxWidth int) string {
	return pickImageURL(flattenImageGroups(groups), maxWidth)
}

func thumbnailFromImages(images CRImages) string {
	// Episode thumbs: prefer thumbnail, then wide poster crop.
	if u := pickImageURLFromGroups(images.Thumbnail, 320); u != "" {
		return u
	}
	if u := pickImageURLFromGroups(images.PosterWide, 480); u != "" {
		return u
	}
	return pickImageURLFromGroups(images.PosterTall, 240)
}

func posterFromImages(images CRImages) string {
	// Series/movie hero: prefer tall poster for portrait card, then wide, then thumb.
	if u := pickImageURLFromGroups(images.PosterTall, 360); u != "" {
		return u
	}
	if u := pickImageURLFromGroups(images.PosterWide, 640); u != "" {
		return u
	}
	return pickImageURLFromGroups(images.Thumbnail, 480)
}
