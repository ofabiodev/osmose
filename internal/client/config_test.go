package client

import (
	"strings"
	"testing"
)

func TestConfigRequiresCredentials(t *testing.T) {
	_, err := New(Config{})
	if err == nil || !strings.Contains(err.Error(), "token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigDefaults(t *testing.T) {
	config := (Config{Token: "token", ClientID: 1}).withDefaults()
	if config.ServerURL == "" || config.RequestTimeout <= 0 || config.EventQueue <= 0 || config.EventWorkers != 1 || config.DeviceType == "" {
		t.Fatalf("defaults were not applied: %#v", config)
	}
}
