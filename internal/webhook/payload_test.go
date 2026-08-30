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

func TestPathMatchSentinelDoesNotCollide(t *testing.T) {
	value := "https://example.com/\ue000/code"
	matched, err := pathMatch("*/code", value)
	if err != nil || !matched {
		t.Fatalf("pathMatch() = %v, %v", matched, err)
	}
}
