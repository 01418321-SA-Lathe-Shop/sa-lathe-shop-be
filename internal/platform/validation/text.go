// Package validation contains transport-independent input primitives, not business policy.
package validation

import (
	"strings"
	"unicode/utf8"
)

// Text validates PostgreSQL-compatible Unicode text. Callers choose whether to
// trim and whether newlines are allowed; this function never normalizes content.
func Text(value string, minimum, maximum int) bool {
	if minimum < 0 || maximum < minimum || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return false
	}
	length := utf8.RuneCountInString(value)
	return length >= minimum && length <= maximum
}
