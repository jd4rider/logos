package biblemeta

import "strings"

type LanguageOption struct {
	Code string
	Name string
}

var displayLanguageNames = map[string]string{
	"eng": "English", "en": "English",
	"spa": "Spanish", "es": "Spanish", "esp": "Spanish",
	"fra": "French", "fr": "French",
	"deu": "German", "ger": "German", "de": "German",
	"ita": "Italian", "it": "Italian",
	"por": "Portuguese", "pt": "Portuguese",
	"nld": "Dutch", "nl": "Dutch",
	"pol": "Polish", "pl": "Polish",
	"rus": "Russian", "ru": "Russian",
	"zho": "Chinese", "zh": "Chinese",
	"hin": "Hindi", "hi": "Hindi",
	"ara": "Arabic", "ar": "Arabic",
	"kor": "Korean", "ko": "Korean",
	"jpn": "Japanese", "ja": "Japanese",
	"vie": "Vietnamese", "vi": "Vietnamese",
	"ind": "Indonesian", "id": "Indonesian",
	"tur": "Turkish", "tr": "Turkish",
	"swa": "Swahili", "sw": "Swahili",
	"urd": "Urdu", "ur": "Urdu",
	"ben": "Bengali", "bn": "Bengali",
	"tam": "Tamil", "ta": "Tamil",
	"afr": "Afrikaans",
	"lat": "Latin",
	"grc": "Greek (Ancient)",
	"heb": "Hebrew",
}

func CommonLanguageOptions() []LanguageOption {
	return []LanguageOption{
		{Code: "", Name: "All languages"},
		{Code: "eng", Name: "English"},
		{Code: "spa", Name: "Spanish"},
		{Code: "fra", Name: "French"},
		{Code: "deu", Name: "German"},
		{Code: "por", Name: "Portuguese"},
		{Code: "ita", Name: "Italian"},
		{Code: "nld", Name: "Dutch"},
		{Code: "pol", Name: "Polish"},
		{Code: "zho", Name: "Chinese"},
		{Code: "hin", Name: "Hindi"},
		{Code: "ara", Name: "Arabic"},
		{Code: "kor", Name: "Korean"},
		{Code: "jpn", Name: "Japanese"},
		{Code: "vie", Name: "Vietnamese"},
		{Code: "ind", Name: "Indonesian"},
		{Code: "tur", Name: "Turkish"},
		{Code: "heb", Name: "Hebrew"},
		{Code: "grc", Name: "Greek (Ancient)"},
		{Code: "lat", Name: "Latin"},
	}
}

func CanonicalLanguageCode(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "":
		return ""
	case "en", "eng":
		return "eng"
	case "es", "spa", "esp":
		return "spa"
	case "fr", "fra":
		return "fra"
	case "de", "deu", "ger":
		return "deu"
	case "pt", "por":
		return "por"
	case "it", "ita":
		return "ita"
	case "nl", "nld":
		return "nld"
	case "pl", "pol":
		return "pol"
	case "ru", "rus":
		return "rus"
	case "zh", "zho":
		return "zho"
	case "hi", "hin":
		return "hin"
	case "ar", "ara":
		return "ara"
	case "ko", "kor":
		return "kor"
	case "ja", "jpn":
		return "jpn"
	case "vi", "vie":
		return "vie"
	case "id", "ind":
		return "ind"
	case "tr", "tur":
		return "tur"
	case "sw", "swa":
		return "swa"
	case "ur", "urd":
		return "urd"
	case "bn", "ben":
		return "ben"
	case "ta", "tam":
		return "tam"
	default:
		return strings.ToLower(strings.TrimSpace(code))
	}
}

func MatchesLanguage(code, filter string) bool {
	if CanonicalLanguageCode(filter) == "" {
		return true
	}
	return CanonicalLanguageCode(code) == CanonicalLanguageCode(filter)
}

func StripLangPrefix(abbr string) string {
	trimmed := strings.TrimSpace(abbr)
	if trimmed == "" {
		return ""
	}

	known3 := []string{
		"eng", "spa", "esp", "fra", "deu", "ger", "por", "zho", "hin",
		"ara", "rus", "kor", "jpn", "vie", "ind", "nld", "ita", "pol",
		"tur", "heb", "grc", "lat", "afr", "swa", "urd", "ben", "tam",
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range known3 {
		if strings.HasPrefix(lower, prefix) && len(trimmed) > len(prefix) {
			result := trimmed[len(prefix):]
			if len(result) >= 2 {
				return result
			}
		}
	}

	known2 := []string{"en", "es", "fr", "de", "pt", "it", "nl", "pl"}
	for _, prefix := range known2 {
		if strings.HasPrefix(lower, prefix) && len(trimmed) > len(prefix) {
			result := trimmed[len(prefix):]
			if len(result) >= 2 && result[0] >= 'A' && result[0] <= 'Z' {
				return result
			}
		}
	}

	return trimmed
}

func DisplayBibleAbbreviation(abbr string) string {
	trimmed := StripLangPrefix(abbr)
	if trimmed == "" {
		return strings.TrimSpace(abbr)
	}
	return trimmed
}

func DisplayLanguageName(code string) string {
	canonical := CanonicalLanguageCode(code)
	if name, ok := displayLanguageNames[canonical]; ok {
		return name
	}
	return strings.TrimSpace(code)
}
