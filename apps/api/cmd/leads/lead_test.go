package main

import (
	"errors"
	"fmt"
	"testing"
)

func valid() Lead {
	return Lead{
		Name: " Анна ", Clinic: "Клиника", Phone: "+7 900 000-00-00", Doctors: "6–20",
		Consent: true, ConsentVersion: "2026-11-09", PolicyVersion: "2026-11-09", Locale: "ru",
	}
}

func TestNormalize(t *testing.T) {
	l := valid()
	if err := l.normalize(); err != nil {
		t.Fatalf("valid lead rejected: %v", err)
	}
	if l.Name != "Анна" {
		t.Fatalf("name not trimmed: %q", l.Name)
	}

	cases := map[string]func(*Lead){
		"no consent":   func(l *Lead) { l.Consent = false },
		"short phone":  func(l *Lead) { l.Phone = "12345" },
		"letters":      func(l *Lead) { l.Phone = "+7 900 abc-00-00" },
		"bad email":    func(l *Lead) { l.Email = "nope" },
		"no name":      func(l *Lead) { l.Name = "  " },
		"bad version":  func(l *Lead) { l.ConsentVersion = "latest" },
		"bad locale":   func(l *Lead) { l.Locale = "de" },
		"long message": func(l *Lead) { l.Message = string(make([]rune, 2001)) },
	}
	for name, mutate := range cases {
		l := valid()
		mutate(&l)
		if err := l.normalize(); !errors.Is(err, errInvalid) {
			t.Errorf("%s: want errInvalid, got %v", name, err)
		}
	}
}

func TestLimiter(t *testing.T) {
	lim := newLimiter(2)
	if !lim.allow("1.1.1.1") || !lim.allow("1.1.1.1") {
		t.Fatal("first requests must pass")
	}
	if lim.allow("1.1.1.1") {
		t.Fatal("third request in window must be limited")
	}
	if !lim.allow("2.2.2.2") {
		t.Fatal("other IP must not be limited")
	}

	// IPv6 addresses in one /64 share a bucket.
	if !lim.allow("2001:db8::1") || !lim.allow("2001:db8::2") || lim.allow("2001:db8::ffff") {
		t.Fatal("same /64 must share the limit")
	}

	// Global cap holds across distinct clients.
	g := newLimiter(1000)
	for i := range globalPerHour {
		if !g.allow(fmt.Sprintf("10.0.%d.%d", i/250, i%250)) {
			t.Fatalf("request %d under global cap rejected", i)
		}
	}
	if g.allow("10.9.9.9") {
		t.Fatal("global cap must reject")
	}
}
