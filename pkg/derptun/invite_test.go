package derptun

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCompactInviteRoundTrip(t *testing.T) {
	now := time.Now().UTC()
	server, err := GenerateServerToken(ServerTokenOptions{Now: now, Days: 7})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	client, err := GenerateClientToken(ClientTokenOptions{Now: now, ServerToken: server, Days: 3})
	if err != nil {
		t.Fatalf("GenerateClientToken() error = %v", err)
	}

	invite, err := EncodeClientInvite(client)
	if err != nil {
		t.Fatalf("EncodeClientInvite() error = %v", err)
	}
	if !strings.HasPrefix(invite, CompactInvitePrefix) {
		t.Fatalf("invite prefix = %q, want %q", invite[:len(CompactInvitePrefix)], CompactInvitePrefix)
	}
	if strings.Contains(invite, "://") || strings.Contains(invite, "?") || strings.Contains(invite, "&") {
		t.Fatalf("invite = %q, want compact non-URL text", invite)
	}
	if len(invite) >= len(client) {
		t.Fatalf("compact invite length = %d, client token length = %d", len(invite), len(client))
	}

	cred, err := DecodeClientInvite(invite, now)
	if err != nil {
		t.Fatalf("DecodeClientInvite() error = %v", err)
	}
	original, err := DecodeClientToken(client, now)
	if err != nil {
		t.Fatalf("DecodeClientToken() error = %v", err)
	}
	if cred != original {
		t.Fatalf("decoded compact credential differs from original\ncompact=%#v\noriginal=%#v", cred, original)
	}
}

func TestCompactInviteRejectsMalformedInput(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	for _, raw := range []string{
		"",
		"dt1ABC",
		"DT2ABC",
		"DT1",
		"DT1not valid",
	} {
		if _, err := DecodeClientInvite(raw, now); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("DecodeClientInvite(%q) error = %v, want ErrInvalidToken", raw, err)
		}
	}
}

func TestCompactInviteRejectsExpiredCredential(t *testing.T) {
	now := time.Now().UTC()
	server, err := GenerateServerToken(ServerTokenOptions{Now: now, Days: 2})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	client, err := GenerateClientToken(ClientTokenOptions{Now: now, ServerToken: server, Days: 1})
	if err != nil {
		t.Fatalf("GenerateClientToken() error = %v", err)
	}
	invite, err := EncodeClientInvite(client)
	if err != nil {
		t.Fatalf("EncodeClientInvite() error = %v", err)
	}

	_, err = DecodeClientInvite(invite, now.Add(25*time.Hour))
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("DecodeClientInvite(expired) error = %v, want ErrExpired", err)
	}
}

func TestBase45RoundTrip(t *testing.T) {
	raw := []byte{0, 1, 2, 3, 4, 5, 254, 255}
	encoded := base45Encode(raw)
	decoded, err := base45Decode(encoded)
	if err != nil {
		t.Fatalf("base45Decode() error = %v", err)
	}
	if string(decoded) != string(raw) {
		t.Fatalf("decoded = %v, want %v", decoded, raw)
	}
}
