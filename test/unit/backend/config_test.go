package backend_test

import (
	"testing"

	"test-auto-pro-v2/internal/config"
)

func TestServerAddressDefaultsToProjectPort(t *testing.T) {
	t.Setenv("TEST_AUTO_PRO_SERVER_ADDR", "")
	if got := config.ServerAddress(); got != "127.0.0.1:19080" {
		t.Fatalf("默认监听地址 = %q", got)
	}
}

func TestServerAddressCanBeOverridden(t *testing.T) {
	t.Setenv("TEST_AUTO_PRO_SERVER_ADDR", "127.0.0.1:29080")
	if got := config.ServerAddress(); got != "127.0.0.1:29080" {
		t.Fatalf("覆盖后的监听地址 = %q", got)
	}
}
