package main

import (
	"context"
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
		msgType   bot.MessageType
		text      string
		wantTypes []string
		wantBody  string
		wantEmoji string
	}{
		"text message": {
			msgType:   bot.MessageTypeText,
			text:      "Hello wagot",
			wantTypes: []string{"text"},
			wantBody:  "You said: Hello wagot",
		},
		"interactive selection": {
			msgType:   bot.MessageTypeInteractive,
			text:      "Option A",
			wantTypes: []string{"text"},
			wantBody:  "You picked: Option A",
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
			wantBody:  "Sorry, I can only read text messages for now.",
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
				lastIdx := len(s.sent) - 1
				if s.sent[lastIdx].Text == nil || s.sent[lastIdx].Text.Body != tc.wantBody {
					t.Errorf("reply text = %v, want %q", s.sent[lastIdx].Text, tc.wantBody)
				}
			}
		})
	}
}
