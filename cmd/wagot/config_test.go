package main

import (
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{
		envPhoneNumberID: "106540352242922",
		envAccessToken:   "EAAG...",
		envVerifyToken:   "my-verify-token",
		envAppSecret:     "app-secret",
	}
}

func lookup(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(lookup(validEnv()))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.addr != ":8080" {
		t.Errorf("addr = %q, want :8080", cfg.addr)
	}
	if cfg.apiVersion != "v26.0" {
		t.Errorf("apiVersion = %q, want v26.0", cfg.apiVersion)
	}
	if cfg.phoneNumberID != "106540352242922" {
		t.Errorf("phoneNumberID = %q", cfg.phoneNumberID)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	env := validEnv()
	env[envPort] = "3000"
	env[envAPIVersion] = "v25.0"

	cfg, err := loadConfig(lookup(env))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.addr != ":3000" || cfg.apiVersion != "v25.0" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := map[string]struct {
		key, value string
		wantErr    string
	}{
		"missing phone number id": {envPhoneNumberID, "", envPhoneNumberID + " is required"},
		"missing access token":    {envAccessToken, "", envAccessToken + " is required"},
		"missing verify token":    {envVerifyToken, "", envVerifyToken + " is required"},
		"missing app secret":      {envAppSecret, "", envAppSecret + " is required"},
		"phone number not id":     {envPhoneNumberID, "+1 555 0100", envPhoneNumberID + " must be"},
		"bad port":                {envPort, "http", envPort + " must be"},
		"port out of range":       {envPort, "70000", envPort + " must be"},
		"bad api version":         {envAPIVersion, "26", envAPIVersion + " must"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env[tc.key] = tc.value

			_, err := loadConfig(lookup(env))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadConfigReportsAllMissing(t *testing.T) {
	_, err := loadConfig(lookup(nil))
	if err == nil {
		t.Fatal("loadConfig succeeded, want error")
	}
	for _, key := range []string{envPhoneNumberID, envAccessToken, envVerifyToken, envAppSecret} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}
