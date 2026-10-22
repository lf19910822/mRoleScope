package jobsync

import (
	"strings"
	"testing"
)

func TestSafeMessageRedactsSecretsAndBoundsLength(t *testing.T) {
	message := SafeMessage("password=hunter2 token:abc postgres://user:secret@db:5432/mrolescope Bearer abc.def.ghi")
	for _, unsafe := range []string{"hunter2", "abc.def.ghi", "postgres://", "secret@"} {
		if strings.Contains(message, unsafe) {
			t.Fatalf("unsafe value %q leaked into %q", unsafe, message)
		}
	}
	if len(SafeMessage(strings.Repeat("x", maxAuditMessageLength+10))) > maxAuditMessageLength+len("…") {
		t.Fatal("safe message was not bounded")
	}
}
