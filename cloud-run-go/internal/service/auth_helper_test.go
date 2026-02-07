package service

import (
	"testing"
	"time"
)

func TestTokenExpiredSoon(t *testing.T) {
	if !tokenExpiredSoon(time.Time{}) {
		t.Fatalf("expected zero time to be expired")
	}

	if !tokenExpiredSoon(time.Now().Add(-1 * time.Minute)) {
		t.Fatalf("expected past time to be expired")
	}

	// Expires in 30s -> should be treated as expired-soon (1m threshold).
	if !tokenExpiredSoon(time.Now().Add(30 * time.Second)) {
		t.Fatalf("expected near-future time to be expired-soon")
	}

	// Expires in 2m -> not expired-soon.
	if tokenExpiredSoon(time.Now().Add(2 * time.Minute)) {
		t.Fatalf("expected time sufficiently in future to not be expired-soon")
	}
}

