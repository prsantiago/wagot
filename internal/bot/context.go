package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/chachin/wagot/internal/cloudapi"
)

// Message is an unnested incoming WhatsApp message provided to a conversational handler.
type Message struct {
	ID          string
	From        string
	Type        MessageType
	Text        string
	SelectionID string
}

// Contact represents the sender of an incoming WhatsApp message.
type Contact struct {
	Profile Profile
	WaID    string
}

// Profile holds a WhatsApp user's profile information.
type Profile struct {
	Name string
}

// Reply is a declarative outbound reply.
type Reply interface {
	toCloudAPI() (cloudapi.Message, error)
}

// Text is a text reply with optional URL preview.
type Text struct {
	Body       string
	PreviewURL bool
}

func (t Text) toCloudAPI() (cloudapi.Message, error) {
	if t.Body == "" {
		return cloudapi.Message{}, errors.New("bot: text body cannot be empty")
	}
	return cloudapi.TextMessageWithPreview(t.Body, t.PreviewURL), nil
}

// Image is a standalone image reply with a public HTTPS URL and optional caption.
type Image struct {
	URL     string
	Caption string
}

func (img Image) toCloudAPI() (cloudapi.Message, error) {
	if img.URL == "" {
		return cloudapi.Message{}, errors.New("bot: image URL cannot be empty")
	}
	if !strings.HasPrefix(img.URL, "https://") {
		return cloudapi.Message{}, errors.New("bot: image URL must use https")
	}
	return cloudapi.ImageMessage(img.URL, img.Caption), nil
}

// Button is an action button on a Button Reply.
type Button struct {
	ID    string
	Title string
}

// Buttons is an interactive button reply presenting up to three buttons.
type Buttons struct {
	Body    string
	Buttons []Button
}

func (b Buttons) toCloudAPI() (cloudapi.Message, error) {
	if b.Body == "" {
		return cloudapi.Message{}, errors.New("bot: button reply body cannot be empty")
	}
	if len(b.Buttons) == 0 || len(b.Buttons) > 3 {
		return cloudapi.Message{}, errors.New("bot: button reply must have between 1 and 3 buttons")
	}
	cloudButtons := make([]cloudapi.Button, len(b.Buttons))
	for i, btn := range b.Buttons {
		if btn.ID == "" || btn.Title == "" {
			return cloudapi.Message{}, errors.New("bot: button ID and title cannot be empty")
		}
		cloudButtons[i] = cloudapi.Button{
			ID:    btn.ID,
			Title: btn.Title,
		}
	}
	return cloudapi.ButtonMessage(b.Body, cloudButtons), nil
}

// ListItem represents a selectable item inside a List Reply.
type ListItem struct {
	ID          string
	Title       string
	Description string
}

// List is an interactive list reply presenting a menu button opening a selectable list of options.
type List struct {
	Body   string
	Button string
	Items  []ListItem
}

func (l List) toCloudAPI() (cloudapi.Message, error) {
	if l.Body == "" {
		return cloudapi.Message{}, errors.New("bot: list reply body cannot be empty")
	}
	if l.Button == "" {
		return cloudapi.Message{}, errors.New("bot: list reply button cannot be empty")
	}
	if len(l.Items) == 0 {
		return cloudapi.Message{}, errors.New("bot: list reply must have at least 1 item")
	}
	if len(l.Items) > 10 {
		return cloudapi.Message{}, errors.New("bot: list reply cannot have more than 10 items")
	}
	rows := make([]cloudapi.ListRow, len(l.Items))
	for i, item := range l.Items {
		if item.ID == "" || item.Title == "" {
			return cloudapi.Message{}, errors.New("bot: list item ID and title cannot be empty")
		}
		rows[i] = cloudapi.ListRow{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
		}
	}
	return cloudapi.ListMessage(l.Body, l.Button, rows), nil
}

// Context provides conversational context and outbound intent methods to a Handler.
type Context struct {
	context.Context
	Message       Message
	Contact       Contact
	sender        Sender
	phoneNumberID string
	logger        *slog.Logger
}

// NewContext returns a new Context for conversational dispatch or testing.
func NewContext(ctx context.Context, sender Sender, phoneNumberID string, msg Message) *Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Context{
		Context:       ctx,
		Message:       msg,
		sender:        sender,
		phoneNumberID: phoneNumberID,
	}
}

// Send dispatches a declarative Reply via the Cloud API sender interface.
func (c *Context) Send(r Reply) error {
	if r == nil {
		return errors.New("bot: reply is nil")
	}
	msg, err := r.toCloudAPI()
	if err != nil {
		return err
	}
	return c.send("reply", msg)
}

// Reply dispatches an immediate text reply via the Cloud API sender interface.
func (c *Context) Reply(text string) error {
	return c.Send(Text{Body: text})
}

// React attaches an immediate emoji reaction to the incoming message ID.
func (c *Context) React(emoji string) error {
	return c.send("reaction", cloudapi.ReactionMessage(c.Message.ID, emoji))
}

func (c *Context) send(action string, msg cloudapi.Message) error {
	if c.sender == nil {
		return errors.New("bot: context has no sender")
	}
	resp, err := c.sender.Send(c.Context, c.phoneNumberID, c.Message.From, msg)
	if err != nil {
		return err
	}
	if c.logger != nil {
		c.logger.InfoContext(c.Context, action+" sent", "reply_id", resp.MessageID(), "type", msg.Type)
	}
	return nil
}
