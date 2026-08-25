// vendor/customer_client.go
package vendor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// CustomerClient handles vendor API operations specific to customers
type CustomerClient struct {
	apiURL     string
	secretKey  string
	tokenCache map[string]CachedToken
	mu         sync.RWMutex
	httpClient *http.Client
}

// NewCustomerClient creates a new customer vendor client
func NewCustomerClient(apiURL, secretKey string) *CustomerClient {
	return &CustomerClient{
		apiURL:     apiURL,
		secretKey:  secretKey,
		tokenCache: make(map[string]CachedToken),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ============================================================
// SESSION TOKEN METHODS
// ============================================================

// GetSessionToken retrieves a valid session token for a customer.
// If the token is not cached or is within 2 minutes of expiry, it mints a new one.
func (cc *CustomerClient) GetSessionToken(ctx context.Context, customerID string) (string, error) {
	cc.mu.RLock()
	cached, ok := cc.tokenCache[customerID]
	cc.mu.RUnlock()

	// If token exists and is valid for at least another 2 minutes, return it
	if ok && time.Now().Add(2*time.Minute).Before(cached.Expiry) {
		return cached.Token, nil
	}

	// Otherwise, mint a new one
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Double-check cache in case another goroutine just updated it
	cached, ok = cc.tokenCache[customerID]
	if ok && time.Now().Add(2*time.Minute).Before(cached.Expiry) {
		return cached.Token, nil
	}

	url := fmt.Sprintf("%s/token", cc.apiURL)
	reqBody, err := json.Marshal(map[string]string{
		"endUserId":   customerID,
		"endUserName": fmt.Sprintf("Customer %s", customerID),
		"userType":    "customer",
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cc.secretKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpClient.Do(req)
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

	cc.tokenCache[customerID] = CachedToken{
		Token:  res.Token,
		Expiry: time.Now().Add(time.Duration(res.ExpiresIn) * time.Second),
	}

	return res.Token, nil
}

// ============================================================
// MESSAGE METHODS
// ============================================================

// ForwardMessage sends the customer's message content to the vendor's /message endpoint.
func (cc *CustomerClient) ForwardMessage(ctx context.Context, customerID, content string) error {
	token, err := cc.GetSessionToken(ctx, customerID)
	if err != nil {
		return fmt.Errorf("failed to get session token for forwarding: %w", err)
	}

	url := fmt.Sprintf("%s/message", cc.apiURL)
	reqBody, err := json.Marshal(map[string]string{
		"content":  content,
		"userType": "customer",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create message forward request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpClient.Do(req)
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

// ForwardMessageWithMedia sends the customer's message with media to the vendor's /message endpoint.
func (cc *CustomerClient) ForwardMessageWithMedia(ctx context.Context, customerID, content, mediaType, mediaURL string) error {
	token, err := cc.GetSessionToken(ctx, customerID)
	if err != nil {
		return fmt.Errorf("failed to get session token for forwarding: %w", err)
	}

	url := fmt.Sprintf("%s/message", cc.apiURL)
	reqBody, err := json.Marshal(map[string]string{
		"content":   content,
		"userType":  "customer",
		"mediaType": mediaType,
		"mediaURL":  mediaURL,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create message forward request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpClient.Do(req)
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

// ============================================================
// AGENT REPLY METHODS
// ============================================================

// ForwardAgentReply sends the admin's reply to the vendor's agent reply endpoint for customers.
func (cc *CustomerClient) ForwardAgentReply(ctx context.Context, customerID, content string) (string, error) {
	url := fmt.Sprintf("%s/agent/reply", cc.apiURL)
	reqBody, err := json.Marshal(map[string]string{
		"endUserId": customerID,
		"content":   content,
		"userType":  "customer",
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal agent reply payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create agent reply request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cc.secretKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpClient.Do(req)
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

// ForwardAgentReplyWithMedia sends the admin's reply with media to the vendor's agent reply endpoint.
func (cc *CustomerClient) ForwardAgentReplyWithMedia(ctx context.Context, customerID, content, mediaType, mediaURL string) (string, error) {
	url := fmt.Sprintf("%s/agent/reply", cc.apiURL)
	reqBody, err := json.Marshal(map[string]string{
		"endUserId": customerID,
		"content":   content,
		"userType":  "customer",
		"mediaType": mediaType,
		"mediaURL":  mediaURL,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal agent reply payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create agent reply request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cc.secretKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpClient.Do(req)
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

// ============================================================
// BOT TOGGLE METHODS
// ============================================================

// ToggleVendorBot enables or disables the vendor's AI bot response for a customer conversation.
func (cc *CustomerClient) ToggleVendorBot(ctx context.Context, customerID string, enabled bool) error {
	url := fmt.Sprintf("%s/agent/bot", cc.apiURL)
	reqBody, err := json.Marshal(map[string]interface{}{
		"endUserId": customerID,
		"enabled":   enabled,
		"userType":  "customer",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal bot toggle payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create bot toggle request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cc.secretKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpClient.Do(req)
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

// ============================================================
// CACHE MANAGEMENT
// ============================================================

// ClearTokenCache clears all cached tokens
func (cc *CustomerClient) ClearTokenCache() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.tokenCache = make(map[string]CachedToken)
}

// ClearTokenForCustomer clears cached token for a specific customer
func (cc *CustomerClient) ClearTokenForCustomer(customerID string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	delete(cc.tokenCache, customerID)
}

// GetTokenCacheSize returns the number of cached tokens
func (cc *CustomerClient) GetTokenCacheSize() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return len(cc.tokenCache)
}

// ============================================================
// CONVERSATION METHODS
// ============================================================

// GetConversationHistory retrieves conversation history for a customer
func (cc *CustomerClient) GetConversationHistory(ctx context.Context, customerID string, limit, offset int) ([]map[string]interface{}, error) {
	token, err := cc.GetSessionToken(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session token: %w", err)
	}

	url := fmt.Sprintf("%s/conversations/history?endUserId=%s&limit=%d&offset=%d", cc.apiURL, customerID, limit, offset)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create history request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation history: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("history request returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode history response: %w", err)
	}

	return result.Messages, nil
}

// ============================================================
// HEALTH CHECK
// ============================================================

// HealthCheck checks if the vendor API is reachable
func (cc *CustomerClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", cc.apiURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}
