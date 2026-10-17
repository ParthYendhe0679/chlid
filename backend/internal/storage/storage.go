// Package storage keeps uploaded files in a private local folder and creates short-lived signed links
// (section 11.5 and 12.3). Files get random names; the original name is stored only in the database.
package storage

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

const LinkTTL = 5 * time.Minute

type Local struct {
	dir     string
	secret  []byte
	baseURL string // e.g. http://localhost:8080/api/v1
}

func NewLocal(dir, secret, baseURL string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Local{dir: dir, secret: []byte("files:" + secret), baseURL: baseURL}, nil
}

var keyRe = regexp.MustCompile(`^[a-z]+/[0-9a-f]{32}\.(pdf|jpg|png)$`)

// Save writes data under a random key such as "docs/<32 hex>.pdf" and returns the key.
func (l *Local) Save(prefix string, data []byte, mime string) (string, error) {
	ext, ok := map[string]string{"application/pdf": "pdf", "image/jpeg": "jpg", "image/png": "png"}[mime]
	if !ok {
		return "", errors.New("unsupported type")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s/%s.%s", prefix, hex.EncodeToString(b), ext)
	p := filepath.Join(l.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	return key, os.WriteFile(p, data, 0o600)
}

func (l *Local) Read(key string) ([]byte, error) {
	if !keyRe.MatchString(key) {
		return nil, errors.New("bad key")
	}
	return os.ReadFile(filepath.Join(l.dir, filepath.FromSlash(key)))
}

func (l *Local) sig(key string, exp int64) string {
	m := hmac.New(sha256.New, l.secret)
	m.Write([]byte(key))
	m.Write([]byte{0})
	m.Write([]byte(strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(m.Sum(nil))
}

// SignedURL returns a link valid for 5 minutes. Call it only after the permission check.
func (l *Local) SignedURL(key string) string {
	exp := time.Now().Add(LinkTTL).Unix()
	return fmt.Sprintf("%s/files/%s?exp=%d&sig=%s", l.baseURL, key, exp, l.sig(key, exp))
}

// VerifyLink checks the signature and expiry of a link created by SignedURL.
func (l *Local) VerifyLink(key, expStr, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(l.sig(key, exp)))
}

// SniffType checks the real file type from the first bytes, not the file name (section 16.2).
func SniffType(data []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return "application/pdf", true
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", true
	case bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", true
	}
	return "", false
}
