package validator

import (
	"regexp"
	"strings"
)

// ValidateEmail validates an email address
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// ValidateNotEmpty validates that a string is not empty
func ValidateNotEmpty(s string) bool {
	return strings.TrimSpace(s) != ""
}

// ValidateLength validates that a string is within the specified length range
func ValidateLength(s string, min, max int) bool {
	length := len(strings.TrimSpace(s))
	return length >= min && length <= max
}

