// Package auth handles passwords, JWT cookies, authentication, role checks and rate limits.
package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessCookie  = "cc_session"
	RefreshCookie = "cc_refresh"
	AccessTTL     = 15 * time.Minute
	RefreshTTL    = 7 * 24 * time.Hour
	refreshPath   = "/api/v1/auth"
)

type Claims struct {
	Type string `json:"typ"` // "access" or "refresh"
	Role string `json:"role"`
	Ver  int    `json:"ver"`
	jwt.RegisteredClaims
}

type Tokens struct {
	secret []byte
	secure bool
}

func NewTokens(secret string, secure bool) *Tokens {
	return &Tokens{secret: []byte(secret), secure: secure}
}

func (t *Tokens) sign(uid uuid.UUID, role, typ string, ver int, ttl time.Duration) (string, error) {
	now := time.Now()
	c := Claims{Type: typ, Role: role, Ver: ver, RegisteredClaims: jwt.RegisteredClaims{
		Subject: uid.String(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl)), Issuer: "childconnect",
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
}

func (t *Tokens) Parse(tok, wantType string) (*Claims, error) {
	c := &Claims{}
	_, err := jwt.ParseWithClaims(tok, c, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("childconnect"), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if c.Type != wantType {
		return nil, errors.New("wrong token type")
	}
	return c, nil
}

// SetSession writes both cookies: httpOnly, SameSite=Lax, Secure when served over HTTPS.
func (t *Tokens) SetSession(w http.ResponseWriter, uid uuid.UUID, role string, ver int) error {
	acc, err := t.sign(uid, role, "access", ver, AccessTTL)
	if err != nil {
		return err
	}
	ref, err := t.sign(uid, role, "refresh", ver, RefreshTTL)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: AccessCookie, Value: acc, Path: "/", HttpOnly: true, Secure: t.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(AccessTTL.Seconds())})
	http.SetCookie(w, &http.Cookie{Name: RefreshCookie, Value: ref, Path: refreshPath, HttpOnly: true, Secure: t.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(RefreshTTL.Seconds())})
	return nil
}

func (t *Tokens) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: AccessCookie, Value: "", Path: "/", HttpOnly: true, Secure: t.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: RefreshCookie, Value: "", Path: refreshPath, HttpOnly: true, Secure: t.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
