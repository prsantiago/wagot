package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/chachin/wagot/internal/cloudapi"
)

// Message is an unnested incoming WhatsApp message provided to a conversational handler.
type Message struct {
	ID   string
	From string
	Type string
	Text string
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

// Reply dispatches an immediate text reply via the Cloud API sender interface.
func (c *Context) Reply(text string) error {
	return c.send("reply", cloudapi.TextMessage(text))
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
