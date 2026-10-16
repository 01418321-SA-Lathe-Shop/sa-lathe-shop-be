package auth

import (
	"fmt"
	"github.com/google/uuid"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,63}$`)
var employeePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.-]{0,29}$`)

func normalizeEmail(s string) (string, error) {
	s = strings.TrimSpace(s)
	a, err := mail.ParseAddress(s)
	if err != nil || len(s) > 254 || a.Address != s || strings.ContainsAny(s, "\r\n\x00") {
		return "", fmt.Errorf("%w: email must be a valid single address of at most 254 bytes", ErrInvalidInput)
	}
	// Preserve local-part case; email is contact information, not a login key.
	return s, nil
}

func normalizePhone(s string) (string, error) {
	s = strings.TrimSpace(s)
	var out strings.Builder
	digits := 0
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			out.WriteRune(r)
			digits++
		case r == '+' && i == 0:
			out.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
		default:
			return "", fmt.Errorf("%w: phone accepts digits, a leading + and formatting separators", ErrInvalidInput)
		}
	}
	if digits < 7 || digits > 15 || len(s) > 64 {
		return "", fmt.Errorf("%w: phone must contain 7-15 digits", ErrInvalidInput)
	}
	return out.String(), nil
}

func normalizeUsername(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if !usernamePattern.MatchString(s) {
		return "", fmt.Errorf("%w: username must be 3-64 ASCII letters, digits, _, . or -", ErrInvalidInput)
	}
	return s, nil
}
func validatePassword(s string) error {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 12 || len(s) > 128 {
		return fmt.Errorf("%w: password must have at least 12 characters and at most 128 UTF-8 bytes", ErrInvalidInput)
	}
	return nil
}
func validateName(s string) error {
	if !utf8.ValidString(s) || strings.TrimSpace(s) == "" || utf8.RuneCountInString(s) > 200 {
		return fmt.Errorf("%w: name must contain 1-200 characters", ErrInvalidInput)
	}
	return nil
}
func normalizeEmployee(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !employeePattern.MatchString(s) {
		return "", fmt.Errorf("%w: employee_no must be 1-30 ASCII letters, digits, _, . or -", ErrInvalidInput)
	}
	return s, nil
}
func validatePosition(s string) error {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > 100 {
		return fmt.Errorf("%w: position must be at most 100 characters", ErrInvalidInput)
	}
	return nil
}
func validateID(s string) error {
	if _, err := uuid.Parse(s); err != nil || len(s) != 36 {
		return fmt.Errorf("%w: invalid account id", ErrInvalidInput)
	}
	return nil
}
