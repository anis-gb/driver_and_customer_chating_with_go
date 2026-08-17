package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// SaveUploadedFile extracts a file from the request, saves it locally, and returns the relative file path.
// Returns an empty string and no error if the file field is not present in the request.
func SaveUploadedFile(r *http.Request, fieldName string, destDir string) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if err == http.ErrMissingFile {
			return "", nil // No file uploaded for this field
		}
		return "", err
	}
	defer file.Close()

	// Create destDir if it doesn't exist
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Generate a unique filename using timestamp
	ext := filepath.Ext(header.Filename)
	newFileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	
	// Create the file on disk
	filePath := filepath.Join(destDir, newFileName)
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file on disk: %w", err)
	}
	defer dst.Close()

	// Copy the uploaded file to the destination file
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Return the relative URL path for the database
	return fmt.Sprintf("/uploads/%s", newFileName), nil
}
