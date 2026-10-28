// Package cloudapi is a minimal client for sending messages through the
// WhatsApp Business Platform Cloud API, which is served by the Meta Graph API.
package cloudapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPIVersion is the Graph API version used when none is given.
const DefaultAPIVersion = "v26.0"

const (
	graphURL         = "https://graph.facebook.com"
	maxResponseBytes = 1 << 20
)

// Client calls the Cloud API. Its methods are safe for concurrent use.
type Client struct {
	// BaseURL is the Graph API URL including the version, for example
	// "https://graph.facebook.com/v26.0". Override it to use a test server.
	BaseURL string
	// HTTPClient defaults to a client with a 15s timeout.
	HTTPClient *http.Client

	accessToken string
}

// NewClient returns a Client that authenticates with accessToken: a System
// User access token with the whatsapp_business_messaging permission.
// apiVersion is a Graph API version such as "v26.0"; empty selects
// DefaultAPIVersion.
func NewClient(accessToken, apiVersion string) *Client {
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}
	return &Client{
		BaseURL:     graphURL + "/" + apiVersion,
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
		accessToken: accessToken,
	}
}

// Send sends msg from the business phone number phoneNumberID to the
// WhatsApp user to (a phone number or wa_id).
func (c *Client) Send(ctx context.Context, phoneNumberID, to string, msg Message) (*SendResponse, error) {
	to = normalizeRecipient(to) // <--- Agrega esta línea aquí

	req := struct {
		MessagingProduct string `json:"messaging_product"`
		RecipientType    string `json:"recipient_type"`
		To               string `json:"to"`
		Message
	}{"whatsapp", "individual", to, msg}

	var resp SendResponse
	if err := c.post(ctx, phoneNumberID, req, &resp); err != nil {
		return nil, fmt.Errorf("send %s message: %w", msg.Type, err)
	}
	return &resp, nil
}

// MarkRead marks messageID as read (blue ticks). If typing is true it also
// shows a typing indicator, which disappears when you reply or after 25s.
func (c *Client) MarkRead(ctx context.Context, phoneNumberID, messageID string, typing bool) error {
	type typingIndicator struct {
		Type string `json:"type"`
	}
	req := struct {
		MessagingProduct string           `json:"messaging_product"`
		Status           string           `json:"status"`
		MessageID        string           `json:"message_id"`
		TypingIndicator  *typingIndicator `json:"typing_indicator,omitempty"`
	}{MessagingProduct: "whatsapp", Status: "read", MessageID: messageID}
	if typing {
		req.TypingIndicator = &typingIndicator{Type: "text"}
	}

	if err := c.post(ctx, phoneNumberID, req, nil); err != nil {
		return fmt.Errorf("mark %s read: %w", messageID, err)
	}
	return nil
}

// post sends body as JSON to /{phoneNumberID}/messages and decodes the
// response into out, if non-nil.
func (c *Client) post(ctx context.Context, phoneNumberID string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	endpoint := c.BaseURL + "/" + url.PathEscape(phoneNumberID) + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseError(resp.StatusCode, data)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func normalizeRecipient(to string) string {
	if strings.HasPrefix(to, "521") && len(to) == 13 {
		return "52" + to[3:]
	}
	return to
}
