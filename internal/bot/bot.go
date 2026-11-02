// Package bot turns verified webhook notifications into replies sent through
// the Cloud API.
//
// Bot implements http.Handler. ServeHTTP handles the Meta verification handshake
// on GET and validates signatures, unmarshals payloads, and queues inbound
// messages on POST so the webhook is acknowledged immediately, as Meta
// requires. A pool of workers then marks each message as read, calls your
// [Handler], and sends its replies. Messages from the same user always go to
// the same worker, so each conversation is handled in order.
package bot

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/chachin/wagot/internal/cloudapi"
)

// ErrQueueFull is returned when a worker's queue is full.
var ErrQueueFull = errors.New("bot: queue full")

// ErrClosed is returned when the bot has been shut down.
var ErrClosed = errors.New("bot: shut down")

// Sender is the part of the Cloud API the bot needs. *cloudapi.Client
// implements it.
type Sender interface {
	Send(ctx context.Context, phoneNumberID, to string, msg cloudapi.Message) (*cloudapi.SendResponse, error)
	MarkRead(ctx context.Context, phoneNumberID, messageID string, typing bool) error
}

// Incoming is an inbound message together with what's needed to answer it.
type Incoming struct {
	// PhoneNumberID is the business phone number that received the message.
	// Replies are sent from it.
	PhoneNumberID string
	// Contact is the sender's profile. It may be the zero value.
	Contact Contact
	Message Message
}

// Handler decides how to answer an inbound message. The returned messages are
// sent, in order, to the user who sent it. Return no messages to stay silent.
type Handler func(ctx context.Context, in Incoming) ([]cloudapi.Message, error)

// Config configures a Bot. Zero values select the defaults.
type Config struct {
	// VerifyToken is the webhook verification token configured in the Meta App Dashboard.
	VerifyToken string
	// AppSecret is the Meta App Secret used to verify payload signatures.
	AppSecret string

	// PhoneNumberID restricts the bot to messages sent to this business phone
	// number; notifications for other numbers on the same WhatsApp Business
	// Account are ignored. Empty accepts every number.
	PhoneNumberID string

	Workers        int           // default 4
	QueueSize      int           // per worker; default 64
	HandlerTimeout time.Duration // per message, including API calls; default 60s
	DedupTTL       time.Duration // how long message IDs are remembered; default 24h
	Logger         *slog.Logger  // default discards logs
}

// Bot processes inbound messages. Create it with New.
type Bot struct {
	sender        Sender
	handler       Handler
	verifyToken   string
	appSecret     []byte
	phoneNumberID string
	timeout       time.Duration
	logger        *slog.Logger
	seen          *dedup

	mu     sync.RWMutex // guards closed and sends on queues
	closed bool
	queues []chan Incoming
	wg     sync.WaitGroup

	// ctx is cancelled if Shutdown gives up waiting, aborting in-flight work.
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a Bot and starts its workers. Call Shutdown to stop them.
func New(sender Sender, handler Handler, cfg Config) (*Bot, error) {
	switch {
	case sender == nil:
		return nil, errors.New("bot: sender is required")
	case handler == nil:
		return nil, errors.New("bot: handler is required")
	case cfg.VerifyToken == "":
		return nil, errors.New("bot: verify token is required")
	case cfg.AppSecret == "":
		return nil, errors.New("bot: app secret is required")
	}

	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 64
	}
	if cfg.HandlerTimeout <= 0 {
		cfg.HandlerTimeout = 60 * time.Second
	}
	if cfg.DedupTTL <= 0 {
		cfg.DedupTTL = 24 * time.Hour
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &Bot{
		sender:        sender,
		handler:       handler,
		verifyToken:   cfg.VerifyToken,
		appSecret:     []byte(cfg.AppSecret),
		phoneNumberID: cfg.PhoneNumberID,
		timeout:       cfg.HandlerTimeout,
		logger:        cfg.Logger,
		seen:          newDedup(cfg.DedupTTL),
		queues:        make([]chan Incoming, cfg.Workers),
		ctx:           ctx,
		cancel:        cancel,
	}
	for i := range b.queues {
		q := make(chan Incoming, cfg.QueueSize)
		b.queues[i] = q
		b.wg.Go(func() { b.work(q) })
	}
	return b, nil
}

// enqueue must be called with b.mu read-locked.
func (b *Bot) enqueue(in Incoming) error {
	if !b.seen.add(in.Message.ID) {
		b.logger.Debug("skipping duplicate message", "id", in.Message.ID)
		return nil
	}
	select {
	case b.queues[shard(in.Message.From, len(b.queues))] <- in:
		return nil
	default:
		b.seen.remove(in.Message.ID) // accept the redelivery
		return fmt.Errorf("%w: dropping message %s", ErrQueueFull, in.Message.ID)
	}
}

// Shutdown stops accepting messages and waits for queued ones to be handled.
// If ctx expires first, in-flight handlers are cancelled and ctx.Err() is
// returned. Call it after the HTTP server has stopped.
func (b *Bot) Shutdown(ctx context.Context) error {
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		for _, q := range b.queues {
			close(q)
		}
	}
	b.mu.Unlock()

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		b.cancel()
		return nil
	case <-ctx.Done():
		b.cancel()
		<-done
		return ctx.Err()
	}
}

func (b *Bot) work(q <-chan Incoming) {
	for in := range q {
		b.handle(in)
	}
}

func (b *Bot) handle(in Incoming) {
	ctx, cancel := context.WithTimeout(b.ctx, b.timeout)
	defer cancel()
	logger := b.logger.With("message_id", in.Message.ID, "from", in.Message.From)

	defer func() {
		if r := recover(); r != nil {
			logger.ErrorContext(ctx, "handler panicked", "panic", r)
		}
	}()

	// Mark read and show "typing…" while the handler works. Not fatal.
	if err := b.sender.MarkRead(ctx, in.PhoneNumberID, in.Message.ID, true); err != nil {
		logger.WarnContext(ctx, "mark read", "error", err)
	}

	replies, err := b.handler(ctx, in)
	if err != nil {
		logger.ErrorContext(ctx, "handler", "error", err)
		return
	}
	for _, r := range replies {
		resp, err := b.sender.Send(ctx, in.PhoneNumberID, in.Message.From, r)
		if err != nil {
			logger.ErrorContext(ctx, "send reply", "error", err)
			return
		}
		logger.InfoContext(ctx, "reply sent", "reply_id", resp.MessageID(), "type", r.Type)
	}
}

func contactFor(contacts []Contact, waID string) Contact {
	for _, c := range contacts {
		if c.WaID == waID {
			return c
		}
	}
	return Contact{}
}

func shard(key string, n int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % uint32(n))
}
