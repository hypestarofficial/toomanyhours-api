package validate

import (
	"errors"
	"strings"
	"testing"
)

func TestUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "simple", input: "hype", want: "hype"},
		{name: "uppercase is normalized", input: "HypE", want: "hype"},
		{name: "surrounding space is trimmed", input: "  hype  ", want: "hype"},
		{name: "digits and underscore allowed", input: "hype_99", want: "hype_99"},
		{name: "minimum length", input: "abc", want: "abc"},
		{name: "maximum length", input: strings.Repeat("a", 16), want: strings.Repeat("a", 16)},

		{name: "too short", input: "ab", wantErr: ErrLength},
		{name: "too long", input: strings.Repeat("a", 17), wantErr: ErrLength},
		{name: "empty", input: "", wantErr: ErrLength},
		{name: "hyphen rejected", input: "hy-pe", wantErr: ErrFormat},
		{name: "space inside rejected", input: "hy pe", wantErr: ErrFormat},
		{name: "unicode rejected", input: "hypé", wantErr: ErrFormat},
		{name: "reserved word", input: "admin", wantErr: ErrReserved},
		{name: "reserved word regardless of case", input: "ADMIN", wantErr: ErrReserved},
		// "u" is reserved, but the length check runs first and wins. This pins
		// the order of checks so a reordering doesn't go unnoticed.
		{name: "reserved but too short, length wins", input: "u", wantErr: ErrLength},
		{name: "profanity rejected", input: "fuck", wantErr: ErrProfane},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Username(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Username(%q) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Username(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Username(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPassword(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "exactly minimum", input: "12345678"},
		{name: "exactly 72 bytes", input: strings.Repeat("a", 72)},
		{name: "one below minimum", input: "1234567", wantErr: ErrLength},
		{name: "73 bytes", input: strings.Repeat("a", 73), wantErr: ErrLength},
		// 37 "é" is 37 runes but 74 bytes. bcrypt truncates past 72 bytes, so
		// counting runes here would silently accept a password whose tail
		// never protects anything.
		{name: "multibyte over 72 bytes", input: strings.Repeat("é", 37), wantErr: ErrLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Password(tt.input)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Password(%d bytes) unexpected error: %v", len(tt.input), err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Password(%d bytes) error = %v, want %v", len(tt.input), err, tt.wantErr)
			}
		})
	}
}

func TestEmail(t *testing.T) {
	if got, err := Email("  Admin@Example.COM "); err != nil || got != "admin@example.com" {
		t.Fatalf("Email() = %q, %v; want %q, nil", got, err, "admin@example.com")
	}
	if _, err := Email("not-an-email"); !errors.Is(err, ErrFormat) {
		t.Fatalf("Email(\"not-an-email\") error = %v, want ErrFormat", err)
	}
}

func TestBio(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		got, err := Bio(nil)
		if err != nil {
			t.Fatalf("Bio(nil) error = %v, want nil", err)
		}
		if got != nil {
			t.Errorf("Bio(nil) = %v, want nil", got)
		}
	})

	// Cleared has one representation in the database, not two.
	t.Run("blank collapses to nil", func(t *testing.T) {
		for _, raw := range []string{"", "   ", "\n\t "} {
			got, err := Bio(&raw)
			if err != nil {
				t.Fatalf("Bio(%q) error = %v, want nil", raw, err)
			}
			if got != nil {
				t.Errorf("Bio(%q) = %q, want nil", raw, *got)
			}
		}
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		raw := "  plays too much Bethesda  "
		got, err := Bio(&raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || *got != "plays too much Bethesda" {
			t.Errorf("Bio = %v, want trimmed", got)
		}
	})

	t.Run("500 runes is allowed", func(t *testing.T) {
		raw := strings.Repeat("a", 500)
		if _, err := Bio(&raw); err != nil {
			t.Errorf("500 runes rejected: %v", err)
		}
	})

	t.Run("501 runes is rejected", func(t *testing.T) {
		raw := strings.Repeat("a", 501)
		if _, err := Bio(&raw); !errors.Is(err, ErrLength) {
			t.Errorf("err = %v, want ErrLength", err)
		}
	})

	// The test that fails the moment the cap is rewritten in bytes: 500 of
	// these are 1500 bytes. The limit is about how much somebody wrote.
	t.Run("500 multi-byte runes is allowed", func(t *testing.T) {
		raw := strings.Repeat("日", 500)
		if _, err := Bio(&raw); err != nil {
			t.Errorf("500 multi-byte runes rejected: %v", err)
		}
	})
}
