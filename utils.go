package main

import "fmt"

var (
	languageNames = map[string]string{
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

	// ffmpegLangs maps to ISO-639-2 codes
	ffmpegLangs = map[string]string{
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
)

func humanSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(size)/float64(1024))
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MiB", float64(size)/float64(1024*1024))
	} else if size < 1024*1024*1024*1024 {
		return fmt.Sprintf("%.1f GiB", float64(size)/float64(1024*1024*1024))
	}

	return fmt.Sprintf("%d B", size)
}
