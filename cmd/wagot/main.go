// Command wagot is a WhatsApp bot server. It receives WhatsApp Business
// Platform webhooks and replies through the Cloud API.
//
// Configuration comes from environment variables, which can also be set in a
// .env file (see .env.example). Variables in the real environment take
// precedence over the file.
//
//	PORT                      port to listen on (default 8080)
//	WHATSAPP_API_VERSION      Graph API version (default v26.0)
//	WHATSAPP_PHONE_NUMBER_ID  business phone number ID to answer from (required)
//	WHATSAPP_ACCESS_TOKEN     token with whatsapp_business_messaging (required)
//	WHATSAPP_VERIFY_TOKEN     webhook verify token you choose in the dashboard (required)
//	WHATSAPP_APP_SECRET       Meta app secret used to verify signatures (required)
//
// Usage:
//
//	wagot [-env-file path]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chachin/wagot/internal/bot"
	"github.com/chachin/wagot/internal/cloudapi"
	"github.com/chachin/wagot/internal/dotenv"
	"github.com/chachin/wagot/internal/webhook"
)

const shutdownTimeout = 30 * time.Second

func main() {
	envFile := flag.String("env-file", ".env", "`path` to a KEY=VALUE file; ignored if missing")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fileVars, err := dotenv.Load(*envFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load env file:", err)
		os.Exit(1)
	}
	if err := run(ctx, getenv(fileVars), os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getenv looks a key up in the real environment first, then in fileVars.
func getenv(fileVars map[string]string) func(string) string {
	return func(key string) string {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		return fileVars[key]
	}
}

// run starts the server and blocks until ctx is cancelled or the server fails.
func run(ctx context.Context, getenv func(string) string, stderr io.Writer) error {
	logger := slog.New(slog.NewJSONHandler(stderr, nil))

	cfg, err := loadConfig(getenv)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}

	client := cloudapi.NewClient(cfg.accessToken, cfg.apiVersion)
	b := bot.New(client, reply, bot.Config{
		PhoneNumberID: cfg.phoneNumberID,
		Logger:        logger,
	})

	wh, err := webhook.NewHandler(cfg.verifyToken, cfg.appSecret, b, logger)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           newMux(wh),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", cfg.addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err = <-serveErr:
		err = fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	// Stop accepting webhooks first, then let the bot finish queued messages.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if serr := srv.Shutdown(shutdownCtx); serr != nil {
		err = errors.Join(err, fmt.Errorf("shutdown server: %w", serr))
	}
	if berr := b.Shutdown(shutdownCtx); berr != nil {
		err = errors.Join(err, fmt.Errorf("shutdown bot: %w", berr))
	}
	return err
}

func newMux(wh *webhook.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /webhook", wh.Verify)
	mux.HandleFunc("POST /webhook", wh.Receive)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// reply is the bot's conversation logic. It echoes text messages; replace it
// with your own (call an LLM, look up an order, etc.).
func reply(_ context.Context, in bot.Incoming) ([]cloudapi.Message, error) {
	m := in.Message
	switch m.Type {
	case webhook.MessageTypeText:
		return []cloudapi.Message{cloudapi.TextMessage("You said: " + m.Text.Body)}, nil
	case webhook.MessageTypeInteractive:
		if m.Interactive.ButtonReply != nil {
			return []cloudapi.Message{cloudapi.TextMessage("You picked: " + m.Interactive.ButtonReply.Title)}, nil
		}
		if m.Interactive.ListReply != nil {
			return []cloudapi.Message{cloudapi.TextMessage("You picked: " + m.Interactive.ListReply.Title)}, nil
		}
	case webhook.MessageTypeReaction, webhook.MessageTypeSystem:
		return nil, nil // nothing to answer
	}
	return []cloudapi.Message{
		cloudapi.ReactionMessage(m.ID, "👀"),
		cloudapi.TextMessage("Sorry, I can only read text messages for now."),
	}, nil
}
