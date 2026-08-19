package utils

import (
	"html"
	"strings"
)

// CleanText trims whitespace and decodes HTML entities.
func CleanText(value string) string {
	return strings.TrimSpace(html.UnescapeString(value))
}
