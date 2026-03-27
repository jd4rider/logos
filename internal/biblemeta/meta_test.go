package biblemeta

import "testing"

func TestStripLangPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "english prefix", input: "engKJV", want: "KJV"},
		{name: "spanish prefix", input: "spaRVR", want: "RVR"},
		{name: "two letter prefix", input: "enWEB", want: "WEB"},
		{name: "preserve nlv", input: "NLV", want: "NLV"},
		{name: "preserve plain kjv", input: "KJV", want: "KJV"},
		{name: "trim whitespace", input: " engKJV ", want: "KJV"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := StripLangPrefix(tc.input); got != tc.want {
				t.Fatalf("StripLangPrefix(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMatchesLanguage(t *testing.T) {
	t.Parallel()

	if !MatchesLanguage("en", "eng") {
		t.Fatal("expected en to match eng")
	}
	if !MatchesLanguage("eng", "") {
		t.Fatal("expected empty filter to match all languages")
	}
	if MatchesLanguage("spa", "eng") {
		t.Fatal("expected spa not to match eng")
	}
}
