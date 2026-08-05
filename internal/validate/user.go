// Package validate holds the input rules for user accounts. It is deliberately
// free of database and HTTP dependencies so it can be tested in isolation, and
// so registration and profile updates cannot drift apart: both call the same
// functions, and both persist the normalized value returned here.
package validate

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	goaway "github.com/TwiN/go-away"
)

var (
	ErrLength   = errors.New("wrong length")
	ErrFormat   = errors.New("invalid format")
	ErrReserved = errors.New("reserved")
	ErrProfane  = errors.New("not allowed")
)

const (
	usernameMinLen = 3
	usernameMaxLen = 16
	passwordMinLen = 8
	// bcrypt silently ignores everything past 72 bytes. Rejecting is honest;
	// truncating would mean part of the password never protects anything.
	passwordMaxBytes = 72
)

// reservedUsernames keeps names that would be confusing or impersonating as
// public profile URLs. Kept here rather than in a handler so the rule applies
// identically at registration and at rename.
var reservedUsernames = map[string]struct{}{
	"admin": {}, "api": {}, "me": {}, "settings": {},
	"login": {}, "logout": {}, "register": {}, "support": {},
	"about": {}, "help": {}, "root": {}, "u": {},
}

// Username normalizes and validates a username, returning the normalized form
// that must be stored. Callers persist the returned value, never the input:
// the unique index is on LOWER(username), and storing an unnormalized value
// would let display and lookup disagree.
func Username(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))

	if len(name) < usernameMinLen || len(name) > usernameMaxLen {
		return "", ErrLength
	}

	// Byte iteration is safe only because the charset below is ASCII; a
	// multibyte rune fails the check and returns before length can mislead.
	for _, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit && r != '_' {
			return "", ErrFormat
		}
	}

	if _, taken := reservedUsernames[name]; taken {
		return "", ErrReserved
	}

	if goaway.IsProfane(name) {
		return "", ErrProfane
	}

	return name, nil
}

// Email normalizes and validates an email address, returning the normalized
// form to store. Lowercasing matters because the unique index is on LOWER(email).
func Email(raw string) (string, error) {
	addr := strings.ToLower(strings.TrimSpace(raw))
	if _, err := mail.ParseAddress(addr); err != nil {
		return "", ErrFormat
	}
	return addr, nil
}

// Password validates a plaintext password. It returns no normalized value:
// passwords are hashed, never stored or compared as text.
func Password(raw string) error {
	// Minimum counts characters, so an 8-character passphrase is 8 regardless
	// of script. Maximum counts bytes, because bcrypt's limit is in bytes.
	if utf8.RuneCountInString(raw) < passwordMinLen {
		return ErrLength
	}
	if len(raw) > passwordMaxBytes {
		return ErrLength
	}
	return nil
}
