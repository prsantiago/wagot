package cloudapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient("test-token", "")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()
	return c
}

func TestNewClientAPIVersion(t *testing.T) {
	if got := NewClient("t", "").BaseURL; got != "https://graph.facebook.com/"+DefaultAPIVersion {
		t.Errorf("default BaseURL = %q", got)
	}
	if got := NewClient("t", "v25.0").BaseURL; got != "https://graph.facebook.com/v25.0" {
		t.Errorf("BaseURL = %q", got)
	}
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode body %s: %v", data, err)
	}
	return m
}

func TestSendText(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/106540352242922/messages" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("Authorization = %q", auth)
		}
		got = decodeBody(t, r)
		_, _ = io.WriteString(w, `{"messaging_product":"whatsapp","contacts":[{"input":"16505551234","wa_id":"16505551234"}],"messages":[{"id":"wamid.out"}]}`)
	})

	resp, err := c.Send(context.Background(), "106540352242922", "16505551234", TextMessage("hello"))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.MessageID() != "wamid.out" {
		t.Errorf("MessageID = %q, want wamid.out", resp.MessageID())
	}
	if got["messaging_product"] != "whatsapp" || got["to"] != "16505551234" || got["type"] != "text" {
		t.Errorf("unexpected body: %v", got)
	}
	if text, _ := got["text"].(map[string]any); text["body"] != "hello" {
		t.Errorf("text = %v", got["text"])
	}
}

func TestMarkRead(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		_, _ = io.WriteString(w, `{"success":true}`)
	})

	if err := c.MarkRead(context.Background(), "1", "wamid.in", true); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if got["status"] != "read" || got["message_id"] != "wamid.in" {
		t.Errorf("unexpected body: %v", got)
	}
	if ti, _ := got["typing_indicator"].(map[string]any); ti["type"] != "text" {
		t.Errorf("typing_indicator = %v", got["typing_indicator"])
	}
}

func TestSendReturnsAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"Re-engagement message","type":"OAuthException","code":131047,"error_data":{"details":"More than 24 hours have passed"},"fbtrace_id":"abc"}}`)
	})

	_, err := c.Send(context.Background(), "1", "2", TextMessage("hi"))

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Code != 131047 || apiErr.HTTPStatus != http.StatusBadRequest || apiErr.ErrorData.Details == "" {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
}

func TestSendNonJSONError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	})

	_, err := c.Send(context.Background(), "1", "2", TextMessage("hi"))

	var apiErr *APIError
	if err == nil || errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want non-APIError", err)
	}
}
