package bot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chachin/wagot/internal/cloudapi"
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

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func newTestBot(t *testing.T, s Sender, h Handler, cfg Config) *Bot {
	t.Helper()
	if cfg.VerifyToken == "" {
		cfg.VerifyToken = testVerifyToken
	}
	if cfg.AppSecret == "" {
		cfg.AppSecret = testAppSecret
	}
	b, err := New(s, h, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestNewValidatesConfig(t *testing.T) {
	s := &fakeSender{}
	tests := map[string]struct {
		sender  Sender
		handler Handler
		cfg     Config
		wantErr bool
	}{
		"valid": {
			sender:  s,
			handler: echo,
			cfg:     Config{VerifyToken: testVerifyToken, AppSecret: testAppSecret},
		},
		"missing verify token": {
			sender:  s,
			handler: echo,
			cfg:     Config{AppSecret: testAppSecret},
			wantErr: true,
		},
		"missing app secret": {
			sender:  s,
			handler: echo,
			cfg:     Config{VerifyToken: testVerifyToken},
			wantErr: true,
		},
		"missing sender": {
			sender:  nil,
			handler: echo,
			cfg:     Config{VerifyToken: testVerifyToken, AppSecret: testAppSecret},
			wantErr: true,
		},
		"missing handler": {
			sender:  s,
			handler: nil,
			cfg:     Config{VerifyToken: testVerifyToken, AppSecret: testAppSecret},
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b, err := New(tc.sender, tc.handler, tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("New() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil {
				shutdown(t, b)
			}
		})
	}
}

func TestServeHTTPVerify(t *testing.T) {
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
	b := newTestBot(t, &fakeSender{}, echo, Config{})
	defer shutdown(t, b)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			q := url.Values{
				"hub.mode":         {tc.mode},
				"hub.verify_token": {tc.token},
				"hub.challenge":    {"1158201444"},
			}
			req := httptest.NewRequest(http.MethodGet, "/webhook?"+q.Encode(), nil)
			rec := httptest.NewRecorder()

			b.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestServeHTTPSecurity(t *testing.T) {
	tests := map[string]struct {
		body       string
		signature  string
		wantStatus int
	}{
		"missing signature": {
			body:       textMessagePayload,
			signature:  "",
			wantStatus: http.StatusUnauthorized,
		},
		"wrong signature": {
			body:       textMessagePayload,
			signature:  sign(testAppSecret, "other body"),
			wantStatus: http.StatusUnauthorized,
		},
		"non-hex signature": {
			body:       textMessagePayload,
			signature:  "sha256=zz",
			wantStatus: http.StatusUnauthorized,
		},
		"malformed json": {
			body:       "{",
			signature:  sign(testAppSecret, "{"),
			wantStatus: http.StatusBadRequest,
		},
		"unsupported object": {
			body:       `{"object":"page"}`,
			signature:  sign(testAppSecret, `{"object":"page"}`),
			wantStatus: http.StatusOK,
		},
	}

	b := newTestBot(t, &fakeSender{}, echo, Config{})
	defer shutdown(t, b)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(tc.body))
			if tc.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tc.signature)
			}
			rec := httptest.NewRecorder()

			b.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestServeHTTPRejectsOversizedBody(t *testing.T) {
	b := newTestBot(t, &fakeSender{}, echo, Config{})
	defer shutdown(t, b)

	body := strings.Repeat("a", (1<<20)+1)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(testAppSecret, body))
	rec := httptest.NewRecorder()

	b.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestServeHTTPReceiveAndDispatch(t *testing.T) {
	s := &fakeSender{}
	b := newTestBot(t, s, echo, Config{})
	defer shutdown(t, b)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(textMessagePayload))
	req.Header.Set("X-Hub-Signature-256", sign(testAppSecret, textMessagePayload))
	rec := httptest.NewRecorder()

	b.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	waitFor(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.sent) == 1 && len(s.read) == 1
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	wantRead := []string{"wamid.HBgLMTY1MDM4Nzk0MzkVAgASGBQzQTRBNjU5OUFFRTAzODEwMTQ0RgA="}
	if !slices.Equal(s.read, wantRead) {
		t.Errorf("read = %v, want %v", s.read, wantRead)
	}
	wantSent := []sent{{"106540352242922", "16505551234", "Sheena Nelson: Does it come in another color?"}}
	if !slices.Equal(s.sent, wantSent) {
		t.Errorf("sent = %v, want %v", s.sent, wantSent)
	}
}

type sent struct {
	phoneNumberID, to, body string
}

type fakeSender struct {
	mu   sync.Mutex
	read []string
	sent []sent
}

func (f *fakeSender) Send(_ context.Context, phoneNumberID, to string, msg cloudapi.Message) (*cloudapi.SendResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{phoneNumberID, to, msg.Text.Body})
	return &cloudapi.SendResponse{}, nil
}

func (f *fakeSender) MarkRead(_ context.Context, _, messageID string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.read = append(f.read, messageID)
	return nil
}

func echo(_ context.Context, in Incoming) ([]cloudapi.Message, error) {
	return []cloudapi.Message{cloudapi.TextMessage(in.Contact.Profile.Name + ": " + in.Message.Text.Body)}, nil
}

func postNotification(t *testing.T, b *Bot, phoneID string, msgs ...[3]string) *httptest.ResponseRecorder {
	t.Helper()
	if phoneID == "" {
		phoneID = "PNID"
	}
	v := wireMessagesValue{
		MessagingProduct: "whatsapp",
		Metadata:         wireMetadata{PhoneNumberID: phoneID},
	}
	for _, m := range msgs {
		v.Contacts = append(v.Contacts, Contact{WaID: m[0], Profile: Profile{Name: "name-" + m[0]}})
		v.Messages = append(v.Messages, wireMessage{
			From: m[0], ID: m[1], Type: string(MessageTypeText), Text: &Text{Body: m[2]},
		})
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	n := wireNotification{
		Object: objectWhatsAppBusinessAccount,
		Entry:  []wireEntry{{ID: "WABA", Changes: []wireChange{{Field: fieldMessages, Value: raw}}}},
	}
	payload, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sign(testAppSecret, string(payload)))
	rec := httptest.NewRecorder()
	b.ServeHTTP(rec, req)
	return rec
}

func shutdown(t *testing.T, b *Bot) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestRepliesToMessage(t *testing.T) {
	s := &fakeSender{}
	b := newTestBot(t, s, echo, Config{})

	rec := postNotification(t, b, "", [3]string{"111", "wamid.1", "hola"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	shutdown(t, b)

	if want := []string{"wamid.1"}; !slices.Equal(s.read, want) {
		t.Errorf("read = %v, want %v", s.read, want)
	}
	if want := []sent{{"PNID", "111", "name-111: hola"}}; !slices.Equal(s.sent, want) {
		t.Errorf("sent = %v, want %v", s.sent, want)
	}
}

func TestIgnoresOtherPhoneNumbers(t *testing.T) {
	s := &fakeSender{}
	b := newTestBot(t, s, echo, Config{PhoneNumberID: "OTHER"})

	rec := postNotification(t, b, "PNID", [3]string{"111", "wamid.1", "hola"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	shutdown(t, b)

	if len(s.read) != 0 || len(s.sent) != 0 {
		t.Errorf("handled message for another number: read=%v sent=%v", s.read, s.sent)
	}
}

func TestIgnoresDuplicateDeliveries(t *testing.T) {
	s := &fakeSender{}
	b := newTestBot(t, s, echo, Config{})

	for range 3 {
		rec := postNotification(t, b, "", [3]string{"111", "wamid.1", "hola"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	}
	shutdown(t, b)

	if len(s.sent) != 1 {
		t.Errorf("sent %d replies, want 1", len(s.sent))
	}
}

func TestPreservesPerUserOrder(t *testing.T) {
	s := &fakeSender{}
	b := newTestBot(t, s, func(_ context.Context, in Incoming) ([]cloudapi.Message, error) {
		return []cloudapi.Message{cloudapi.TextMessage(in.Message.Text.Body)}, nil
	}, Config{Workers: 8})

	var want []string
	for i := range 50 {
		body := fmt.Sprint(i)
		want = append(want, body)
		rec := postNotification(t, b, "", [3]string{"111", "wamid." + body, body})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	}
	shutdown(t, b)

	var got []string
	for _, m := range s.sent {
		got = append(got, m.body)
	}
	if !slices.Equal(got, want) {
		t.Errorf("replies out of order: %v", got)
	}
}

func TestQueueFullRejectsAndAllowsRedelivery(t *testing.T) {
	release := make(chan struct{})
	s := &fakeSender{}
	b := newTestBot(t, s, func(ctx context.Context, in Incoming) ([]cloudapi.Message, error) {
		<-release
		return nil, nil
	}, Config{Workers: 1, QueueSize: 1})
	defer func() { close(release); shutdown(t, b) }()

	// The first message occupies the worker, the second fills the queue.
	// Wait until the worker has picked up the first so the queue is empty.
	_ = postNotification(t, b, "", [3]string{"1", "a", "x"})
	waitFor(t, func() bool { return len(b.queues[0]) == 0 })
	_ = postNotification(t, b, "", [3]string{"1", "b", "x"})

	rec := postNotification(t, b, "", [3]string{"1", "c", "x"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !b.seen.add("c") {
		t.Error("rejected message was remembered; redelivery would be dropped")
	}
}

func TestProcessAfterShutdown(t *testing.T) {
	b := newTestBot(t, &fakeSender{}, echo, Config{})
	shutdown(t, b)

	rec := postNotification(t, b, "", [3]string{"1", "a", "x"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestShutdownTimeoutCancelsHandlers(t *testing.T) {
	b := newTestBot(t, &fakeSender{}, func(ctx context.Context, in Incoming) ([]cloudapi.Message, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}, Config{})
	_ = postNotification(t, b, "", [3]string{"1", "a", "x"})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := b.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v, want DeadlineExceeded", err)
	}
}

func TestRecoversFromHandlerPanic(t *testing.T) {
	s := &fakeSender{}
	b := newTestBot(t, s, func(ctx context.Context, in Incoming) ([]cloudapi.Message, error) {
		if in.Message.ID == "boom" {
			panic("boom")
		}
		return echo(ctx, in)
	}, Config{Workers: 1})

	_ = postNotification(t, b, "", [3]string{"1", "boom", "x"})
	_ = postNotification(t, b, "", [3]string{"1", "ok", "y"})
	shutdown(t, b)

	if len(s.sent) != 1 {
		t.Errorf("sent %d replies, want 1", len(s.sent))
	}
}

func TestDedupExpires(t *testing.T) {
	now := time.Unix(0, 0)
	d := newDedup(time.Hour)
	d.now = func() time.Time { return now }

	if !d.add("x") || d.add("x") {
		t.Fatal("add should accept once, then reject")
	}
	now = now.Add(2 * time.Hour)
	if !d.add("x") {
		t.Error("add rejected id after TTL")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met")
		}
		time.Sleep(time.Millisecond)
	}
}
