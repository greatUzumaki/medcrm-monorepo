package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Lead is the landing demo request. It holds personal data (name, phone, email):
// never log it, never send it to services outside Russia.
type Lead struct {
	Name           string `json:"name"`
	Clinic         string `json:"clinic"`
	Phone          string `json:"phone"`
	Email          string `json:"email,omitempty"`
	Doctors        string `json:"doctors"`
	Message        string `json:"message,omitempty"`
	Consent        bool   `json:"consent"`
	ConsentVersion string `json:"consentVersion"`
	PolicyVersion  string `json:"policyVersion"`
	Locale         string `json:"locale"`
	Source         string `json:"source"`
}

var (
	phoneRe   = regexp.MustCompile(`^\+?[\d\s()-]+$`)
	emailRe   = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	versionRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	locales   = map[string]bool{"ru": true, "en": true, "zh": true, "es": true}
)

var errInvalid = errors.New("invalid lead")

// normalize trims fields and checks them against the same rules as the landing form.
func (l *Lead) normalize() error {
	for _, f := range []*string{&l.Name, &l.Clinic, &l.Phone, &l.Email, &l.Doctors, &l.Message, &l.ConsentVersion, &l.PolicyVersion, &l.Locale, &l.Source} {
		*f = strings.TrimSpace(*f)
	}
	bad := func(field string) error { return fmt.Errorf("%w: %s", errInvalid, field) }
	switch {
	case l.Name == "" || utf8.RuneCountInString(l.Name) > 120:
		return bad("name")
	case l.Clinic == "" || utf8.RuneCountInString(l.Clinic) > 200:
		return bad("clinic")
	case !validPhone(l.Phone):
		return bad("phone")
	case l.Email != "" && (len(l.Email) > 200 || !emailRe.MatchString(l.Email)):
		return bad("email")
	case utf8.RuneCountInString(l.Doctors) > 40:
		return bad("doctors")
	case utf8.RuneCountInString(l.Message) > 2000:
		return bad("message")
	case !l.Consent: // 152-FZ art. 9: no processing without consent
		return bad("consent")
	case !versionRe.MatchString(l.ConsentVersion) || !versionRe.MatchString(l.PolicyVersion):
		return bad("version")
	case !locales[l.Locale]:
		return bad("locale")
	}
	return nil
}

func validPhone(p string) bool {
	if len(p) > 24 || !phoneRe.MatchString(p) {
		return false
	}
	digits := 0
	for _, r := range p {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 10 && digits <= 15
}
