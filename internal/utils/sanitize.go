package utils

import (
	"html"
	"strings"
)

// CleanText sanitizes string inputs by stripping null bytes, trimming leading/trailing whitespace,
// and escaping HTML entities (<, >, &, ', ") to prevent Cross-Site Scripting (XSS) attacks.
func CleanText(input string) string {
	if input == "" {
		return ""
	}
	// Remove null bytes
	cleaned := strings.ReplaceAll(input, "\x00", "")
	// Escape HTML special characters
	cleaned = html.EscapeString(cleaned)
	// Trim surrounding whitespace
	return strings.TrimSpace(cleaned)
}
