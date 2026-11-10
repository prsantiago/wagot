package flow

import (
	"context"
	"strings"
	"testing"

	"github.com/chachin/wagot/internal/bot"
	"github.com/chachin/wagot/internal/cloudapi"
)

type recordSender struct {
	msgs []cloudapi.Message
}

func (s *recordSender) Send(ctx context.Context, phoneNumberID, to string, msg cloudapi.Message) (*cloudapi.SendResponse, error) {
	s.msgs = append(s.msgs, msg)
	return &cloudapi.SendResponse{}, nil
}

func (s *recordSender) MarkRead(ctx context.Context, phoneNumberID, messageID string, typing bool) error {
	return nil
}

func TestGreetingKeywordsPersonalized(t *testing.T) {
	keywords := []string{
		"hola",
		"Hola",
		"HOLA",
		"¡Hola!",
		"¿informacion?",
		"información",
		"informacion",
		"menu",
		"  MENU  ",
		"¡Menú!",
	}

	for _, kw := range keywords {
		t.Run(kw, func(t *testing.T) {
			sender := &recordSender{}
			h := NewHandler()

			msg := bot.Message{
				ID:   "wamid.1",
				From: "15551234567",
				Type: bot.MessageTypeText,
				Text: kw,
			}
			c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)
			c.Contact = bot.Contact{
				WaID:    "15551234567",
				Profile: bot.Profile{Name: "Santiago"},
			}

			if err := h(c); err != nil {
				t.Fatalf("h(c) error = %v", err)
			}

			if len(sender.msgs) != 2 {
				t.Fatalf("got %d messages sent, want 2 (welcome + main menu)", len(sender.msgs))
			}

			// First message must be personalized welcome text addressing the user
			msg1 := sender.msgs[0]
			if msg1.Type != "text" || msg1.Text == nil {
				t.Fatalf("msg[0] = %+v, want text message", msg1)
			}
			if !strings.Contains(msg1.Text.Body, "Santiago") {
				t.Errorf("msg[0].Text.Body = %q, want it to address 'Santiago'", msg1.Text.Body)
			}

			// Second message must be Main Menu Button Reply
			msg2 := sender.msgs[1]
			if msg2.Type != "interactive" || msg2.Interactive == nil {
				t.Fatalf("msg[1] = %+v, want interactive button reply", msg2)
			}
			inter := msg2.Interactive
			if inter.Type != "button" {
				t.Fatalf("msg[1].Interactive = %+v, want button", msg2.Interactive)
			}
			if inter.Action == nil || len(inter.Action.Buttons) != 2 {
				t.Fatalf("buttons = %+v, want 2 buttons", inter.Action)
			}
			if inter.Action.Buttons[0].Reply.Title != "Nuestros viajes" {
				t.Errorf("button[0].Title = %q, want 'Nuestros viajes'", inter.Action.Buttons[0].Reply.Title)
			}
			if inter.Action.Buttons[1].Reply.Title != "Contacto" {
				t.Errorf("button[1].Title = %q, want 'Contacto'", inter.Action.Buttons[1].Reply.Title)
			}
		})
	}
}

func TestGreetingKeywordsFallbackWithoutProfileName(t *testing.T) {
	sender := &recordSender{}
	h := NewHandler()

	msg := bot.Message{
		ID:   "wamid.1",
		From: "15551234567",
		Type: bot.MessageTypeText,
		Text: "hola",
	}
	c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)
	c.Contact = bot.Contact{
		WaID:    "15551234567",
		Profile: bot.Profile{Name: ""}, // Empty profile name
	}

	if err := h(c); err != nil {
		t.Fatalf("h(c) error = %v", err)
	}

	if len(sender.msgs) != 2 {
		t.Fatalf("got %d messages sent, want 2", len(sender.msgs))
	}

	msg1 := sender.msgs[0]
	if msg1.Type != "text" || msg1.Text == nil {
		t.Fatalf("msg[0] = %+v, want text message", msg1)
	}
	if strings.Contains(msg1.Text.Body, "¡Hola, !") || strings.Contains(msg1.Text.Body, "%s") {
		t.Errorf("msg[0].Text.Body = %q, leaked format or empty name placeholder", msg1.Text.Body)
	}
	if !strings.Contains(strings.ToLower(msg1.Text.Body), "hola") {
		t.Errorf("msg[0].Text.Body = %q, want greeting text", msg1.Text.Body)
	}

	msg2 := sender.msgs[1]
	if msg2.Type != "interactive" {
		t.Fatalf("msg[1].Type = %q, want interactive", msg2.Type)
	}
}

func TestNonGreetingFreeTextTriggersMainMenuOnly(t *testing.T) {
	nonGreetings := []string{
		"quiero viajar a Cancun",
		"ayuda por favor",
		"random message",
		"12345",
	}

	for _, text := range nonGreetings {
		t.Run(text, func(t *testing.T) {
			sender := &recordSender{}
			h := NewHandler()

			msg := bot.Message{
				ID:   "wamid.non_greeting",
				From: "15551234567",
				Type: bot.MessageTypeText,
				Text: text,
			}
			c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)
			c.Contact = bot.Contact{
				WaID:    "15551234567",
				Profile: bot.Profile{Name: "Santiago"},
			}

			if err := h(c); err != nil {
				t.Fatalf("h(c) error = %v", err)
			}

			if len(sender.msgs) != 1 {
				t.Fatalf("got %d messages sent, want exactly 1 (Main Menu Button Reply)", len(sender.msgs))
			}

			sent := sender.msgs[0]
			if sent.Type != "interactive" || sent.Interactive == nil {
				t.Fatalf("sent = %+v, want interactive Button Reply", sent)
			}
			inter := sent.Interactive
			if inter.Type != "button" {
				t.Fatalf("Interactive = %+v, want button", sent.Interactive)
			}
			if inter.Action == nil || len(inter.Action.Buttons) != 2 {
				t.Fatalf("buttons = %+v, want 2 buttons", inter.Action)
			}
			if inter.Action.Buttons[0].Reply.Title != "Nuestros viajes" || inter.Action.Buttons[1].Reply.Title != "Contacto" {
				t.Errorf("buttons = %+v, want 'Nuestros viajes' and 'Contacto'", inter.Action.Buttons)
			}
		})
	}
}

func TestReactionAndSystemMessagesIgnored(t *testing.T) {
	for _, msgType := range []bot.MessageType{bot.MessageTypeReaction, bot.MessageTypeSystem} {
		t.Run(string(msgType), func(t *testing.T) {
			sender := &recordSender{}
			h := NewHandler()

			msg := bot.Message{
				ID:   "wamid.ignored",
				From: "15551234567",
				Type: msgType,
			}
			c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

			if err := h(c); err != nil {
				t.Fatalf("h(c) error = %v", err)
			}

			if len(sender.msgs) != 0 {
				t.Fatalf("got %d messages, want 0 for %s", len(sender.msgs), msgType)
			}
		})
	}
}

func TestUnsupportedMessageTypes(t *testing.T) {
	unsupportedTypes := []bot.MessageType{
		bot.MessageTypeAudio,
		bot.MessageTypeVideo,
		bot.MessageTypeSticker,
		bot.MessageTypeLocation,
		bot.MessageTypeDocument,
		bot.MessageTypeUnsupported,
	}

	for _, msgType := range unsupportedTypes {
		t.Run(string(msgType), func(t *testing.T) {
			sender := &recordSender{}
			h := NewHandler()

			msg := bot.Message{
				ID:   "wamid.unsupported_" + string(msgType),
				From: "15551234567",
				Type: msgType,
			}
			c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

			if err := h(c); err != nil {
				t.Fatalf("h(c) error = %v", err)
			}

			if len(sender.msgs) != 2 {
				t.Fatalf("got %d messages, want 2 (reaction + reply)", len(sender.msgs))
			}

			// First message must be reaction with emoji 👀
			m1 := sender.msgs[0]
			if m1.Type != "reaction" || m1.Reaction == nil {
				t.Fatalf("msg[0] = %+v, want reaction", m1)
			}
			if m1.Reaction.Emoji != "👀" {
				t.Errorf("reaction emoji = %q, want '👀'", m1.Reaction.Emoji)
			}
			if m1.Reaction.MessageID != msg.ID {
				t.Errorf("reaction message_id = %q, want %q", m1.Reaction.MessageID, msg.ID)
			}

			// Second message must be text reply
			m2 := sender.msgs[1]
			if m2.Type != "text" || m2.Text == nil || m2.Text.Body == "" {
				t.Errorf("msg[1] = %+v, want non-empty text reply", m2)
			}
		})
	}
}
