package validation

import "testing"

func TestText(t *testing.T) {
	for _, test := range []struct {
		name, value string
		min, max    int
		valid       bool
	}{
		{"empty optional", "", 0, 2, true}, {"empty required", "", 1, 2, false},
		{"unicode characters not bytes", "กข", 1, 2, true}, {"over limit", "กขค", 1, 2, false},
		{"NUL", "a\x00b", 0, 9, false}, {"invalid UTF8", string([]byte{0xff}), 0, 9, false},
		{"newlines remain caller policy", "a\nb", 0, 9, true}, {"invalid bounds", "a", 2, 1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if Text(test.value, test.min, test.max) != test.valid {
				t.Fatal("text validation result changed")
			}
		})
	}
}
