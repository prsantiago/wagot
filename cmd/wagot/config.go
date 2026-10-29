package main

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/chachin/wagot/internal/cloudapi"
)

// Environment variables read by the server. Each can be set in the real
// environment or in the .env file; the real environment wins.
const (
	envPort          = "PORT"
	envAPIVersion    = "WHATSAPP_API_VERSION"
	envPhoneNumberID = "WHATSAPP_PHONE_NUMBER_ID"
	envAccessToken   = "WHATSAPP_ACCESS_TOKEN"
	envVerifyToken   = "WHATSAPP_VERIFY_TOKEN"
	envAppSecret     = "WHATSAPP_APP_SECRET"
)

const defaultPort = "8080"

var (
	apiVersionRE = regexp.MustCompile(`^v\d+\.\d+$`)
	digitsRE     = regexp.MustCompile(`^\d+$`)
)

type config struct {
	addr          string
	apiVersion    string
	phoneNumberID string
	accessToken   string
	verifyToken   string
	appSecret     string
}

// loadConfig reads and validates the configuration, reporting every problem
// at once.
func loadConfig(getenv func(string) string) (config, error) {
	var errs []error
	required := func(key string) string {
		v := getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	withDefault := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}

	port := withDefault(envPort, defaultPort)
	cfg := config{
		addr:          net.JoinHostPort("", port),
		apiVersion:    withDefault(envAPIVersion, cloudapi.DefaultAPIVersion),
		phoneNumberID: required(envPhoneNumberID),
		accessToken:   required(envAccessToken),
		verifyToken:   required(envVerifyToken),
		appSecret:     required(envAppSecret),
	}

	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		errs = append(errs, fmt.Errorf("%s must be a port number, got %q", envPort, port))
	}
	if !apiVersionRE.MatchString(cfg.apiVersion) {
		errs = append(errs, fmt.Errorf("%s must look like v26.0, got %q", envAPIVersion, cfg.apiVersion))
	}
	if cfg.phoneNumberID != "" && !digitsRE.MatchString(cfg.phoneNumberID) {
		errs = append(errs, fmt.Errorf("%s must be the numeric ID from the Meta dashboard, not the phone number, got %q",
			envPhoneNumberID, cfg.phoneNumberID))
	}
	return cfg, errors.Join(errs...)
}
