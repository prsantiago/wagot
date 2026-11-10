package main

import (
	"context"
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
				ID:   "wamid.incoming",
				From: "15551234567",
				Type: tc.msgType,
				Text: tc.text,
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
