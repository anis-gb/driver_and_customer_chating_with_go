package vendor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CachedToken struct {
	Token  string
	Expiry time.Time
}

type VendorClient struct {
	apiURL     string
	secretKey  string
	tokenCache map[string]CachedToken
	mu         sync.RWMutex
	httpClient *http.Client
}

func NewVendorClient(apiURL, secretKey string) *VendorClient {
	return &VendorClient{
		apiURL:     apiURL,
		secretKey:  secretKey,
		tokenCache: make(map[string]CachedToken),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetSessionToken retrieves a valid session token for a driver.
// If the token is not cached or is within 2 minutes of expiry, it mints a new one.
func (vc *VendorClient) GetSessionToken(ctx context.Context, driverID string) (string, error) {
	vc.mu.RLock()
	cached, ok := vc.tokenCache[driverID]
	vc.mu.RUnlock()

	// If token exists and is valid for at least another 2 minutes, return it
	if ok && time.Now().Add(2*time.Minute).Before(cached.Expiry) {
		return cached.Token, nil
	}

	// Otherwise, mint a new one
	vc.mu.Lock()
	defer vc.mu.Unlock()

	// Double-check cache in case another goroutine just updated it
	cached, ok = vc.tokenCache[driverID]
	if ok && time.Now().Add(2*time.Minute).Before(cached.Expiry) {
		return cached.Token, nil
	}

	url := fmt.Sprintf("%s/token", vc.apiURL)
	reqBody, err := json.Marshal(map[string]string{
		"endUserId":   driverID,
		"endUserName": fmt.Sprintf("Driver %s", driverID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", vc.secretKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expiresIn"` // in seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	vc.tokenCache[driverID] = CachedToken{
		Token:  res.Token,
		Expiry: time.Now().Add(time.Duration(res.ExpiresIn) * time.Second),
	}

	return res.Token, nil
}

// UploadMedia uploads a local file to the vendor and returns the URL and classified messageType.
func (vc *VendorClient) UploadMedia(ctx context.Context, driverID, localFilePath string) (string, string, error) {
	token, err := vc.GetSessionToken(ctx, driverID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get session token for upload: %w", err)
	}

	file, err := os.Open(localFilePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", filepath.Base(localFilePath))
	if err != nil {
		return "", "", fmt.Errorf("failed to create form file: %w", err)
	}
	
	if _, err = io.Copy(part, file); err != nil {
		return "", "", fmt.Errorf("failed to copy file contents: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/upload", vc.apiURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return "", "", fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to perform upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("upload returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		URL         string `json:"url"`
		MessageType string `json:"messageType"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", "", fmt.Errorf("failed to decode upload response: %w", err)
	}

	return res.URL, res.MessageType, nil
}

// ForwardMessage sends the driver's message content (text or media URL) to the vendor's /message endpoint.
func (vc *VendorClient) ForwardMessage(ctx context.Context, driverID, content, messageType string) error {
	token, err := vc.GetSessionToken(ctx, driverID)
	if err != nil {
		return fmt.Errorf("failed to get session token for forwarding: %w", err)
	}

	url := fmt.Sprintf("%s/message", vc.apiURL)
	
	payload := map[string]string{
		"content": content,
	}
	if messageType != "" && messageType != "TEXT" {
		payload["messageType"] = messageType
	}
	
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create message forward request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform message forward: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("message forward returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// ToggleVendorBot enables or disables the vendor's AI bot response for a conversation.
func (vc *VendorClient) ToggleVendorBot(ctx context.Context, driverID string, enabled bool) error {
	url := fmt.Sprintf("%s/agent/bot", vc.apiURL)
	reqBody, err := json.Marshal(map[string]interface{}{
		"endUserId": driverID,
		"enabled":   enabled,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal bot toggle payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create bot toggle request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", vc.secretKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform bot toggle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bot toggle returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetVendorBotStatus gets the current toggle status of the AI agent for a conversation.
func (vc *VendorClient) GetVendorBotStatus(ctx context.Context, driverID string) (bool, error) {
	url := fmt.Sprintf("%s/agent/bot?endUserId=%s", vc.apiURL, driverID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create bot status request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", vc.secretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to perform bot status query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("bot status query returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		BotEnabled bool `json:"botEnabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return false, fmt.Errorf("failed to decode bot status response: %w", err)
	}

	return res.BotEnabled, nil
}


// ForwardAgentReply sends the admin's reply to the vendor's agent reply endpoint and returns the vendor's message ID.
func (vc *VendorClient) ForwardAgentReply(ctx context.Context, driverID, content string) (string, error) {
	url := fmt.Sprintf("%s/agent/reply", vc.apiURL)
	reqBody, err := json.Marshal(map[string]string{
		"endUserId": driverID,
		"content":   content,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal agent reply payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create agent reply request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", vc.secretKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform agent reply: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("agent reply returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		MessageID string `json:"messageId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode agent reply response: %w", err)
	}

	return res.MessageID, nil
}

