package bot

import (
	"context"
	"errors"
	"log/slog"

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
