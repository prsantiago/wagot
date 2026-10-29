package cloudapi

import "encoding/json"

// Message is an outbound message. Build one with [TextMessage],
// [ReactionMessage] or [TemplateMessage], or fill in the fields directly.
// The recipient is passed to [Client.Send], not stored here.
type Message struct {
	Type    string   `json:"type"`
	Context *Context `json:"context,omitempty"`

	Text     *Text     `json:"text,omitempty"`
	Reaction *Reaction `json:"reaction,omitempty"`
	Image    *Media    `json:"image,omitempty"`
	Audio    *Media    `json:"audio,omitempty"`
	Video    *Media    `json:"video,omitempty"`
	Document *Media    `json:"document,omitempty"`
	Sticker  *Media    `json:"sticker,omitempty"`
	Template *Template `json:"template,omitempty"`

	// Interactive holds a raw interactive object (buttons, lists, flows...).
	// See the Cloud API reference for its schema.
	Interactive json.RawMessage `json:"interactive,omitempty"`
}

// Context makes a message a quoted reply to MessageID.
type Context struct {
	MessageID string `json:"message_id"`
}

// Text is the content of a text message.
type Text struct {
	Body       string `json:"body"`
	PreviewURL bool   `json:"preview_url,omitempty"`
}

// Reaction reacts to MessageID with Emoji. An empty Emoji removes the reaction.
type Reaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

// Media references either an uploaded media ID or a public HTTPS link.
type Media struct {
	ID       string `json:"id,omitempty"`
	Link     string `json:"link,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// Template is a pre-approved message template. Templates are the only
// messages you can send outside the 24-hour customer service window.
type Template struct {
	Name     string   `json:"name"`
	Language Language `json:"language"`
	// Components holds raw template components (header/body/button parameters).
	Components json.RawMessage `json:"components,omitempty"`
}

// Language selects a template translation, for example "en_US".
type Language struct {
	Code string `json:"code"`
}

// TextMessage returns a text message.
func TextMessage(body string) Message {
	return Message{Type: "text", Text: &Text{Body: body}}
}

// ReactionMessage returns a reaction to messageID.
func ReactionMessage(messageID, emoji string) Message {
	return Message{Type: "reaction", Reaction: &Reaction{MessageID: messageID, Emoji: emoji}}
}

// TemplateMessage returns a template message without parameters.
func TemplateMessage(name, languageCode string) Message {
	return Message{Type: "template", Template: &Template{Name: name, Language: Language{Code: languageCode}}}
}

// SendResponse is returned by the Cloud API after a message is accepted.
type SendResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts         []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID            string `json:"id"`
		MessageStatus string `json:"message_status,omitempty"`
	} `json:"messages"`
}

// MessageID returns the ID of the accepted message, or "" if none.
func (r *SendResponse) MessageID() string {
	if r == nil || len(r.Messages) == 0 {
		return ""
	}
	return r.Messages[0].ID
}
