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
//	FLOW_FILE                 path to custom flow JSON configuration file (optional)
//
// Usage:
//
//	wagot [-env-file path] [-flow-file path]
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
	"github.com/chachin/wagot/internal/dotenv"
	"github.com/chachin/wagot/internal/flow"
)

const shutdownTimeout = 30 * time.Second

func main() {
	envFile := flag.String("env-file", ".env", "`path` to a KEY=VALUE file; ignored if missing")
	flowFile := flag.String("flow-file", "", "`path` to custom JSON flow configuration file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fileVars, err := dotenv.Load(*envFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load env file:", err)
		os.Exit(1)
	}
	if err := run(ctx, getenv(fileVars), os.Stderr, *flowFile); err != nil {
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
func run(ctx context.Context, getenv func(string) string, stderr io.Writer, flowFlag string) error {
	logger := slog.New(slog.NewJSONHandler(stderr, nil))

	cfg, err := loadConfig(getenv, flowFlag)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}

	fl, err := loadFlow(cfg.flowFile)
	if err != nil {
		return fmt.Errorf("load flow: %w", err)
	}

	b, err := bot.New(nil, fl.Handle, bot.Config{
		PhoneNumberID: cfg.phoneNumberID,
		VerifyToken:   cfg.verifyToken,
		AppSecret:     cfg.appSecret,
		AccessToken:   cfg.accessToken,
		APIVersion:    cfg.apiVersion,
		Logger:        logger,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           newMux(b),
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

func newMux(b *bot.Bot) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/webhook", b)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func loadFlow(path string) (*flow.Flow, error) {
	if path == "" {
		return flow.DefaultFlow(), nil
	}
	return flow.LoadFile(path)
}
