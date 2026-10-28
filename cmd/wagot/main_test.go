package main

import "testing"

func TestLoadConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	t.Run("defaults port", func(t *testing.T) {
		cfg, err := loadConfig(env(map[string]string{
			"WHATSAPP_VERIFY_TOKEN": "t",
			"WHATSAPP_APP_SECRET":   "s",
		}))
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		if cfg.addr != ":8080" {
			t.Errorf("addr = %q, want :8080", cfg.addr)
		}
	})

	t.Run("custom port", func(t *testing.T) {
		cfg, err := loadConfig(env(map[string]string{
			"PORT":                  "3000",
			"WHATSAPP_VERIFY_TOKEN": "t",
			"WHATSAPP_APP_SECRET":   "s",
		}))
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		if cfg.addr != ":3000" {
			t.Errorf("addr = %q, want :3000", cfg.addr)
		}
	})

	t.Run("missing secrets", func(t *testing.T) {
		if _, err := loadConfig(env(nil)); err == nil {
			t.Fatal("loadConfig succeeded, want error")
		}
	})
}
