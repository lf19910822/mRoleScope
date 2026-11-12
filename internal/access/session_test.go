package access

import (
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func testInviteHash(t *testing.T, invite string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(invite), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate invite hash: %v", err)
	}
	return string(hash)
}

func TestSessionIssuesAndValidatesOpaqueCookie(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	service, err := New(Config{
		InviteBcryptHash:     testInviteHash(t, "known-invite"),
		SessionSigningSecret: strings.Repeat("a", 64),
		Now:                  func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	token, err := service.Issue("known-invite")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token == "" || strings.Contains(token, "known-invite") {
		t.Fatalf("session token is not opaque: %q", token)
	}
	if err := service.Validate(token); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestSessionRejectsIncorrectExpiredMalformedAndRotatedValues(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	hash := testInviteHash(t, "known-invite")
	config := Config{
		InviteBcryptHash:     hash,
		SessionSigningSecret: strings.Repeat("b", 64),
		Now:                  func() time.Time { return now },
	}
	service, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := service.Issue("wrong-invite"); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("wrong invite error = %v", err)
	}
	token, err := service.Issue("known-invite")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	for _, malformed := range []string{"", "not-a-token", token + ".extra"} {
		if err := service.Validate(malformed); !errors.Is(err, ErrInvalidSession) {
			t.Errorf("Validate(%q) error = %v, want invalid session", malformed, err)
		}
	}
	now = now.Add(3 * 24 * time.Hour)
	if err := service.Validate(token); !errors.Is(err, ErrExpiredSession) {
		t.Fatalf("expired session error = %v", err)
	}

	rotated, err := New(Config{
		InviteBcryptHash:     testInviteHash(t, "replacement-invite"),
		SessionSigningSecret: config.SessionSigningSecret,
		Now:                  func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New(rotated) error = %v", err)
	}
	if err := rotated.Validate(token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("rotated session error = %v, want invalid session", err)
	}
}

func TestSessionConfigurationFailsClosed(t *testing.T) {
	validHash := testInviteHash(t, "known-invite")
	for name, config := range map[string]Config{
		"missing invite hash": {
			SessionSigningSecret: strings.Repeat("a", 64),
		},
		"malformed invite hash": {
			InviteBcryptHash:     "not-a-bcrypt-hash",
			SessionSigningSecret: strings.Repeat("a", 64),
		},
		"missing signing secret": {
			InviteBcryptHash: validHash,
		},
		"short signing secret": {
			InviteBcryptHash:     validHash,
			SessionSigningSecret: "abcd",
		},
		"non-hex signing secret": {
			InviteBcryptHash:     validHash,
			SessionSigningSecret: strings.Repeat("z", 64),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(config); err == nil {
				t.Fatal("New() unexpectedly accepted invalid configuration")
			}
		})
	}
}
