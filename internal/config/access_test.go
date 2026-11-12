package config

import (
	"strings"
	"testing"
)

func TestValidateAccessRequiresRootRuntimeValues(t *testing.T) {
	validHash := "$2b$04$C6UzMDM.H6dfI/f/IKcEe.2Jp3P3XwD1d5W3hQ5QJ6GQq8H0nT0mK"
	for name, cfg := range map[string]Config{
		"missing invite hash": {
			SessionSigningSecret: strings.Repeat("a", 64),
		},
		"missing session secret": {
			InviteBcryptHash: validHash,
		},
		"malformed session secret": {
			InviteBcryptHash:     validHash,
			SessionSigningSecret: "not-hex",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := cfg.ValidateAccess(); err == nil {
				t.Fatal("ValidateAccess() unexpectedly accepted invalid configuration")
			}
		})
	}
}
