package domain

import (
	"testing"
	"time"
)

func TestNotificationEligibility(t *testing.T) {
	p := DefaultNotificationPreferences()
	day := time.Date(2026, 11, 9, 4, 0, 0, 0, time.UTC)
	if p.Eligible(day, 0) {
		t.Fatal("default must not push")
	}
	p.OptIn = true
	if !p.Eligible(day, 0) {
		t.Fatal("authorized daytime blocked")
	}
	if p.Eligible(day, 3) {
		t.Fatal("daily quota exceeded")
	}
	if p.Eligible(time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC), 0) {
		t.Fatal("quiet hours ignored")
	}
	p.NoContact = true
	if p.Eligible(day, 0) {
		t.Fatal("no-contact ignored")
	}
	p.NoContact = false
	p.QuietStart = 8
	p.QuietEnd = 18
	if p.Eligible(day, 0) {
		t.Fatal("daytime quiet hours ignored")
	}
}
