package razorpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"erp-billing-service/internal/ports/external"
)

type client struct {
	keyID      string
	keySecret  string
	httpClient *http.Client
}

func NewRazorpayClient(keyID, keySecret string) external.RazorpayClient {
	return &client{
		keyID:     keyID,
		keySecret: keySecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Helper to convert float amount to paise (integer subunits)
func toPaise(amount float64) int64 {
	return int64(math.Round(amount * 100.0))
}

func (c *client) doRequest(ctx context.Context, method, url string, body interface{}, response interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.SetBasicAuth(c.keyID, c.keySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("razorpay api error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	if response != nil {
		if err := json.Unmarshal(respBytes, response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w (body: %s)", err, string(respBytes))
		}
	}

	return nil
}

func (c *client) CreatePlan(ctx context.Context, name string, amount float64, currency string) (string, error) {
	url := "https://api.razorpay.com/v1/plans"

	payload := map[string]interface{}{
		"period":   "monthly",
		"interval": 1,
		"item": map[string]interface{}{
			"name":     name,
			"amount":   toPaise(amount),
			"currency": currency,
		},
	}

	var resp struct {
		ID string `json:"id"`
	}

	if err := c.doRequest(ctx, "POST", url, payload, &resp); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (c *client) CreateSubscription(ctx context.Context, planID string, customerEmail string, totalCycles int) (string, string, error) {
	url := "https://api.razorpay.com/v1/subscriptions"

	payload := map[string]interface{}{
		"plan_id":         planID,
		"total_count":     totalCycles,
		"quantity":        1,
		"customer_notify": 1,
		"notify_info": map[string]interface{}{
			"notify_email": customerEmail,
		},
	}

	var resp struct {
		ID          string `json:"id"`
		ShortURL    string `json:"short_url"`
		PaymentLink string `json:"payment_link"`
	}

	if err := c.doRequest(ctx, "POST", url, payload, &resp); err != nil {
		return "", "", err
	}

	link := resp.PaymentLink
	if link == "" {
		link = resp.ShortURL
	}

	return resp.ID, link, nil
}

func (c *client) UpdateSubscriptionPlan(ctx context.Context, subID string, newPlanID string, scheduleChangeAt string) error {
	url := fmt.Sprintf("https://api.razorpay.com/v1/subscriptions/%s", subID)

	payload := map[string]interface{}{
		"plan_id":            newPlanID,
		"schedule_change_at": scheduleChangeAt,
	}

	return c.doRequest(ctx, "PATCH", url, payload, nil)
}

func (c *client) CreateOrder(ctx context.Context, amount float64, currency string, receipt string) (string, error) {
	url := "https://api.razorpay.com/v1/orders"

	payload := map[string]interface{}{
		"amount":   toPaise(amount),
		"currency": currency,
		"receipt":  receipt,
	}

	var resp struct {
		ID string `json:"id"`
	}

	if err := c.doRequest(ctx, "POST", url, payload, &resp); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (c *client) VerifyWebhookSignature(body []byte, signature string, secret string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}
