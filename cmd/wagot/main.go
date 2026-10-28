// Command wagot runs an HTTP server that receives WhatsApp Business Platform
// webhooks.
//
// Configuration is read from the environment:
//
//	WHATSAPP_VERIFY_TOKEN  token entered in the Meta App Dashboard (required)
//	WHATSAPP_APP_SECRET    Meta app secret used to verify signatures (required)
//	PORT                   port to listen on (default 8080)
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chachin/wagot/internal/webhook"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type config struct {
	addr        string
	verifyToken string
	appSecret   string
}

func loadConfig(getenv func(string) string) (config, error) {
	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	cfg := config{
		addr:        net.JoinHostPort("", port),
		verifyToken: getenv("WHATSAPP_VERIFY_TOKEN"),
		appSecret:   getenv("WHATSAPP_APP_SECRET"),
	}
	var errs []error
	if cfg.verifyToken == "" {
		errs = append(errs, errors.New("WHATSAPP_VERIFY_TOKEN is required"))
	}
	if cfg.appSecret == "" {
		errs = append(errs, errors.New("WHATSAPP_APP_SECRET is required"))
	}
	return cfg, errors.Join(errs...)
}

// run starts the server and blocks until ctx is cancelled or the server fails.
func run(ctx context.Context, getenv func(string) string, stderr io.Writer) error {
	logger := slog.New(slog.NewJSONHandler(stderr, nil))

	cfg, err := loadConfig(getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	wh, err := webhook.NewHandler(cfg.verifyToken, cfg.appSecret, logProcessor(logger), logger)
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
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
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

// logProcessor logs inbound messages and status updates. Replace it with
// your own business logic.
func logProcessor(logger *slog.Logger) webhook.Processor {
	return webhook.ProcessorFunc(func(ctx context.Context, n *webhook.Notification) error {
		for _, entry := range n.Entry {
			for _, change := range entry.Changes {
				if change.Field != webhook.FieldMessages {
					logger.InfoContext(ctx, "unhandled change", "field", change.Field, "waba_id", entry.ID)
					continue
				}
				v, err := change.Messages()
				if err != nil {
					// A malformed value won't fix itself on retry, so log and move on.
					logger.ErrorContext(ctx, "decode change", "error", err)
					continue
				}
				for _, m := range v.Messages {
					attrs := []any{
						"phone_number_id", v.Metadata.PhoneNumberID,
						"from", m.From,
						"id", m.ID,
						"type", m.Type,
					}
					if m.Text != nil {
						attrs = append(attrs, "text", m.Text.Body)
					}
					logger.InfoContext(ctx, "message received", attrs...)
				}
				for _, s := range v.Statuses {
					logger.InfoContext(ctx, "status update",
						"phone_number_id", v.Metadata.PhoneNumberID,
						"id", s.ID,
						"status", s.Status,
						"recipient_id", s.RecipientID,
					)
				}
			}
		}
		return nil
	})
}
