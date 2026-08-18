package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Allowed MIME types map
var allowedMIMETypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/webp":      true,
	"application/pdf": true,
	"audio/mpeg":      true,
	"audio/ogg":       true,
	"audio/wav":       true,
	"audio/x-wav":     true,
	"audio/webm":      true,
	"video/webm":      true,
	"audio/mp4":       true,
	"audio/aac":       true,
	"audio/x-m4a":     true,
}

// Allowed extensions map (lowercase)
var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".pdf":  true,
	".mp3":  true,
	".ogg":  true,
	".wav":  true,
	".webm": true,
	".m4a":  true,
	".aac":  true,
	".mp4":  true,
}

// Blacklisted extensions that must NEVER be saved or executed
var dangerousExtensions = map[string]bool{
	".php":   true,
	".php3":  true,
	".php4":  true,
	".php5":  true,
	".phtml": true,
	".exe":   true,
	".sh":    true,
	".bat":   true,
	".cmd":   true,
	".html":  true,
	".htm":   true,
	".svg":   true, // Block SVG to prevent XML/JS XSS injection
	".js":    true,
	".jsp":   true,
	".asp":   true,
	".aspx":  true,
	".cgi":   true,
	".pl":    true,
	".py":    true,
	".dll":   true,
	".so":    true,
}

// generateSecureRandomFilename generates a collision-resistant filename using timestamp and cryptographically secure random bytes.
func generateSecureRandomFilename(ext string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	randomHex := hex.EncodeToString(bytes)
	return fmt.Sprintf("%d_%s%s", time.Now().Unix(), randomHex, ext), nil
}

// SaveUploadedFile extracts a file from the request, inspects its magic bytes and MIME type,
// verifies security rules, saves it locally with a random filename, and returns the relative file path.
func SaveUploadedFile(r *http.Request, fieldName string, destDir string) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil // No file uploaded for this field
		}
		return "", err
	}
	defer file.Close()

	// 1. Verify extension against blacklist and whitelist
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" || dangerousExtensions[ext] {
		return "", fmt.Errorf("dangerous or invalid file extension: %s", ext)
	}
	if !allowedExtensions[ext] {
		return "", fmt.Errorf("file extension '%s' is not allowed", ext)
	}

	// 2. Read first 512 bytes to sniff actual Magic Bytes (Content-Type)
	buffer := make([]byte, 512)
	n, readErr := file.Read(buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", fmt.Errorf("failed to inspect file content header: %w", readErr)
	}

	detectedMIME := http.DetectContentType(buffer[:n])

	// If detected MIME is octet-stream, check client header fallback for binary audio formats, otherwise validate detected MIME
	if detectedMIME == "application/octet-stream" {
		clientMIME := strings.ToLower(header.Header.Get("Content-Type"))
		if !allowedMIMETypes[clientMIME] && clientMIME != "audio/m4a" && clientMIME != "audio/x-m4a" {
			return "", fmt.Errorf("unrecognized file type (MIME: %s)", clientMIME)
		}
	} else {
		// Strip parameters like charset from MIME type if present
		cleanMIME := strings.Split(detectedMIME, ";")[0]
		if !allowedMIMETypes[cleanMIME] {
			return "", fmt.Errorf("unsupported file content type: %s", cleanMIME)
		}
	}

	// Reset file reader back to start position
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// 3. Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// 4. Generate random collision-free filename
	newFileName, err := generateSecureRandomFilename(ext)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure filename: %w", err)
	}

	// 5. Create destination file on disk
	filePath := filepath.Join(destDir, newFileName)
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file on disk: %w", err)
	}
	defer dst.Close()

	// 6. Stream content securely to disk
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("failed to write file content to disk: %w", err)
	}

	return fmt.Sprintf("/uploads/%s", newFileName), nil
}
