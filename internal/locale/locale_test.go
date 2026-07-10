package locale

import "testing"

func TestLanguageNames(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"ja-JP", "日本語"},
		{"en-US", "English"},
		{"en-IN", "English (India)"},
		{"id-ID", "Bahasa Indonesia"},
		{"ms-MY", "Bahasa Melayu"},
		{"ca-ES", "Català"},
		{"de-DE", "Deutsch"},
		{"es-419", "Español (América Latina)"},
		{"es-ES", "Español (España)"},
		{"fr-FR", "Français"},
		{"it-IT", "Italiano"},
		{"pl-PL", "Polski"},
		{"pt-BR", "Português (Brasil)"},
		{"pt-PT", "Português (Portugal)"},
		{"vi-VN", "Tiếng Việt"},
		{"tr-TR", "Türkçe"},
		{"ru-RU", "Русский"},
		{"ar-SA", "العربية"},
		{"hi-IN", "हिंदी"},
		{"ta-IN", "தமிழ்"},
		{"te-IN", "తెలుగు"},
		{"zh-CN", "中文 (普通话)"},
		{"zh-HK", "中文 (粵語)"},
		{"zh-TW", "中文 (國語)"},
		{"ko-KR", "한국어"},
		{"th-TH", "ไทย"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := LanguageNames[tt.code]
			if got != tt.want {
				t.Fatalf("LanguageNames[%q] = %q, want %q", tt.code, got, tt.want)
			}
		})
	}

	t.Run("missing locale returns empty string", func(t *testing.T) {
		got := LanguageNames["xx-XX"]
		if got != "" {
			t.Fatalf("LanguageNames[xx-XX] = %q, want empty string", got)
		}
	})
}

func TestLanguageCodes(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"ja-JP", "jpn"},
		{"en-US", "eng"},
		{"en-IN", "eng"},
		{"id-ID", "ind"},
		{"ms-MY", "msa"},
		{"ca-ES", "cat"},
		{"de-DE", "deu"},
		{"es-419", "spa"},
		{"es-ES", "spa"},
		{"fr-FR", "fra"},
		{"it-IT", "ita"},
		{"pl-PL", "pol"},
		{"pt-BR", "por"},
		{"pt-PT", "por"},
		{"vi-VN", "vie"},
		{"tr-TR", "tur"},
		{"ru-RU", "rus"},
		{"ar-SA", "ara"},
		{"hi-IN", "hin"},
		{"ta-IN", "tam"},
		{"te-IN", "tel"},
		{"zh-CN", "zho"},
		{"zh-HK", "zho"},
		{"zh-TW", "zho"},
		{"ko-KR", "kor"},
		{"th-TH", "tha"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := LanguageCodes[tt.code]
			if got != tt.want {
				t.Fatalf("LanguageCodes[%q] = %q, want %q", tt.code, got, tt.want)
			}
		})
	}

	t.Run("missing locale returns empty string", func(t *testing.T) {
		got := LanguageCodes["xx-XX"]
		if got != "" {
			t.Fatalf("LanguageCodes[xx-XX] = %q, want empty string", got)
		}
	})
}
