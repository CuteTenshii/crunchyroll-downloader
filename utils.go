package main

import "regexp"

var languageNames = map[string]string{
	"ja-JP":  "日本語",
	"en-US":  "English",
	"en-IN":  "English (India)",
	"id-ID":  "Bahasa Indonesia",
	"ms-MY":  "Bahasa Melayu",
	"ca-ES":  "Català",
	"de-DE":  "Deutsch",
	"es-419": "Español (América Latina)",
	"es-ES":  "Español (España)",
	"fr-FR":  "Français",
	"it-IT":  "Italiano",
	"pl-PL":  "Polski",
	"pt-BR":  "Português (Brasil)",
	"pt-PT":  "Português (Portugal)",
	"vi-VN":  "Tiếng Việt",
	"tr-TR":  "Türkçe",
	"ru-RU":  "Русский",
	"ar-SA":  "العربية",
	"hi-IN":  "हिंदी",
	"ta-IN":  "தமிழ்",
	"te-IN":  "తెలుగు",
	"zh-CN":  "中文 (普通话)",
	"zh-HK":  "中文 (粵語)",
	"zh-TW":  "中文 (國語)",
	"ko-KR":  "한국어",
	"th-TH":  "ไทย",
}

// languageCodes maps a Crunchyroll locale to its ISO 639-2 code, used for the
// ffmpeg "language" stream tag so media servers can select tracks correctly.
var languageCodes = map[string]string{
	"ja-JP":  "jpn",
	"en-US":  "eng",
	"en-IN":  "eng",
	"id-ID":  "ind",
	"ms-MY":  "msa",
	"ca-ES":  "cat",
	"de-DE":  "deu",
	"es-419": "spa",
	"es-ES":  "spa",
	"fr-FR":  "fra",
	"it-IT":  "ita",
	"pl-PL":  "pol",
	"pt-BR":  "por",
	"pt-PT":  "por",
	"vi-VN":  "vie",
	"tr-TR":  "tur",
	"ru-RU":  "rus",
	"ar-SA":  "ara",
	"hi-IN":  "hin",
	"ta-IN":  "tam",
	"te-IN":  "tel",
	"zh-CN":  "zho",
	"zh-HK":  "zho",
	"zh-TW":  "zho",
	"ko-KR":  "kor",
	"th-TH":  "tha",
}

// sanitizeFilename removes illegal OS characters and formats the string safely
func sanitizeFilename(name string) string {
	if name == "" {
		return "Unknown"
	}
	// Replace illegal characters and quotes with an underscore
	re := regexp.MustCompile(`[\\/:*?"<>|'’\x60“”]`)
	res := re.ReplaceAllString(name, "_")
	
	// Collapse multiple consecutive underscores into a single one
	reCollapse := regexp.MustCompile(`_+`)
	res = reCollapse.ReplaceAllString(res, "_")
	
	return strings.TrimRight(res, " .")
}