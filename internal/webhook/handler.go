// Package webhook receives WhatsApp Business Platform (Cloud API) webhooks.
//
// It implements Meta's verification handshake, checks the
// X-Hub-Signature-256 HMAC on every notification, decodes the payload and
// passes it to a [Processor].
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const (
	// maxBodyBytes caps the notification size. Meta payloads are a few KB.
	maxBodyBytes = 1 << 20

	signatureHeader = "X-Hub-Signature-256"
	signaturePrefix = "sha256="
)

// Processor handles verified webhook notifications.
//
// Process runs before the HTTP response is sent, so it should return quickly;
// hand slow work off to a queue. Returning an error makes the handler respond
// 500, which causes Meta to retry the delivery. Meta may deliver the same
// notification more than once, so implementations should be idempotent
// (for example, by de-duplicating on Message.ID).
type Processor interface {
	Process(ctx context.Context, n *Notification) error
}

// ProcessorFunc adapts an ordinary function to the [Processor] interface.
type ProcessorFunc func(ctx context.Context, n *Notification) error

// Process calls f(ctx, n).
func (f ProcessorFunc) Process(ctx context.Context, n *Notification) error { return f(ctx, n) }

// Handler serves the webhook verification and notification endpoints.
type Handler struct {
	verifyToken string
	appSecret   []byte
	processor   Processor
	logger      *slog.Logger
}

// NewHandler returns a Handler.
//
// verifyToken is the token you entered when configuring the webhook in the
// Meta App Dashboard. appSecret is your Meta app's App Secret, used to verify
// payload signatures. A nil logger discards logs.
func NewHandler(verifyToken, appSecret string, p Processor, logger *slog.Logger) (*Handler, error) {
	switch {
	case verifyToken == "":
		return nil, errors.New("webhook: verify token is required")
	case appSecret == "":
		return nil, errors.New("webhook: app secret is required")
	case p == nil:
		return nil, errors.New("webhook: processor is required")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Handler{
		verifyToken: verifyToken,
		appSecret:   []byte(appSecret),
		processor:   p,
		logger:      logger,
	}, nil
}

// Verify handles the GET request Meta sends when you configure the webhook.
// It echoes hub.challenge if hub.verify_token matches the configured token.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("hub.mode") != "subscribe" || !secureEqual(q.Get("hub.verify_token"), h.verifyToken) {
		h.logger.WarnContext(r.Context(), "webhook verification failed", "remote_addr", r.RemoteAddr)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.WriteString(w, q.Get("hub.challenge"))
}

// Receive handles the POST requests that carry webhook notifications.
func (h *Handler) Receive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !h.validSignature(body, r.Header.Get(signatureHeader)) {
		h.logger.WarnContext(ctx, "rejected webhook with invalid signature", "remote_addr", r.RemoteAddr)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var n Notification
	if err := json.Unmarshal(body, &n); err != nil {
		h.logger.ErrorContext(ctx, "decode webhook payload", "error", err)
		http.Error(w, "malformed payload", http.StatusBadRequest)
		return
	}

	// Acknowledge, but skip, objects we don't handle so Meta doesn't retry.
	if n.Object != ObjectWhatsAppBusinessAccount {
		h.logger.InfoContext(ctx, "ignoring webhook for unsupported object", "object", n.Object)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.processor.Process(ctx, &n); err != nil {
		h.logger.ErrorContext(ctx, "process webhook", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// validSignature reports whether header is "sha256=" followed by the hex
// HMAC-SHA256 of body keyed with the app secret.
func (h *Handler) validSignature(body []byte, header string) bool {
	hexSig, ok := strings.CutPrefix(header, signaturePrefix)
	if !ok {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, h.appSecret)
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// secureEqual compares a and b in constant time. Hashing first means the
// comparison doesn't leak the length of the secret either.
func secureEqual(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
