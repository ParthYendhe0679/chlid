// Package rules holds the legal rules that run without AI: eligibility, fees, document categories,
// and the user-facing messages in English, Hindi and Marathi.
package rules

import (
	"net/http"
	"strings"
)

// Langs supported by the interface and assistant.
var Langs = []string{"en", "hi", "mr"}

func ValidLang(l string) bool { return l == "en" || l == "hi" || l == "mr" }

// Text is one message in the three languages. {name} placeholders are replaced by T.
type Text struct{ En, Hi, Mr string }

func (t Text) In(lang string, args ...string) string {
	s := t.En
	switch lang {
	case "hi":
		if t.Hi != "" {
			s = t.Hi
		}
	case "mr":
		if t.Mr != "" {
			s = t.Mr
		}
	}
	for i := 0; i+1 < len(args); i += 2 {
		s = strings.ReplaceAll(s, "{"+args[i]+"}", args[i+1])
	}
	return s
}

// All returns the message in every language, with the same arguments.
func (t Text) All(args ...string) (en, hi, mr string) {
	return t.In("en", args...), t.In("hi", args...), t.In("mr", args...)
}

// RequestLang picks the language for a request: the website's cc_locale cookie, then Accept-Language.
func RequestLang(r *http.Request, fallback string) string {
	if c, err := r.Cookie("cc_locale"); err == nil && ValidLang(c.Value) {
		return c.Value
	}
	if fallback != "" && ValidLang(fallback) {
		return fallback
	}
	al := strings.ToLower(r.Header.Get("Accept-Language"))
	switch {
	case strings.HasPrefix(al, "hi"):
		return "hi"
	case strings.HasPrefix(al, "mr"):
		return "mr"
	}
	return "en"
}
