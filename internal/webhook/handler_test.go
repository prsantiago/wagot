package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	testVerifyToken = "verify-me"
	testAppSecret   = "app-secret"
)

const textMessagePayload = `{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "102290129340398",
    "changes": [{
      "field": "messages",
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {"display_phone_number": "15550783881", "phone_number_id": "106540352242922"},
        "contacts": [{"profile": {"name": "Sheena Nelson"}, "wa_id": "16505551234"}],
        "messages": [{
          "from": "16505551234",
          "id": "wamid.HBgLMTY1MDM4Nzk0MzkVAgASGBQzQTRBNjU5OUFFRTAzODEwMTQ0RgA=",
          "timestamp": "1749416383",
          "type": "text",
          "text": {"body": "Does it come in another color?"}
        }]
      }
    }]
  }]
}`

func sign(body string) string {
	mac := hmac.New(sha256.New, []byte(testAppSecret))
	mac.Write([]byte(body))
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

func newTestHandler(t *testing.T, p Processor) *Handler {
	t.Helper()
	if p == nil {
		p = ProcessorFunc(func(context.Context, *Notification) error { return nil })
	}
	h, err := NewHandler(testVerifyToken, testAppSecret, p, nil)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

func TestNewHandlerValidatesArgs(t *testing.T) {
	p := ProcessorFunc(func(context.Context, *Notification) error { return nil })
	tests := map[string]struct {
		token, secret string
		p             Processor
	}{
		"missing token":     {"", testAppSecret, p},
		"missing secret":    {testVerifyToken, "", p},
		"missing processor": {testVerifyToken, testAppSecret, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewHandler(tc.token, tc.secret, tc.p, nil); err == nil {
				t.Fatal("NewHandler succeeded, want error")
			}
		})
	}
}

func TestVerify(t *testing.T) {
	tests := map[string]struct {
		mode, token string
		wantStatus  int
		wantBody    string
	}{
		"valid":       {"subscribe", testVerifyToken, http.StatusOK, "1158201444"},
		"wrong token": {"subscribe", "nope", http.StatusForbidden, ""},
		"wrong mode":  {"unsubscribe", testVerifyToken, http.StatusForbidden, ""},
		"no params":   {"", "", http.StatusForbidden, ""},
	}
	h := newTestHandler(t, nil)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			q := url.Values{
				"hub.mode":         {tc.mode},
				"hub.verify_token": {tc.token},
				"hub.challenge":    {"1158201444"},
			}
			req := httptest.NewRequest(http.MethodGet, "/webhook?"+q.Encode(), nil)
			rec := httptest.NewRecorder()

			h.Verify(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestReceive(t *testing.T) {
	tests := map[string]struct {
		body          string
		signature     string
		processErr    error
		wantStatus    int
		wantProcessed bool
	}{
		"valid":              {textMessagePayload, sign(textMessagePayload), nil, http.StatusOK, true},
		"missing signature":  {textMessagePayload, "", nil, http.StatusUnauthorized, false},
		"wrong signature":    {textMessagePayload, sign("other body"), nil, http.StatusUnauthorized, false},
		"non-hex signature":  {textMessagePayload, "sha256=zz", nil, http.StatusUnauthorized, false},
		"malformed json":     {"{", sign("{"), nil, http.StatusBadRequest, false},
		"unsupported object": {`{"object":"page"}`, sign(`{"object":"page"}`), nil, http.StatusOK, false},
		"processor error":    {textMessagePayload, sign(textMessagePayload), errors.New("boom"), http.StatusInternalServerError, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var processed bool
			h := newTestHandler(t, ProcessorFunc(func(context.Context, *Notification) error {
				processed = true
				return tc.processErr
			}))
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(tc.body))
			if tc.signature != "" {
				req.Header.Set(signatureHeader, tc.signature)
			}
			rec := httptest.NewRecorder()

			h.Receive(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if processed != tc.wantProcessed {
				t.Errorf("processed = %v, want %v", processed, tc.wantProcessed)
			}
		})
	}
}

func TestReceiveRejectsOversizedBody(t *testing.T) {
	body := strings.Repeat("a", maxBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set(signatureHeader, sign(body))
	rec := httptest.NewRecorder()

	newTestHandler(t, nil).Receive(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestReceiveDecodesMessages(t *testing.T) {
	var got *Notification
	h := newTestHandler(t, ProcessorFunc(func(_ context.Context, n *Notification) error {
		got = n
		return nil
	}))
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(textMessagePayload))
	req.Header.Set(signatureHeader, sign(textMessagePayload))

	h.Receive(httptest.NewRecorder(), req)

	if got == nil {
		t.Fatal("processor not called")
	}
	v, err := got.Entry[0].Changes[0].Messages()
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if v.Metadata.PhoneNumberID != "106540352242922" {
		t.Errorf("PhoneNumberID = %q", v.Metadata.PhoneNumberID)
	}
	if len(v.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(v.Messages))
	}
	msg := v.Messages[0]
	if msg.Type != MessageTypeText || msg.Text == nil || msg.Text.Body != "Does it come in another color?" {
		t.Errorf("unexpected message: %+v", msg)
	}
	ts, err := msg.Time()
	if err != nil || ts.Unix() != 1749416383 {
		t.Errorf("Time() = %v, %v", ts, err)
	}
}

func TestChangeMessagesRejectsOtherFields(t *testing.T) {
	c := Change{Field: "account_update", Value: []byte(`{}`)}
	if _, err := c.Messages(); err == nil {
		t.Fatal("Messages succeeded for non-messages field, want error")
	}
}
