package bot

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestMessageTypeIsDefinedDomainType(t *testing.T) {
	msg := Message{Type: MessageTypeText}
	typ := reflect.TypeOf(msg.Type)
	if typ.PkgPath() != "github.com/chachin/wagot/internal/bot" || typ.Name() != "MessageType" {
		t.Fatalf("Message.Type is %s (pkg: %q, name: %q), want defined domain type bot.MessageType", typ, typ.PkgPath(), typ.Name())
	}
}

func TestContextDoesNotExposeWireJSONTags(t *testing.T) {
	typesToCheck := []any{Context{}, Message{}, Contact{}, Profile{}, Text{}, Buttons{}, Button{}, List{}, ListItem{}, Image{}}
	for _, target := range typesToCheck {
		typ := reflect.TypeOf(target)
		for i := range typ.NumField() {
			field := typ.Field(i)
			if tag := field.Tag.Get("json"); tag != "" {
				t.Errorf("%s field %q exposes json tag %q", typ.Name(), field.Name, tag)
			}
			fieldType := field.Type
			if fieldType.Kind() == reflect.Struct && fieldType.PkgPath() == "github.com/chachin/wagot/internal/bot" {
				for j := range fieldType.NumField() {
					subField := fieldType.Field(j)
					if subTag := subField.Tag.Get("json"); subTag != "" {
						t.Errorf("%s field %q (type %s) leaks wire JSON tag on %q", typ.Name(), field.Name, fieldType.Name(), subField.Name)
					}
				}
			}
		}
	}
}

func TestContextSendText(t *testing.T) {
	sender := &fakeSender{}
	msg := Message{ID: "wamid.123", From: "15551234567", Type: MessageTypeText, Text: "hi"}
	c := NewContext(context.Background(), sender, "PNID_TEST", msg)

	err := c.Send(Text{Body: "https://wagot.example/preview", PreviewURL: true})
	if err != nil {
		t.Fatalf("c.Send(Text) error = %v", err)
	}

	if len(sender.msgs) != 1 {
		t.Fatalf("len(sender.msgs) = %d, want 1", len(sender.msgs))
	}
	sentMsg := sender.msgs[0]
	if sentMsg.Type != "text" || sentMsg.Text == nil {
		t.Fatalf("unexpected sentMsg = %+v", sentMsg)
	}
	if sentMsg.Text.Body != "https://wagot.example/preview" || !sentMsg.Text.PreviewURL {
		t.Errorf("sentMsg.Text = %+v, want body and PreviewURL=true", sentMsg.Text)
	}
}

func TestContextSendButtons(t *testing.T) {
	sender := &fakeSender{}
	msg := Message{ID: "wamid.123", From: "15551234567", Type: MessageTypeText, Text: "menu"}
	c := NewContext(context.Background(), sender, "PNID_TEST", msg)

	buttons := []Button{
		{ID: "btn_trips", Title: "Nuestros viajes"},
		{ID: "btn_contact", Title: "Contacto"},
	}
	err := c.Send(Buttons{Body: "¿Cómo podemos ayudarte?", Buttons: buttons})
	if err != nil {
		t.Fatalf("c.Send(Buttons) error = %v", err)
	}

	if len(sender.msgs) != 1 {
		t.Fatalf("len(sender.msgs) = %d, want 1", len(sender.msgs))
	}
	sentMsg := sender.msgs[0]
	if sentMsg.Type != "interactive" {
		t.Fatalf("sentMsg.Type = %q, want interactive", sentMsg.Type)
	}
	inter := sentMsg.Interactive
	if inter == nil || inter.Type != "button" {
		t.Fatalf("Interactive = %+v, want button", sentMsg.Interactive)
	}
	if inter.Body == nil || inter.Body.Text != "¿Cómo podemos ayudarte?" {
		t.Errorf("inter.Body = %+v", inter.Body)
	}
	if inter.Action == nil || len(inter.Action.Buttons) != 2 {
		t.Fatalf("inter.Action.Buttons = %+v, want 2 buttons", inter.Action)
	}
	if inter.Action.Buttons[0].Reply.ID != "btn_trips" || inter.Action.Buttons[0].Reply.Title != "Nuestros viajes" {
		t.Errorf("button 0 = %+v", inter.Action.Buttons[0])
	}
	if inter.Action.Buttons[1].Reply.ID != "btn_contact" || inter.Action.Buttons[1].Reply.Title != "Contacto" {
		t.Errorf("button 1 = %+v", inter.Action.Buttons[1])
	}
}

func TestContextSendList(t *testing.T) {
	sender := &fakeSender{}
	msg := Message{ID: "wamid.123", From: "15551234567", Type: MessageTypeInteractive, SelectionID: "trips"}
	c := NewContext(context.Background(), sender, "PNID_TEST", msg)

	items := []ListItem{
		{ID: "trip_1", Title: "Cancun", Description: "Playas"},
		{ID: "trip_2", Title: "Machu Picchu", Description: "Montañas"},
	}
	err := c.Send(List{Body: "Elige tu viaje:", Button: "Ver viajes", Items: items})
	if err != nil {
		t.Fatalf("c.Send(List) error = %v", err)
	}

	if len(sender.msgs) != 1 {
		t.Fatalf("len(sender.msgs) = %d, want 1", len(sender.msgs))
	}
	sentMsg := sender.msgs[0]
	if sentMsg.Type != "interactive" {
		t.Fatalf("sentMsg.Type = %q, want interactive", sentMsg.Type)
	}
	inter := sentMsg.Interactive
	if inter == nil || inter.Type != "list" {
		t.Fatalf("Interactive = %+v, want list", sentMsg.Interactive)
	}
	if inter.Body == nil || inter.Body.Text != "Elige tu viaje:" {
		t.Errorf("inter.Body = %+v", inter.Body)
	}
	if inter.Action == nil || inter.Action.Button != "Ver viajes" {
		t.Errorf("inter.Action.Button = %q, want 'Ver viajes'", inter.Action.Button)
	}
	if len(inter.Action.Sections) != 1 || len(inter.Action.Sections[0].Rows) != 2 {
		t.Fatalf("inter.Action.Sections = %+v, want 1 section with 2 rows", inter.Action.Sections)
	}
	row0 := inter.Action.Sections[0].Rows[0]
	if row0.ID != "trip_1" || row0.Title != "Cancun" || row0.Description != "Playas" {
		t.Errorf("row0 = %+v", row0)
	}
	row1 := inter.Action.Sections[0].Rows[1]
	if row1.ID != "trip_2" || row1.Title != "Machu Picchu" || row1.Description != "Montañas" {
		t.Errorf("row1 = %+v", row1)
	}
}

func TestContextSendImage(t *testing.T) {
	sender := &fakeSender{}
	msg := Message{ID: "wamid.123", From: "15551234567", Type: MessageTypeInteractive, SelectionID: "trip_cancun"}
	c := NewContext(context.Background(), sender, "PNID_TEST", msg)

	err := c.Send(Image{URL: "https://example.com/cancun.jpg", Caption: "Cancun package"})
	if err != nil {
		t.Fatalf("c.Send(Image) error = %v", err)
	}

	if len(sender.msgs) != 1 {
		t.Fatalf("len(sender.msgs) = %d, want 1", len(sender.msgs))
	}
	sentMsg := sender.msgs[0]
	if sentMsg.Type != "image" || sentMsg.Image == nil {
		t.Fatalf("sentMsg = %+v, want image message", sentMsg)
	}
	if sentMsg.Image.Link != "https://example.com/cancun.jpg" || sentMsg.Image.Caption != "Cancun package" {
		t.Errorf("sentMsg.Image = %+v", sentMsg.Image)
	}
}

func TestContextSendValidation(t *testing.T) {
	sender := &fakeSender{}
	c := NewContext(context.Background(), sender, "PNID_TEST", Message{ID: "wamid.1", From: "15551234567"})

	tests := map[string]struct {
		reply   Reply
		wantErr bool
	}{
		"nil reply": {
			reply:   nil,
			wantErr: true,
		},
		"empty text": {
			reply:   Text{Body: ""},
			wantErr: true,
		},
		"empty buttons body": {
			reply:   Buttons{Body: "", Buttons: []Button{{ID: "1", Title: "A"}}},
			wantErr: true,
		},
		"no buttons": {
			reply:   Buttons{Body: "select", Buttons: nil},
			wantErr: true,
		},
		"too many buttons": {
			reply: Buttons{Body: "select", Buttons: []Button{
				{ID: "1", Title: "1"}, {ID: "2", Title: "2"}, {ID: "3", Title: "3"}, {ID: "4", Title: "4"},
			}},
			wantErr: true,
		},
		"empty button ID": {
			reply:   Buttons{Body: "select", Buttons: []Button{{ID: "", Title: "A"}}},
			wantErr: true,
		},
		"empty button Title": {
			reply:   Buttons{Body: "select", Buttons: []Button{{ID: "1", Title: ""}}},
			wantErr: true,
		},
		"empty list body": {
			reply:   List{Body: "", Button: "btn", Items: []ListItem{{ID: "1", Title: "A"}}},
			wantErr: true,
		},
		"empty list button": {
			reply:   List{Body: "list", Button: "", Items: []ListItem{{ID: "1", Title: "A"}}},
			wantErr: true,
		},
		"no list items": {
			reply:   List{Body: "list", Button: "btn", Items: nil},
			wantErr: true,
		},
		"too many list items": {
			reply: List{Body: "list", Button: "btn", Items: []ListItem{
				{ID: "1", Title: "1"}, {ID: "2", Title: "2"}, {ID: "3", Title: "3"}, {ID: "4", Title: "4"},
				{ID: "5", Title: "5"}, {ID: "6", Title: "6"}, {ID: "7", Title: "7"}, {ID: "8", Title: "8"},
				{ID: "9", Title: "9"}, {ID: "10", Title: "10"}, {ID: "11", Title: "11"},
			}},
			wantErr: true,
		},
		"empty list item ID": {
			reply:   List{Body: "list", Button: "btn", Items: []ListItem{{ID: "", Title: "A"}}},
			wantErr: true,
		},
		"empty list item Title": {
			reply:   List{Body: "list", Button: "btn", Items: []ListItem{{ID: "1", Title: ""}}},
			wantErr: true,
		},
		"empty image URL": {
			reply:   Image{URL: "", Caption: "caption"},
			wantErr: true,
		},
		"non-https image URL": {
			reply:   Image{URL: "http://example.com/pic.jpg", Caption: "caption"},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := c.Send(tc.reply)
			if (err != nil) != tc.wantErr {
				t.Errorf("c.Send() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

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
	if len(sender.msgs) != 1 || sender.msgs[0].Type != string(MessageTypeText) || sender.msgs[0].Text.Body != "Hello user" {
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
	if sender.msgs[0].Type != string(MessageTypeReaction) {
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
