package webhook

import "testing"

func TestMatchesFieldWildcardsCrossSlashes(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{pattern: "*code*", value: "visit https://example.com/code", want: true},
		{pattern: "https://*/code", value: "https://example.com/code", want: true},
		{pattern: "https://example.com/*", value: "https://example.com/a/b", want: true},
		{pattern: "https://example.com/?", value: "https://example.com/a/b", want: false},
	}
	for _, test := range tests {
		if got := matchesField([]string{test.pattern}, []string{test.value}); got != test.want {
			t.Errorf("matchesField(%q, %q) = %v, want %v", test.pattern, test.value, got, test.want)
		}
	}
}

func TestTextGlobPreservesSlashCharacterRanges(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{pattern: "[.-0]", value: "/", want: true},
		{pattern: "[^.-0]", value: "/", want: false},
		{pattern: "[\ue000-\ue002]", value: "/", want: false},
		{pattern: "[\ue000-\ue002]", value: "\ue001", want: true},
	}
	for _, test := range tests {
		matched, err := pathMatch(test.pattern, test.value)
		if err != nil || matched != test.want {
			t.Errorf("pathMatch(%q, %q) = %v, %v; want %v", test.pattern, test.value, matched, err, test.want)
		}
	}
}
