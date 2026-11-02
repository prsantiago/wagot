package bot

import (
	"context"
	"errors"
	"testing"
)

func TestContextReply(t *testing.T) {
	sender := &fakeSender{}
	msg := Message{
		ID:   "wamid.123",
		From: "15551234567",
		Type: MessageTypeText,
		Text: "Hello bot",
	}
	c := NewContext(context.Background(), sender, "PNID_TEST", msg)

	if c.Message.Text != "Hello bot" {
		t.Fatalf("c.Message.Text = %q, want %q", c.Message.Text, "Hello bot")
	}

	err := c.Reply("Hello user")
	if err != nil {
		t.Fatalf("c.Reply() error = %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("len(sender.sent) = %d, want 1", len(sender.sent))
	}
	if sender.sent[0].phoneNumberID != "PNID_TEST" {
		t.Errorf("sender.phoneNumberID = %q, want %q", sender.sent[0].phoneNumberID, "PNID_TEST")
	}
	if sender.sent[0].to != "15551234567" {
		t.Errorf("sender.to = %q, want %q", sender.sent[0].to, "15551234567")
	}
	if sender.sent[0].body != "Hello user" {
		t.Errorf("sender.sent[0].body = %q, want 'Hello user'", sender.sent[0].body)
	}
	if len(sender.msgs) != 1 || sender.msgs[0].Type != MessageTypeText || sender.msgs[0].Text.Body != "Hello user" {
		t.Errorf("sender.msgs[0] = %+v, want TextMessage with 'Hello user'", sender.msgs[0])
	}
}

func TestContextReact(t *testing.T) {
	sender := &fakeSender{}
	msg := Message{
		ID:   "wamid.msg456",
		From: "15551234567",
		Type: MessageTypeText,
		Text: "Interesting",
	}
	c := NewContext(context.Background(), sender, "PNID_TEST", msg)

	err := c.React("👍")
	if err != nil {
		t.Fatalf("c.React() error = %v", err)
	}

	if len(sender.msgs) != 1 {
		t.Fatalf("len(sender.msgs) = %d, want 1", len(sender.msgs))
	}
	if sender.msgs[0].Type != MessageTypeReaction {
		t.Fatalf("msgs[0].Type = %q, want %q", sender.msgs[0].Type, MessageTypeReaction)
	}
	if sender.msgs[0].Reaction.Emoji != "👍" {
		t.Errorf("Reaction.Emoji = %q, want '👍'", sender.msgs[0].Reaction.Emoji)
	}
	if sender.msgs[0].Reaction.MessageID != "wamid.msg456" {
		t.Errorf("Reaction.MessageID = %q, want 'wamid.msg456'", sender.msgs[0].Reaction.MessageID)
	}
}

func TestContextErrorPropagation(t *testing.T) {
	expectedErr := errors.New("network failure")
	sender := &fakeSender{sendErr: expectedErr}
	msg := Message{
		ID:   "wamid.123",
		From: "15551234567",
		Type: MessageTypeText,
		Text: "test",
	}
	c := NewContext(context.Background(), sender, "PNID_TEST", msg)

	if err := c.Reply("test"); !errors.Is(err, expectedErr) {
		t.Errorf("c.Reply() error = %v, want %v", err, expectedErr)
	}

	if err := c.React("❤️"); !errors.Is(err, expectedErr) {
		t.Errorf("c.React() error = %v, want %v", err, expectedErr)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	sender := &fakeSender{}
	msg := Message{
		ID:   "wamid.123",
		From: "15551234567",
		Type: MessageTypeText,
		Text: "test",
	}
	c := NewContext(ctx, sender, "PNID_TEST", msg)

	// Context should embed context.Context and reflect cancellation
	if err := c.Err(); !errors.Is(err, context.Canceled) {
		t.Errorf("c.Err() = %v, want context.Canceled", err)
	}

	select {
	case <-c.Done():
		// ok
	default:
		t.Fatal("c.Done() channel should be closed")
	}

	if err := c.Reply("test"); !errors.Is(err, context.Canceled) {
		t.Errorf("c.Reply() error = %v, want context.Canceled", err)
	}

	if err := c.React("👍"); !errors.Is(err, context.Canceled) {
		t.Errorf("c.React() error = %v, want context.Canceled", err)
	}
}
