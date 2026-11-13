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

func TestInteractiveTripsSelectionEmitsListReply(t *testing.T) {
	sender := &recordSender{}
	h := NewHandler()

	msg := bot.Message{
		ID:          "wamid.interactive_trips",
		From:        "15551234567",
		Type:        bot.MessageTypeInteractive,
		SelectionID: "trips",
		Text:        "Nuestros viajes",
	}
	c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

	if err := h(c); err != nil {
		t.Fatalf("h(c) error = %v", err)
	}

	if len(sender.msgs) != 1 {
		t.Fatalf("got %d messages sent, want exactly 1 (List Reply without Main Menu)", len(sender.msgs))
	}

	sent := sender.msgs[0]
	if sent.Type != "interactive" || sent.Interactive == nil {
		t.Fatalf("sent = %+v, want interactive List Reply", sent)
	}
	if sent.Interactive.Type != "list" {
		t.Fatalf("sent.Interactive.Type = %q, want 'list'", sent.Interactive.Type)
	}
	if sent.Interactive.Body == nil || sent.Interactive.Body.Text == "" {
		t.Errorf("sent.Interactive.Body = %+v, want non-empty body text", sent.Interactive.Body)
	}
	if sent.Interactive.Action == nil || sent.Interactive.Action.Button == "" {
		t.Errorf("sent.Interactive.Action.Button = %q, want non-empty button label", sent.Interactive.Action.Button)
	}
	if len(sent.Interactive.Action.Sections) == 0 || len(sent.Interactive.Action.Sections[0].Rows) == 0 {
		t.Fatalf("sent.Interactive.Action.Sections = %+v, want at least 1 section with rows", sent.Interactive.Action.Sections)
	}
}

func TestInteractiveDestinationSelectionEmitsImageReplyAndFollowUpMainMenu(t *testing.T) {
	destinations := []string{"trip_cancun", "trip_machu_picchu", "trip_buenos_aires"}

	for _, destID := range destinations {
		t.Run(destID, func(t *testing.T) {
			sender := &recordSender{}
			h := NewHandler()

			msg := bot.Message{
				ID:          "wamid.dest." + destID,
				From:        "15551234567",
				Type:        bot.MessageTypeInteractive,
				SelectionID: destID,
			}
			c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

			if err := h(c); err != nil {
				t.Fatalf("h(c) error = %v", err)
			}

			if len(sender.msgs) != 2 {
				t.Fatalf("got %d messages sent, want 2 (image reply + Main Menu)", len(sender.msgs))
			}

			// First message must be Image Reply with HTTPS link and non-empty caption
			msg1 := sender.msgs[0]
			if msg1.Type != "image" || msg1.Image == nil {
				t.Fatalf("msg[0] = %+v, want image message", msg1)
			}
			if !strings.HasPrefix(msg1.Image.Link, "https://") {
				t.Errorf("msg[0].Image.Link = %q, want public HTTPS URL", msg1.Image.Link)
			}
			if msg1.Image.Caption == "" {
				t.Errorf("msg[0].Image.Caption is empty, want descriptive caption")
			}

			// Second message must be Main Menu Button Reply
			assertMainMenuButtons(t, sender.msgs[1])
		})
	}
}

func TestInteractiveContactSelectionEmitsTextReplyWithPreviewAndFollowUpMainMenu(t *testing.T) {
	sender := &recordSender{}
	h := NewHandler()

	msg := bot.Message{
		ID:          "wamid.interactive_contact",
		From:        "15551234567",
		Type:        bot.MessageTypeInteractive,
		SelectionID: "contact",
		Text:        "Contacto",
	}
	c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

	if err := h(c); err != nil {
		t.Fatalf("h(c) error = %v", err)
	}

	if len(sender.msgs) != 2 {
		t.Fatalf("got %d messages sent, want 2 (contact text reply + Main Menu)", len(sender.msgs))
	}

	// First message must be text Reply with URL preview enabled and containing Google Maps link
	msg1 := sender.msgs[0]
	if msg1.Type != "text" || msg1.Text == nil {
		t.Fatalf("msg[0] = %+v, want text message", msg1)
	}
	if !msg1.Text.PreviewURL {
		t.Errorf("msg[0].Text.PreviewURL = false, want true")
	}
	if !strings.Contains(msg1.Text.Body, "maps.google.com") && !strings.Contains(msg1.Text.Body, "goo.gl/maps") {
		t.Errorf("msg[0].Text.Body = %q, want Google Maps URL", msg1.Text.Body)
	}

	// Second message must be Main Menu Button Reply
	assertMainMenuButtons(t, sender.msgs[1])
}

func TestUnknownSelectionIDEmitsMainMenuButtonReply(t *testing.T) {
	testCases := []struct {
		name    string
		msgType bot.MessageType
		selID   string
	}{
		{"interactive unknown id", bot.MessageTypeInteractive, "unknown_selection"},
		{"interactive outdated id", bot.MessageTypeInteractive, "outdated_promo_2024"},
		{"interactive empty id", bot.MessageTypeInteractive, ""},
		{"button unknown selection", bot.MessageTypeButton, "unknown_button"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sender := &recordSender{}
			h := NewHandler()

			msg := bot.Message{
				ID:          "wamid.unknown",
				From:        "15551234567",
				Type:        tc.msgType,
				SelectionID: tc.selID,
			}
			c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

			if err := h(c); err != nil {
				t.Fatalf("h(c) error = %v", err)
			}

			if len(sender.msgs) != 1 {
				t.Fatalf("got %d messages sent, want exactly 1 (Main Menu)", len(sender.msgs))
			}

			assertMainMenuButtons(t, sender.msgs[0])
		})
	}
}

func assertMainMenuButtons(t *testing.T, sent cloudapi.Message) {
	t.Helper()
	if sent.Type != "interactive" || sent.Interactive == nil || sent.Interactive.Type != "button" {
		t.Fatalf("sent = %+v, want interactive Main Menu button reply", sent)
	}
	if sent.Interactive.Action == nil || len(sent.Interactive.Action.Buttons) != 2 {
		t.Fatalf("buttons = %+v, want 2 buttons", sent.Interactive.Action)
	}
	if sent.Interactive.Action.Buttons[0].Reply.Title != "Nuestros viajes" || sent.Interactive.Action.Buttons[1].Reply.Title != "Contacto" {
		t.Errorf("buttons = %+v, want 'Nuestros viajes' and 'Contacto'", sent.Interactive.Action.Buttons)
	}
}

func TestWelcomeTemplateNamePlaceholder(t *testing.T) {
	t.Run("replaces {{name}} with contact profile name", func(t *testing.T) {
		sender := &recordSender{}
		fl := &Flow{
			Greetings:       []string{"hola"},
			WelcomeTemplate: "¡Hola {{name}}! Bienvenido a Wagot Viajes ✈️",
			WelcomeFallback: "¡Hola! Bienvenido a Wagot Viajes ✈️",
			MainMenu: bot.Buttons{
				Body: "¿En qué podemos ayudarte?",
				Buttons: []bot.Button{{ID: "b1", Title: "Opciones"}},
			},
		}

		msg := bot.Message{ID: "m1", From: "1555", Type: bot.MessageTypeText, Text: "hola"}
		c := bot.NewContext(context.Background(), sender, "PNID", msg)
		c.Contact = bot.Contact{Profile: bot.Profile{Name: "Santiago"}}

		if err := fl.Handle(c); err != nil {
			t.Fatalf("Handle() err = %v", err)
		}

		if len(sender.msgs) != 2 {
			t.Fatalf("got %d messages, want 2", len(sender.msgs))
		}
		want := "¡Hola Santiago! Bienvenido a Wagot Viajes ✈️"
		if got := sender.msgs[0].Text.Body; got != want {
			t.Errorf("got welcome text %q, want %q", got, want)
		}
	})

	t.Run("uses fallback when profile name is empty", func(t *testing.T) {
		sender := &recordSender{}
		fl := &Flow{
			Greetings:       []string{"hola"},
			WelcomeTemplate: "¡Hola {{name}}! Bienvenido a Wagot Viajes ✈️",
			WelcomeFallback: "¡Hola! Bienvenido a Wagot Viajes ✈️",
			MainMenu: bot.Buttons{
				Body: "¿En qué podemos ayudarte?",
				Buttons: []bot.Button{{ID: "b1", Title: "Opciones"}},
			},
		}

		msg := bot.Message{ID: "m1", From: "1555", Type: bot.MessageTypeText, Text: "hola"}
		c := bot.NewContext(context.Background(), sender, "PNID", msg)
		c.Contact = bot.Contact{Profile: bot.Profile{Name: ""}}

		if err := fl.Handle(c); err != nil {
			t.Fatalf("Handle() err = %v", err)
		}

		if len(sender.msgs) != 2 {
			t.Fatalf("got %d messages, want 2", len(sender.msgs))
		}
		want := "¡Hola! Bienvenido a Wagot Viajes ✈️"
		if got := sender.msgs[0].Text.Body; got != want {
			t.Errorf("got welcome text %q, want %q", got, want)
		}
	})

	t.Run("strips {{name}} gracefully when profile name is empty and no fallback", func(t *testing.T) {
		sender := &recordSender{}
		fl := &Flow{
			Greetings:       []string{"hola"},
			WelcomeTemplate: "¡Hola, {{name}}! Bienvenido a Wagot Viajes ✈️",
			MainMenu: bot.Buttons{
				Body: "¿En qué podemos ayudarte?",
				Buttons: []bot.Button{{ID: "b1", Title: "Opciones"}},
			},
		}

		msg := bot.Message{ID: "m1", From: "1555", Type: bot.MessageTypeText, Text: "hola"}
		c := bot.NewContext(context.Background(), sender, "PNID", msg)
		c.Contact = bot.Contact{Profile: bot.Profile{Name: ""}}

		if err := fl.Handle(c); err != nil {
			t.Fatalf("Handle() err = %v", err)
		}

		if len(sender.msgs) != 2 {
			t.Fatalf("got %d messages, want 2", len(sender.msgs))
		}
		want := "¡Hola! Bienvenido a Wagot Viajes ✈️"
		if got := sender.msgs[0].Text.Body; got != want {
			t.Errorf("got welcome text %q, want %q", got, want)
		}
	})

	t.Run("safely handles template with percent characters and {{name}}", func(t *testing.T) {
		sender := &recordSender{}
		fl := &Flow{
			Greetings:       []string{"hola"},
			WelcomeTemplate: "100% garantizado: ¡Hola {{name}}!",
			MainMenu: bot.Buttons{
				Body: "¿En qué podemos ayudarte?",
				Buttons: []bot.Button{{ID: "b1", Title: "Opciones"}},
			},
		}

		msg := bot.Message{ID: "m1", From: "1555", Type: bot.MessageTypeText, Text: "hola"}
		c := bot.NewContext(context.Background(), sender, "PNID", msg)
		c.Contact = bot.Contact{Profile: bot.Profile{Name: "Santiago"}}

		if err := fl.Handle(c); err != nil {
			t.Fatalf("Handle() err = %v", err)
		}

		want := "100% garantizado: ¡Hola Santiago!"
		if got := sender.msgs[0].Text.Body; got != want {
			t.Errorf("got welcome text %q, want %q", got, want)
		}
	})
}
