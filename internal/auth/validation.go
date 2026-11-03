package auth

import (
	"fmt"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/validation"
	"github.com/google/uuid"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,63}$`)
var employeePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.-]{0,29}$`)
var postalCodePattern = regexp.MustCompile(`^[0-9]{5}$`)
var bangkokZone = time.FixedZone("Asia/Bangkok", 7*3600)

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
	if !validation.Text(s, 1, 200) || strings.TrimSpace(s) == "" {
		return fmt.Errorf("%w: name must contain 1-200 characters", ErrInvalidInput)
	}
	return nil
}
func validateAddress(s string) error {
	valid := validation.Text(s, 1, 500)
	for _, r := range s {
		valid = valid && (!unicode.IsControl(r) || r == '\n' || r == '\r' || r == '\t')
	}
	if !valid {
		return fmt.Errorf("%w: address must contain 1-500 characters", ErrInvalidInput)
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
	if !validation.Text(s, 0, 100) {
		return fmt.Errorf("%w: position must be at most 100 characters", ErrInvalidInput)
	}
	return nil
}
func validateHiredAt(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().In(bangkokZone).Format(time.DateOnly), nil
	}
	parsed, err := time.ParseInLocation(time.DateOnly, s, bangkokZone)
	if err != nil {
		return "", fmt.Errorf("%w: hired_at must be a valid YYYY-MM-DD date", ErrInvalidInput)
	}
	today := time.Now().In(bangkokZone).Format(time.DateOnly)
	if parsed.Format(time.DateOnly) > today {
		return "", fmt.Errorf("%w: hired_at must not be in the future", ErrInvalidInput)
	}
	return parsed.Format(time.DateOnly), nil
}
func validateID(s string) error {
	if _, err := uuid.Parse(s); err != nil || len(s) != 36 {
		return fmt.Errorf("%w: invalid account id", ErrInvalidInput)
	}
	return nil
}
