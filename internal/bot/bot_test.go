package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/chachin/wagot/internal/cloudapi"
	"github.com/chachin/wagot/internal/webhook"
)

type sent struct {
	phoneNumberID, to, body string
}

type fakeSender struct {
	mu   sync.Mutex
	read []string
	sent []sent
}

func (f *fakeSender) Send(_ context.Context, phoneNumberID, to string, msg cloudapi.Message) (*cloudapi.SendResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{phoneNumberID, to, msg.Text.Body})
	return &cloudapi.SendResponse{}, nil
}

func (f *fakeSender) MarkRead(_ context.Context, _, messageID string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.read = append(f.read, messageID)
	return nil
}

func echo(_ context.Context, in Incoming) ([]cloudapi.Message, error) {
	return []cloudapi.Message{cloudapi.TextMessage(in.Contact.Profile.Name + ": " + in.Message.Text.Body)}, nil
}

// notification builds a "messages" notification with one text message per
// (from, id, body) triple.
func notification(t *testing.T, msgs ...[3]string) *webhook.Notification {
	t.Helper()
	v := webhook.MessagesValue{
		MessagingProduct: "whatsapp",
		Metadata:         webhook.Metadata{PhoneNumberID: "PNID"},
	}
	for _, m := range msgs {
		v.Contacts = append(v.Contacts, webhook.Contact{WaID: m[0], Profile: webhook.Profile{Name: "name-" + m[0]}})
		v.Messages = append(v.Messages, webhook.Message{
			From: m[0], ID: m[1], Type: webhook.MessageTypeText, Text: &webhook.Text{Body: m[2]},
		})
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return &webhook.Notification{
		Object: webhook.ObjectWhatsAppBusinessAccount,
		Entry:  []webhook.Entry{{ID: "WABA", Changes: []webhook.Change{{Field: webhook.FieldMessages, Value: raw}}}},
	}
}

func shutdown(t *testing.T, b *Bot) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestRepliesToMessage(t *testing.T) {
	s := &fakeSender{}
	b := New(s, echo, Config{})

	if err := b.Process(context.Background(), notification(t, [3]string{"111", "wamid.1", "hola"})); err != nil {
		t.Fatalf("Process: %v", err)
	}
	shutdown(t, b)

	if want := []string{"wamid.1"}; !slices.Equal(s.read, want) {
		t.Errorf("read = %v, want %v", s.read, want)
	}
	if want := []sent{{"PNID", "111", "name-111: hola"}}; !slices.Equal(s.sent, want) {
		t.Errorf("sent = %v, want %v", s.sent, want)
	}
}

func TestIgnoresOtherPhoneNumbers(t *testing.T) {
	s := &fakeSender{}
	b := New(s, echo, Config{PhoneNumberID: "OTHER"})

	if err := b.Process(context.Background(), notification(t, [3]string{"111", "wamid.1", "hola"})); err != nil {
		t.Fatalf("Process: %v", err)
	}
	shutdown(t, b)

	if len(s.read) != 0 || len(s.sent) != 0 {
		t.Errorf("handled message for another number: read=%v sent=%v", s.read, s.sent)
	}
}

func TestIgnoresDuplicateDeliveries(t *testing.T) {
	s := &fakeSender{}
	b := New(s, echo, Config{})

	n := notification(t, [3]string{"111", "wamid.1", "hola"})
	for range 3 {
		if err := b.Process(context.Background(), n); err != nil {
			t.Fatalf("Process: %v", err)
		}
	}
	shutdown(t, b)

	if len(s.sent) != 1 {
		t.Errorf("sent %d replies, want 1", len(s.sent))
	}
}

func TestPreservesPerUserOrder(t *testing.T) {
	s := &fakeSender{}
	b := New(s, func(_ context.Context, in Incoming) ([]cloudapi.Message, error) {
		return []cloudapi.Message{cloudapi.TextMessage(in.Message.Text.Body)}, nil
	}, Config{Workers: 8})

	var want []string
	for i := range 50 {
		body := fmt.Sprint(i)
		want = append(want, body)
		if err := b.Process(context.Background(), notification(t, [3]string{"111", "wamid." + body, body})); err != nil {
			t.Fatalf("Process: %v", err)
		}
	}
	shutdown(t, b)

	var got []string
	for _, m := range s.sent {
		got = append(got, m.body)
	}
	if !slices.Equal(got, want) {
		t.Errorf("replies out of order: %v", got)
	}
}

func TestQueueFullRejectsAndAllowsRedelivery(t *testing.T) {
	release := make(chan struct{})
	s := &fakeSender{}
	b := New(s, func(ctx context.Context, in Incoming) ([]cloudapi.Message, error) {
		<-release
		return nil, nil
	}, Config{Workers: 1, QueueSize: 1})
	defer func() { close(release); shutdown(t, b) }()

	// The first message occupies the worker, the second fills the queue.
	// Wait until the worker has picked up the first so the queue is empty.
	_ = b.Process(context.Background(), notification(t, [3]string{"1", "a", "x"}))
	waitFor(t, func() bool { return len(b.queues[0]) == 0 })
	_ = b.Process(context.Background(), notification(t, [3]string{"1", "b", "x"}))

	n := notification(t, [3]string{"1", "c", "x"})
	if err := b.Process(context.Background(), n); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Process = %v, want ErrQueueFull", err)
	}
	if !b.seen.add("c") {
		t.Error("rejected message was remembered; redelivery would be dropped")
	}
}

func TestProcessAfterShutdown(t *testing.T) {
	b := New(&fakeSender{}, echo, Config{})
	shutdown(t, b)

	if err := b.Process(context.Background(), notification(t, [3]string{"1", "a", "x"})); !errors.Is(err, ErrClosed) {
		t.Fatalf("Process = %v, want ErrClosed", err)
	}
}

func TestShutdownTimeoutCancelsHandlers(t *testing.T) {
	b := New(&fakeSender{}, func(ctx context.Context, in Incoming) ([]cloudapi.Message, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}, Config{})
	_ = b.Process(context.Background(), notification(t, [3]string{"1", "a", "x"}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := b.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v, want DeadlineExceeded", err)
	}
}

func TestRecoversFromHandlerPanic(t *testing.T) {
	s := &fakeSender{}
	b := New(s, func(ctx context.Context, in Incoming) ([]cloudapi.Message, error) {
		if in.Message.ID == "boom" {
			panic("boom")
		}
		return echo(ctx, in)
	}, Config{Workers: 1})

	_ = b.Process(context.Background(), notification(t, [3]string{"1", "boom", "x"}))
	_ = b.Process(context.Background(), notification(t, [3]string{"1", "ok", "y"}))
	shutdown(t, b)

	if len(s.sent) != 1 {
		t.Errorf("sent %d replies, want 1", len(s.sent))
	}
}

func TestDedupExpires(t *testing.T) {
	now := time.Unix(0, 0)
	d := newDedup(time.Hour)
	d.now = func() time.Time { return now }

	if !d.add("x") || d.add("x") {
		t.Fatal("add should accept once, then reject")
	}
	now = now.Add(2 * time.Hour)
	if !d.add("x") {
		t.Error("add rejected id after TTL")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met")
		}
		time.Sleep(time.Millisecond)
	}
}
