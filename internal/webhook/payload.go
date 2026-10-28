package webhook

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ObjectWhatsAppBusinessAccount is the value of Notification.Object for
// WhatsApp Business Platform webhooks.
const ObjectWhatsAppBusinessAccount = "whatsapp_business_account"

// FieldMessages is the webhook field that carries inbound messages and
// status updates for outbound messages.
const FieldMessages = "messages"

// Notification is the top-level payload Meta POSTs to the webhook endpoint.
// A single notification may batch several entries and changes.
type Notification struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

// Entry groups changes for one WhatsApp Business Account.
type Entry struct {
	// ID is the WhatsApp Business Account ID.
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

// Change is a single change notification for a subscribed field.
//
// The shape of Value depends on Field, so it is kept undecoded. Use
// [Change.Messages] for the "messages" field, or decode Value yourself for
// other fields (for example "message_template_status_update").
type Change struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

// Messages decodes the value of a "messages" change.
func (c Change) Messages() (*MessagesValue, error) {
	if c.Field != FieldMessages {
		return nil, fmt.Errorf("webhook: change field is %q, not %q", c.Field, FieldMessages)
	}
	var v MessagesValue
	if err := json.Unmarshal(c.Value, &v); err != nil {
		return nil, fmt.Errorf("webhook: decode messages value: %w", err)
	}
	return &v, nil
}

// MessagesValue is the value of a "messages" change.
type MessagesValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts,omitempty"`
	Messages         []Message `json:"messages,omitempty"`
	Statuses         []Status  `json:"statuses,omitempty"`
	Errors           []Error   `json:"errors,omitempty"`
}

// Metadata identifies the business phone number that received the event.
type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
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

// Message is an inbound message from a WhatsApp user. Exactly one of the
// content fields is set, according to Type.
type Message struct {
	From      string          `json:"from"`
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	Type      MessageType     `json:"type"`
	Context   *MessageContext `json:"context,omitempty"`

	Text        *Text        `json:"text,omitempty"`
	Image       *Media       `json:"image,omitempty"`
	Audio       *Media       `json:"audio,omitempty"`
	Video       *Media       `json:"video,omitempty"`
	Document    *Media       `json:"document,omitempty"`
	Sticker     *Media       `json:"sticker,omitempty"`
	Location    *Location    `json:"location,omitempty"`
	Reaction    *Reaction    `json:"reaction,omitempty"`
	Button      *Button      `json:"button,omitempty"`
	Interactive *Interactive `json:"interactive,omitempty"`

	Errors []Error `json:"errors,omitempty"`
}

// Time parses the message's Unix timestamp.
func (m Message) Time() (time.Time, error) { return parseUnix(m.Timestamp) }

// MessageContext is set when a message replies to, or was forwarded from,
// another message.
type MessageContext struct {
	From      string `json:"from,omitempty"`
	ID        string `json:"id,omitempty"`
	Forwarded bool   `json:"forwarded,omitempty"`
}

// Text is the content of a text message.
type Text struct {
	Body string `json:"body"`
}

// Media describes an image, audio, video, document or sticker. Download it
// through the Graph API using ID.
type Media struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
	Voice    bool   `json:"voice,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// Location is a shared location.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// Reaction is an emoji reaction to a previous message. An empty Emoji means
// the reaction was removed.
type Reaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

// Button is a reply to a quick-reply button in a template message.
type Button struct {
	Payload string `json:"payload"`
	Text    string `json:"text"`
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

// Status is a delivery status update for an outbound message.
type Status struct {
	ID           string        `json:"id"`
	Status       string        `json:"status"` // sent, delivered, read or failed
	Timestamp    string        `json:"timestamp"`
	RecipientID  string        `json:"recipient_id"`
	Conversation *Conversation `json:"conversation,omitempty"`
	Pricing      *Pricing      `json:"pricing,omitempty"`
	Errors       []Error       `json:"errors,omitempty"`
}

// Time parses the status's Unix timestamp.
func (s Status) Time() (time.Time, error) { return parseUnix(s.Timestamp) }

// Conversation describes the conversation a status belongs to.
type Conversation struct {
	ID                  string             `json:"id"`
	Origin              ConversationOrigin `json:"origin"`
	ExpirationTimestamp string             `json:"expiration_timestamp,omitempty"`
}

// ConversationOrigin describes what opened a conversation.
type ConversationOrigin struct {
	Type string `json:"type"`
}

// Pricing describes how a message is billed.
type Pricing struct {
	Billable     bool   `json:"billable"`
	PricingModel string `json:"pricing_model"`
	Category     string `json:"category"`
}

// Error is an error reported by the WhatsApp Business Platform.
type Error struct {
	Code      int        `json:"code"`
	Title     string     `json:"title"`
	Message   string     `json:"message,omitempty"`
	ErrorData *ErrorData `json:"error_data,omitempty"`
	Href      string     `json:"href,omitempty"`
}

// ErrorData holds extra detail for an Error.
type ErrorData struct {
	Details string `json:"details"`
}

func parseUnix(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("webhook: parse timestamp %q: %w", s, err)
	}
	return time.Unix(sec, 0), nil
}
