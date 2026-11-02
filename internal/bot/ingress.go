package bot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const (
	maxBodyBytes = 1 << 20 // 1MB

	signatureHeader = "X-Hub-Signature-256"
	signaturePrefix = "sha256="

	objectWhatsAppBusinessAccount = "whatsapp_business_account"
	fieldMessages                 = "messages"
)

// MessageType identifies the kind of content in a Message.
type MessageType string

// Message types sent by the WhatsApp Business Platform.
const (
	MessageTypeText        MessageType = "text"
	MessageTypeImage       MessageType = "image"
	MessageTypeAudio       MessageType = "audio"
	MessageTypeVideo       MessageType = "video"
	MessageTypeDocument    MessageType = "document"
	MessageTypeSticker     MessageType = "sticker"
	MessageTypeLocation    MessageType = "location"
	MessageTypeContacts    MessageType = "contacts"
	MessageTypeReaction    MessageType = "reaction"
	MessageTypeButton      MessageType = "button"
	MessageTypeInteractive MessageType = "interactive"
	MessageTypeOrder       MessageType = "order"
	MessageTypeSystem      MessageType = "system"
	MessageTypeUnsupported MessageType = "unsupported"
)

// Message is an inbound message from a WhatsApp user.
type Message struct {
	From        string       `json:"from"`
	ID          string       `json:"id"`
	Timestamp   string       `json:"timestamp"`
	Type        MessageType  `json:"type"`
	Text        *Text        `json:"text,omitempty"`
	Interactive *Interactive `json:"interactive,omitempty"`
	Reaction    *Reaction    `json:"reaction,omitempty"`
}

// Text is the content of a text message.
type Text struct {
	Body string `json:"body"`
}

// Interactive is a reply to an interactive (button or list) message.
type Interactive struct {
	Type        string       `json:"type"`
	ButtonReply *ButtonReply `json:"button_reply,omitempty"`
	ListReply   *ListReply   `json:"list_reply,omitempty"`
}

// ButtonReply is the button a user tapped.
type ButtonReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListReply is the list row a user selected.
type ListReply struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// Reaction is an emoji reaction to a previous message.
type Reaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

// Contact is the WhatsApp user who sent a message.
type Contact struct {
	Profile Profile `json:"profile"`
	WaID    string  `json:"wa_id"`
}

// Profile holds a WhatsApp user's public profile data.
type Profile struct {
	Name string `json:"name"`
}

// Unexported wire structs for decoding Meta's webhook envelope.
type wireNotification struct {
	Object string      `json:"object"`
	Entry  []wireEntry `json:"entry"`
}

type wireEntry struct {
	ID      string       `json:"id"`
	Changes []wireChange `json:"changes"`
}

type wireChange struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

type wireMessagesValue struct {
	MessagingProduct string        `json:"messaging_product"`
	Metadata         wireMetadata  `json:"metadata"`
	Contacts         []Contact     `json:"contacts,omitempty"`
	Messages         []wireMessage `json:"messages,omitempty"`
	Statuses         []wireStatus  `json:"statuses,omitempty"`
	Errors           []wireError   `json:"errors,omitempty"`
}

type wireMessage struct {
	From        string              `json:"from"`
	ID          string              `json:"id"`
	Timestamp   string              `json:"timestamp"`
	Type        string              `json:"type"`
	Context     *wireMessageContext `json:"context,omitempty"`
	Text        *Text               `json:"text,omitempty"`
	Image       *wireMedia          `json:"image,omitempty"`
	Audio       *wireMedia          `json:"audio,omitempty"`
	Video       *wireMedia          `json:"video,omitempty"`
	Document    *wireMedia          `json:"document,omitempty"`
	Sticker     *wireMedia          `json:"sticker,omitempty"`
	Location    *wireLocation       `json:"location,omitempty"`
	Reaction    *Reaction           `json:"reaction,omitempty"`
	Button      *wireButton         `json:"button,omitempty"`
	Interactive *Interactive        `json:"interactive,omitempty"`
	Errors      []wireError         `json:"errors,omitempty"`
}

type wireMessageContext struct {
	From      string `json:"from,omitempty"`
	ID        string `json:"id,omitempty"`
	Forwarded bool   `json:"forwarded,omitempty"`
}

type wireMedia struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
	Voice    bool   `json:"voice,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

type wireLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

type wireButton struct {
	Payload string `json:"payload"`
	Text    string `json:"text"`
}

type wireMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type wireStatus struct {
	ID          string      `json:"id"`
	Status      string      `json:"status"`
	RecipientID string      `json:"recipient_id"`
	Errors      []wireError `json:"errors,omitempty"`
}

type wireError struct {
	Code  int    `json:"code"`
	Title string `json:"title"`
}

// ServeHTTP implements http.Handler for webhook ingress.
// GET handles the Meta verification handshake; POST receives notifications.
func (b *Bot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		b.verify(w, r)
	case http.MethodPost:
		b.receive(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (b *Bot) verify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("hub.mode") != "subscribe" || !secureEqual(q.Get("hub.verify_token"), b.verifyToken) {
		b.logger.WarnContext(r.Context(), "webhook verification failed", "remote_addr", r.RemoteAddr)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, q.Get("hub.challenge"))
}

func (b *Bot) receive(w http.ResponseWriter, r *http.Request) {
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

	if !b.validSignature(body, r.Header.Get(signatureHeader)) {
		b.logger.WarnContext(ctx, "rejected webhook with invalid signature", "remote_addr", r.RemoteAddr)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var n wireNotification
	if err := json.Unmarshal(body, &n); err != nil {
		b.logger.ErrorContext(ctx, "decode webhook payload", "error", err)
		http.Error(w, "malformed payload", http.StatusBadRequest)
		return
	}

	if n.Object != objectWhatsAppBusinessAccount {
		b.logger.InfoContext(ctx, "ignoring webhook for unsupported object", "object", n.Object)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := b.processNotification(ctx, &n); err != nil {
		b.logger.ErrorContext(ctx, "process webhook", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bot) processNotification(ctx context.Context, n *wireNotification) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return ErrClosed
	}

	for _, entry := range n.Entry {
		for _, change := range entry.Changes {
			if change.Field != fieldMessages {
				continue
			}
			var v wireMessagesValue
			if err := json.Unmarshal(change.Value, &v); err != nil {
				b.logger.ErrorContext(ctx, "decode change", "error", err)
				continue
			}
			if b.phoneNumberID != "" && v.Metadata.PhoneNumberID != b.phoneNumberID {
				b.logger.DebugContext(ctx, "ignoring change for other phone number",
					"phone_number_id", v.Metadata.PhoneNumberID)
				continue
			}
			for _, e := range v.Errors {
				b.logger.WarnContext(ctx, "platform error", "code", e.Code, "title", e.Title)
			}
			for _, s := range v.Statuses {
				b.logWireStatus(ctx, s)
			}
			for _, m := range v.Messages {
				if err := b.enqueue(Incoming{
					PhoneNumberID: v.Metadata.PhoneNumberID,
					Contact:       contactFor(v.Contacts, m.From),
					Message: Message{
						From:        m.From,
						ID:          m.ID,
						Timestamp:   m.Timestamp,
						Type:        MessageType(m.Type),
						Text:        m.Text,
						Interactive: m.Interactive,
						Reaction:    m.Reaction,
					},
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (b *Bot) logWireStatus(ctx context.Context, s wireStatus) {
	attrs := []any{"id", s.ID, "status", s.Status, "recipient_id", s.RecipientID}
	if len(s.Errors) > 0 {
		e := s.Errors[0]
		b.logger.WarnContext(ctx, "message failed", append(attrs, "code", e.Code, "title", e.Title)...)
		return
	}
	b.logger.DebugContext(ctx, "status update", attrs...)
}

func (b *Bot) validSignature(body []byte, header string) bool {
	hexSig, ok := strings.CutPrefix(header, signaturePrefix)
	if !ok {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, b.appSecret)
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

func secureEqual(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
