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

	// Interactive holds an interactive reply object (Button Reply, List Reply).
	// See the Cloud API reference for its schema.
	Interactive *Interactive `json:"interactive,omitempty"`
}

// Interactive represents the payload of an interactive message (Button Reply, List Reply).
type Interactive struct {
	Type   string             `json:"type"`
	Body   *InteractiveBody   `json:"body,omitempty"`
	Action *InteractiveAction `json:"action,omitempty"`
}

// InteractiveBody contains the text body for an interactive message.
type InteractiveBody struct {
	Text string `json:"text"`
}

// InteractiveAction contains the interactive components (buttons, sections).
type InteractiveAction struct {
	Button   string               `json:"button,omitempty"`
	Buttons  []InteractiveButton  `json:"buttons,omitempty"`
	Sections []InteractiveSection `json:"sections,omitempty"`
}

// InteractiveSection contains a list of rows.
type InteractiveSection struct {
	Title string           `json:"title,omitempty"`
	Rows  []InteractiveRow `json:"rows"`
}

// InteractiveRow represents a single selectable item inside a list section.
type InteractiveRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// ListRow is a row definition for constructing a list reply.
type ListRow struct {
	ID          string
	Title       string
	Description string
}

// InteractiveButton represents an action button inside an interactive Button Reply.
type InteractiveButton struct {
	Type  string                 `json:"type"` // always "reply" for button replies
	Reply InteractiveButtonReply `json:"reply"`
}

// InteractiveButtonReply contains the ID and title of an interactive button.
type InteractiveButtonReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Button is an action button definition for constructing a button reply.
type Button struct {
	ID    string
	Title string
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

// TextMessageWithPreview returns a text message with link preview enabled or disabled.
func TextMessageWithPreview(body string, previewURL bool) Message {
	return Message{Type: "text", Text: &Text{Body: body, PreviewURL: previewURL}}
}

// ImageMessage returns a standalone image reply referencing a public HTTPS link with an optional caption.
func ImageMessage(link, caption string) Message {
	return Message{
		Type: "image",
		Image: &Media{
			Link:    link,
			Caption: caption,
		},
	}
}

// ButtonMessage returns an interactive button reply message.
func ButtonMessage(body string, buttons []Button) Message {
	interactiveButtons := make([]InteractiveButton, len(buttons))
	for i, b := range buttons {
		interactiveButtons[i] = InteractiveButton{
			Type: "reply",
			Reply: InteractiveButtonReply{
				ID:    b.ID,
				Title: b.Title,
			},
		}
	}
	return Message{
		Type: "interactive",
		Interactive: &Interactive{
			Type:   "button",
			Body:   &InteractiveBody{Text: body},
			Action: &InteractiveAction{Buttons: interactiveButtons},
		},
	}
}

// ListMessage returns an interactive list reply message.
func ListMessage(body, button string, rows []ListRow) Message {
	interactiveRows := make([]InteractiveRow, len(rows))
	for i, r := range rows {
		interactiveRows[i] = InteractiveRow{
			ID:          r.ID,
			Title:       r.Title,
			Description: r.Description,
		}
	}
	return Message{
		Type: "interactive",
		Interactive: &Interactive{
			Type: "list",
			Body: &InteractiveBody{Text: body},
			Action: &InteractiveAction{
				Button: button,
				Sections: []InteractiveSection{
					{Rows: interactiveRows},
				},
			},
		},
	}
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
