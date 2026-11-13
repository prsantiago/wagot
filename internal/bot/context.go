package bot

import (
	"context"
	"errors"
	"fmt"
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
	Validate() error
	toCloudAPI() (cloudapi.Message, error)
}

// Text is a text reply with optional URL preview.
type Text struct {
	Body       string
	PreviewURL bool
}

// Validate ensures text reply invariants hold.
func (t Text) Validate() error {
	if strings.TrimSpace(t.Body) == "" {
		return errors.New("bot: text reply body cannot be empty")
	}
	return nil
}

func (t Text) toCloudAPI() (cloudapi.Message, error) {
	if err := t.Validate(); err != nil {
		return cloudapi.Message{}, err
	}
	return cloudapi.TextMessageWithPreview(t.Body, t.PreviewURL), nil
}

// Image is a standalone image reply with a public HTTPS URL and optional caption.
type Image struct {
	URL     string
	Caption string
}

// Validate ensures image reply invariants hold.
func (img Image) Validate() error {
	url := strings.TrimSpace(img.URL)
	if url == "" {
		return errors.New("bot: image reply url cannot be empty")
	}
	if !strings.HasPrefix(strings.ToLower(url), "https://") {
		return errors.New("bot: image reply url must be a public HTTPS URL")
	}
	return nil
}

func (img Image) toCloudAPI() (cloudapi.Message, error) {
	if err := img.Validate(); err != nil {
		return cloudapi.Message{}, err
	}
	return cloudapi.ImageMessage(strings.TrimSpace(img.URL), img.Caption), nil
}

// Button is an action button on a Button Reply.
type Button struct {
	ID    string
	Title string
}

// Validate ensures button invariants hold.
func (b Button) Validate() error {
	if strings.TrimSpace(b.ID) == "" {
		return errors.New("bot: button id cannot be empty")
	}
	if strings.TrimSpace(b.Title) == "" {
		return errors.New("bot: button title cannot be empty")
	}
	return nil
}

// Buttons is an interactive button reply presenting up to three buttons.
type Buttons struct {
	Body    string
	Buttons []Button
}

// Validate ensures buttons reply invariants hold.
func (b Buttons) Validate() error {
	if strings.TrimSpace(b.Body) == "" {
		return errors.New("bot: buttons reply body is required")
	}
	if len(b.Buttons) < 1 || len(b.Buttons) > 3 {
		return errors.New("bot: buttons reply must have between 1 and 3 buttons")
	}
	for i, btn := range b.Buttons {
		if strings.TrimSpace(btn.ID) == "" {
			return fmt.Errorf("bot: button[%d]: button id cannot be empty", i)
		}
		if strings.TrimSpace(btn.Title) == "" {
			return fmt.Errorf("bot: button[%d]: button title cannot be empty", i)
		}
	}
	return nil
}

func (b Buttons) toCloudAPI() (cloudapi.Message, error) {
	if err := b.Validate(); err != nil {
		return cloudapi.Message{}, err
	}
	cloudButtons := make([]cloudapi.Button, len(b.Buttons))
	for i, btn := range b.Buttons {
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

// Validate ensures list item invariants hold.
func (it ListItem) Validate() error {
	if strings.TrimSpace(it.ID) == "" {
		return errors.New("bot: list item id cannot be empty")
	}
	if strings.TrimSpace(it.Title) == "" {
		return errors.New("bot: list item title cannot be empty")
	}
	return nil
}

// List is an interactive list reply presenting a menu button opening a selectable list of options.
type List struct {
	Body   string
	Button string
	Items  []ListItem
}

// Validate ensures list reply invariants hold.
func (l List) Validate() error {
	if strings.TrimSpace(l.Body) == "" {
		return errors.New("bot: list reply body cannot be empty")
	}
	if strings.TrimSpace(l.Button) == "" {
		return errors.New("bot: list reply button label cannot be empty")
	}
	if len(l.Items) < 1 || len(l.Items) > 10 {
		return errors.New("bot: list reply must have between 1 and 10 items")
	}
	for i, item := range l.Items {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("bot: item[%d]: list item id cannot be empty", i)
		}
		if strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("bot: item[%d]: list item title cannot be empty", i)
		}
	}
	return nil
}

func (l List) toCloudAPI() (cloudapi.Message, error) {
	if err := l.Validate(); err != nil {
		return cloudapi.Message{}, err
	}
	rows := make([]cloudapi.ListRow, len(l.Items))
	for i, item := range l.Items {
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
