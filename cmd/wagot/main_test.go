package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chachin/wagot/internal/bot"
	"github.com/chachin/wagot/internal/cloudapi"
)

type fakeSender struct {
	sent []cloudapi.Message
}

func (f *fakeSender) Send(ctx context.Context, phoneNumberID, to string, msg cloudapi.Message) (*cloudapi.SendResponse, error) {
	f.sent = append(f.sent, msg)
	return &cloudapi.SendResponse{}, nil
}

func (f *fakeSender) MarkRead(ctx context.Context, phoneNumberID, messageID string, typing bool) error {
	return nil
}

func TestConverse(t *testing.T) {
	tests := map[string]struct {
		msgType     bot.MessageType
		text        string
		selectionID string
		profileName string
		wantTypes   []string
		wantBody    string
		wantEmoji   string
	}{
		"greeting keyword triggers personalized welcome and main menu": {
			msgType:     bot.MessageTypeText,
			text:        "¡Hola!",
			profileName: "Santiago",
			wantTypes:   []string{"text", "interactive"},
			wantBody:    "Santiago",
		},
		"greeting keyword without profile name triggers fallback welcome and main menu": {
			msgType:     bot.MessageTypeText,
			text:        "menu",
			profileName: "",
			wantTypes:   []string{"text", "interactive"},
			wantBody:    "Wagot Viajes",
		},
		"non-greeting free text triggers only main menu button reply": {
			msgType:     bot.MessageTypeText,
			text:        "Hello wagot",
			profileName: "Santiago",
			wantTypes:   []string{"interactive"},
		},
		"interactive selection trips emits list reply": {
			msgType:     bot.MessageTypeInteractive,
			selectionID: "trips",
			text:        "Nuestros viajes",
			wantTypes:   []string{"interactive"},
		},
		"interactive selection destination emits image and main menu": {
			msgType:     bot.MessageTypeInteractive,
			selectionID: "trip_cancun",
			text:        "Cancún, México",
			wantTypes:   []string{"image", "interactive"},
		},
		"interactive selection contact emits text with preview and main menu": {
			msgType:     bot.MessageTypeInteractive,
			selectionID: "contact",
			text:        "Contacto",
			wantTypes:   []string{"text", "interactive"},
			wantBody:    "maps.google.com",
		},
		"interactive unknown selection emits main menu button reply": {
			msgType:     bot.MessageTypeInteractive,
			selectionID: "unknown_id",
			text:        "",
			wantTypes:   []string{"interactive"},
		},
		"reaction is ignored": {
			msgType:   bot.MessageTypeReaction,
			text:      "",
			wantTypes: nil,
		},
		"system message is ignored": {
			msgType:   bot.MessageTypeSystem,
			text:      "",
			wantTypes: nil,
		},
		"unsupported message triggers reaction and reply": {
			msgType:   bot.MessageTypeUnsupported,
			text:      "",
			wantTypes: []string{"reaction", "text"},
			wantEmoji: "👀",
			wantBody:  "Por el momento solo puedo leer mensajes de texto.",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := &fakeSender{}
			msg := bot.Message{
				ID:          "wamid.incoming",
				From:        "15551234567",
				Type:        tc.msgType,
				Text:        tc.text,
				SelectionID: tc.selectionID,
			}
			c := bot.NewContext(context.Background(), s, "106540352242922", msg)
			c.Contact = bot.Contact{
				WaID:    "15551234567",
				Profile: bot.Profile{Name: tc.profileName},
			}

			if err := converse(c); err != nil {
				t.Fatalf("converse() unexpected error: %v", err)
			}

			if len(tc.wantTypes) == 0 {
				if len(s.sent) != 0 {
					t.Fatalf("sent %d messages, want 0", len(s.sent))
				}
				return
			}

			if len(s.sent) != len(tc.wantTypes) {
				t.Fatalf("sent %d messages, want %d", len(s.sent), len(tc.wantTypes))
			}

			for i, wantType := range tc.wantTypes {
				if s.sent[i].Type != wantType {
					t.Errorf("msg[%d] type = %q, want %q", i, s.sent[i].Type, wantType)
				}
			}

			if tc.wantEmoji != "" {
				if s.sent[0].Reaction == nil || s.sent[0].Reaction.Emoji != tc.wantEmoji {
					t.Errorf("reaction emoji = %v, want %q", s.sent[0].Reaction, tc.wantEmoji)
				}
			}

			if tc.wantBody != "" {
				// For text reply, check that body contains wantBody
				var textBody string
				for _, m := range s.sent {
					if m.Type == "text" && m.Text != nil {
						textBody = m.Text.Body
						break
					}
				}
				if !strings.Contains(textBody, tc.wantBody) {
					t.Errorf("reply text %q does not contain %q", textBody, tc.wantBody)
				}
			}
		})
	}
}

func TestCustomFlowFileEndToEnd(t *testing.T) {
	flowJSON := `{
		"greetings": ["hola", "aloha"],
		"welcome_template": "¡Aloha, %s! Bienvenido a Surf & Sun 🏄",
		"welcome_fallback": "¡Aloha! Bienvenido a Surf & Sun 🏄",
		"main_menu": {
			"body": "¿Qué aventura buscas hoy?",
			"buttons": [
				{"id": "lessons", "title": "Clases de surf"},
				{"id": "rentals", "title": "Alquiler tablas"}
			]
		}
	}`

	tmpDir := t.TempDir()
	flowPath := filepath.Join(tmpDir, "surf_flow.json")
	if err := os.WriteFile(flowPath, []byte(flowJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fl, err := loadFlow(flowPath)
	if err != nil {
		t.Fatalf("loadFlow(%q): %v", flowPath, err)
	}

	sender := &fakeSender{}
	msg := bot.Message{
		ID:   "wamid.surf_greet",
		From: "15551234567",
		Type: bot.MessageTypeText,
		Text: "aloha",
	}
	c := bot.NewContext(context.Background(), sender, "106540352242922", msg)
	c.Contact = bot.Contact{
		WaID:    "15551234567",
		Profile: bot.Profile{Name: "Keahi"},
	}

	if err := fl.Handle(c); err != nil {
		t.Fatalf("fl.Handle: %v", err)
	}

	if len(sender.sent) != 2 {
		t.Fatalf("got %d messages, want 2", len(sender.sent))
	}
	if sender.sent[0].Type != "text" || !strings.Contains(sender.sent[0].Text.Body, "Surf & Sun") {
		t.Errorf("msg[0] = %+v, want Surf & Sun welcome", sender.sent[0])
	}
	if sender.sent[1].Type != "interactive" || sender.sent[1].Interactive == nil {
		t.Fatalf("msg[1] = %+v, want interactive main menu", sender.sent[1])
	}
	if sender.sent[1].Interactive.Body.Text != "¿Qué aventura buscas hoy?" {
		t.Errorf("msg[1] body = %q", sender.sent[1].Interactive.Body.Text)
	}
}

func TestRunFailsFastOnInvalidFlowFile(t *testing.T) {
	env := validEnv()
	env[envFlowFile] = "non_existent_flow_file.json"

	err := run(context.Background(), lookup(env), io.Discard, "")
	if err == nil || !strings.Contains(err.Error(), "load flow") {
		t.Fatalf("run() err = %v, want error containing 'load flow'", err)
	}
}
