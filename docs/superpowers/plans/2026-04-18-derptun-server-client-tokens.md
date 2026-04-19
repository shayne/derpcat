# Derptun Server And Client Tokens Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Execution status:** Completed on `main` across `8c75cd1`, `dcb7fef`, `36d5150`, `a160857`, `eac4744`, `f73d8ea`, and `281591e`. The final token defaults were later updated to 180 days for server tokens and 90 days for client tokens.

**Goal:** Replace the current all-in-one `derptun` token with breaking-change server and client token roles so only the server token can serve or mint client tokens.

**Architecture:** `dts1_` server tokens carry durable private server identity and a signing secret. `dtc1_` client tokens carry only client access material, server public identity, expiry, and a server-signed proof that the server can verify during rendezvous. `derptun serve` accepts only server tokens, while `derptun open` and `derptun connect` accept only client tokens.

**Tech Stack:** Go, existing DERP binding in `pkg/derpbind`, existing rendezvous claim flow in `pkg/rendezvous`, existing QUIC transport in `pkg/quicpath`, existing CLI parsing through `github.com/shayne/yargs`.

---

## Constraints And Current Baseline

- This is a breaking change. Do not support legacy `dt1_` tokens.
- Use the words `server` and `client` in public CLI flags, help text, tests, and docs.
- Use `serverhost` and `clienthost` only as example machine names in docs and shell examples.
- V1 supports one server, one client, and one active connector at a time.
- Leave an explicit code comment where single-client state is stored explaining that future multi-client support should replace the single active claim with a map keyed by client ID.
- Include stable `client_id`, per-issued-token `token_id`, and generated `client_name` metadata in client tokens now. Do not expose `--client-name` in V1. This prevents a token-format migration when multi-client support later adds human-readable naming.
- No filesystem persistence is added in this plan. Durability comes from the user storing the `dts1_` server token and reusing it from a process manager such as systemd.
- Active TCP streams are not process-restart durable. After a `derptun serve` restart, clients can reconnect with the same `dtc1_` token, but existing TCP sessions are gone.
- Worktree baseline:
  - `go test ./...` passed.
  - `mise trust` was required once for this new worktree.
  - `mise run test` passed after trust.

## Target CLI Workflow

On the example serving machine named `serverhost`, create a durable server credential:

```bash
derptun token server --days 365 > server.dts
```

On `serverhost`, mint a client access token for the example connecting machine named `clienthost`:

```bash
derptun token client --token "$(cat server.dts)" --days 7 > client.dtc
```

Run the serving side on `serverhost` from the server token:

```bash
derptun serve --token "$(cat server.dts)" --tcp 127.0.0.1:22
```

Copy only `client.dtc` to `clienthost`. Run the connecting side on `clienthost` from the client token:

```bash
derptun open --token "$(cat client.dtc)" --listen 127.0.0.1:2222
ssh -p 2222 user@127.0.0.1
```

Or use stdio for SSH `ProxyCommand`:

```bash
derptun connect --token "$(cat client.dtc)" --stdio
```

Legacy-looking commands must fail:

```bash
derptun token --days 7
derptun serve --token dt1_...
derptun open --token dt1_...
derptun connect --token dt1_... --stdio
```

## Future Multi-Client And Revocation Shape

This plan must not implement multi-client serving, token listing, revocation, state files, or `--max-clients`. It should only avoid designing those additions into a corner.

The future multi-client CLI should continue to use shell redirection for token files:

```bash
derptun token server --days 365 > server.dts
derptun token client --token "$(cat server.dts)" --client-name clienthost --days 30 > clienthost.dtc
derptun token client --token "$(cat server.dts)" --client-name alice-laptop --days 30 > alice-laptop.dtc
```

When multi-client serving is ungated, serving should add policy flags without changing token format:

```bash
derptun serve --token "$(cat server.dts)" --tcp 127.0.0.1:22 --max-clients 10
```

Durable per-client revocation requires server-side durable state and is explicitly future work. A future server state feature can add commands like:

```bash
derptun server init --token "$(cat server.dts)" --state /var/lib/derptun/server.json
derptun token client --server-state /var/lib/derptun/server.json --client-name clienthost --days 30 > clienthost.dtc
derptun token list --server-state /var/lib/derptun/server.json
derptun token revoke --server-state /var/lib/derptun/server.json --token-id t_...
derptun serve --server-state /var/lib/derptun/server.json --tcp 127.0.0.1:22 --max-clients 10
```

Without that future state file, V1 can expire individual client tokens and can rotate the server token to revoke all clients, but it cannot durably revoke exactly one client across process restarts. The V1 token shape still includes enough identity to support the later state-backed flow: `client_id` is the stable client key, `token_id` is the per-issued-token key for future revocation and audit, and `client_name` is generated display metadata that can later become user-supplied.

## File Structure

- Modify `pkg/derptun/token.go`: replace all-in-one credential with server/client token encode, decode, signing, and conversion helpers.
- Modify `pkg/derptun/token_test.go`: token role, expiry, proof, and legacy rejection tests.
- Modify `pkg/rendezvous/messages.go`: add optional client proof fields to claims without changing existing derphole claim behavior.
- Modify `pkg/rendezvous/state.go`: keep existing one-shot behavior; ignore optional client proof for non-derptun flows.
- Modify `pkg/rendezvous/durable_gate.go`: keep generic durable gate behavior for existing tests; derptun-specific proof validation belongs in `pkg/derptun`.
- Modify `pkg/session/derptun.go`: decode server on serve, decode client on connect/open, attach client proof to claims, verify proof on server side, and preserve single-active-client behavior.
- Modify `pkg/session/derptun_test.go`: update existing derptun session tests to generate a server token, mint a client token, serve with server token, and connect/open with client token.
- Modify `cmd/derptun/root.go`: update help and route `token server` / `token client`.
- Modify `cmd/derptun/token.go`: parse token subcommands and call the new package helpers.
- Modify `cmd/derptun/serve.go`: keep `--token`; pass it to server role validation.
- Modify `cmd/derptun/open.go`: keep `--token`; pass it to client role validation.
- Modify `cmd/derptun/connect.go`: keep `--token`; pass it to client role validation.
- Modify `cmd/derptun/*_test.go`: update command parsing tests and expected prefixes.
- Modify `scripts/smoke-remote-derptun.sh`: generate server/client tokens and pass role-specific flags.
- Modify `README.md`: update derptun examples and security language.

### Task 1: Split Derptun Token Types

**Files:**
- Modify: `pkg/derptun/token.go`
- Modify: `pkg/derptun/token_test.go`

- [x] **Step 1: Replace token tests with role-specific failing tests**

Replace `pkg/derptun/token_test.go` with:

```go
package derptun

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sessiontoken "github.com/shayne/derphole/pkg/token"
)

func TestGenerateServerTokenDefaultsToSevenDays(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	encoded, err := GenerateServerToken(ServerTokenOptions{Now: now})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	if got := encoded[:len(ServerTokenPrefix)]; got != ServerTokenPrefix {
		t.Fatalf("prefix = %q, want %q", got, ServerTokenPrefix)
	}
	cred, err := DecodeServerToken(encoded, now)
	if err != nil {
		t.Fatalf("DecodeServerToken() error = %v", err)
	}
	if got, want := time.Unix(cred.ExpiresUnix, 0).UTC(), now.Add(7*24*time.Hour); !got.Equal(want) {
		t.Fatalf("expiry = %s, want %s", got, want)
	}
}

func TestGenerateClientTokenFromServerToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	serverToken, err := GenerateServerToken(ServerTokenOptions{Now: now, Days: 30})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	clientToken, err := GenerateClientToken(ClientTokenOptions{
		Now:             now,
		ServerToken:     serverToken,
		Days:            7,
	})
	if err != nil {
		t.Fatalf("GenerateClientToken() error = %v", err)
	}
	if got := clientToken[:len(ClientTokenPrefix)]; got != ClientTokenPrefix {
		t.Fatalf("prefix = %q, want %q", got, ClientTokenPrefix)
	}
	clientCred, err := DecodeClientToken(clientToken, now)
	if err != nil {
		t.Fatalf("DecodeClientToken() error = %v", err)
	}
	if clientCred.ClientName == "" {
		t.Fatalf("ClientName = empty, want generated name")
	}
	payload := decodeTokenPayload(t, ClientTokenPrefix, clientToken)
	for _, field := range []string{"derp_private", "quic_private", "signing_secret"} {
		if _, ok := payload[field]; ok {
			t.Fatalf("client token exposed private field %q", field)
		}
	}
	if got, want := time.Unix(clientCred.ExpiresUnix, 0).UTC(), now.Add(7*24*time.Hour); !got.Equal(want) {
		t.Fatalf("client expiry = %s, want %s", got, want)
	}
}

func TestClientTokenCannotOutliveServerToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	server, err := GenerateServerToken(ServerTokenOptions{Now: now, Days: 1})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	_, err = GenerateClientToken(ClientTokenOptions{
		Now:             now,
		ServerToken:     server,
		Days:            2,
	})
	if err == nil || err.Error() != "client expiry exceeds server expiry" {
		t.Fatalf("GenerateClientToken() error = %v, want client expiry exceeds server expiry", err)
	}
}

func TestDecodeRejectsWrongTokenRole(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	server, err := GenerateServerToken(ServerTokenOptions{Now: now})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	client, err := GenerateClientToken(ClientTokenOptions{Now: now, ServerToken: server})
	if err != nil {
		t.Fatalf("GenerateClientToken() error = %v", err)
	}
	if _, err := DecodeClientToken(server, now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("DecodeClientToken(server) error = %v, want ErrInvalidToken", err)
	}
	if _, err := DecodeServerToken(client, now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("DecodeServerToken(client) error = %v, want ErrInvalidToken", err)
	}
	if _, err := DecodeServerToken("dt1_legacy", now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("DecodeServerToken(legacy) error = %v, want ErrInvalidToken", err)
	}
}

func TestServerAndClientSessionTokensShareServerIdentity(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	server, err := GenerateServerToken(ServerTokenOptions{Now: now})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	serverCred, err := DecodeServerToken(server, now)
	if err != nil {
		t.Fatalf("DecodeServerToken() error = %v", err)
	}
	client, err := GenerateClientToken(ClientTokenOptions{Now: now, ServerToken: server})
	if err != nil {
		t.Fatalf("GenerateClientToken() error = %v", err)
	}
	clientCred, err := DecodeClientToken(client, now)
	if err != nil {
		t.Fatalf("DecodeClientToken() error = %v", err)
	}
	serverTok, err := serverCred.SessionToken()
	if err != nil {
		t.Fatalf("server SessionToken() error = %v", err)
	}
	clientTok, err := clientCred.SessionToken()
	if err != nil {
		t.Fatalf("client SessionToken() error = %v", err)
	}
	if serverTok.SessionID != clientTok.SessionID {
		t.Fatalf("session mismatch")
	}
	if serverTok.DERPPublic != clientTok.DERPPublic {
		t.Fatalf("DERP public mismatch")
	}
	if serverTok.QUICPublic != clientTok.QUICPublic {
		t.Fatalf("QUIC public mismatch")
	}
	if clientTok.Capabilities&sessiontoken.CapabilityDerptunTCP == 0 {
		t.Fatalf("client capabilities = %b, want derptun tcp bit", clientTok.Capabilities)
	}
}

func decodeTokenPayload(t *testing.T, prefix, encoded string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(encoded[len(prefix):])
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return payload
}
```

- [x] **Step 2: Run token tests and verify they fail**

Run: `go test ./pkg/derptun -run 'TestGenerateServerToken|TestGenerateClientToken|TestClientToken|TestDecodeRejectsWrongTokenRole|TestServerAndClientSessionTokens' -count=1`

Expected: FAIL with undefined identifiers such as `GenerateServerToken`, `ServerTokenOptions`, and `ClientTokenPrefix`.

- [x] **Step 3: Implement server/client token types**

Replace `pkg/derptun/token.go` with a role split based on this structure:

```go
package derptun

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sessiontoken "github.com/shayne/derphole/pkg/token"
	"tailscale.com/types/key"
)

const (
	ServerTokenPrefix = "dts1_"
	ClientTokenPrefix = "dtc1_"
	TokenVersion          = 1
	DefaultDays           = 7
	ProtocolTCP           = "tcp"
	ProtocolUDP           = "udp"
)

var (
	ErrExpired      = errors.New("derptun token expired")
	ErrInvalidToken = errors.New("invalid derptun token")
)

type ServerTokenOptions struct {
	Now     time.Time
	Days    int
	Expires time.Time
}

type ClientTokenOptions struct {
	Now             time.Time
	ServerToken     string
	Days            int
	Expires         time.Time
}

type ForwardSpec struct {
	Protocol           string `json:"protocol"`
	ListenAddr         string `json:"listen_addr,omitempty"`
	TargetAddr         string `json:"target_addr,omitempty"`
	IdleTimeoutSeconds int    `json:"idle_timeout_seconds,omitempty"`
}

type ServerCredential struct {
	Version       int           `json:"version"`
	SessionID     [16]byte      `json:"session_id"`
	ExpiresUnix   int64         `json:"expires_unix"`
	DERPPrivate   string        `json:"derp_private"`
	QUICPrivate   []byte        `json:"quic_private"`
	SigningSecret [32]byte      `json:"signing_secret"`
	Forwards      []ForwardSpec `json:"forwards,omitempty"`
}

type ClientCredential struct {
	Version      int      `json:"version"`
	SessionID    [16]byte `json:"session_id"`
	ClientID     [16]byte `json:"client_id"`
	TokenID      [16]byte `json:"token_id"`
	ClientName   string   `json:"client_name"`
	ExpiresUnix  int64    `json:"expires_unix"`
	DERPPublic   [32]byte `json:"derp_public"`
	QUICPublic   [32]byte `json:"quic_public"`
	BearerSecret [32]byte `json:"bearer_secret"`
	ProofMAC     string   `json:"proof_mac"`
}
```

Add helpers:

```go
func GenerateServerToken(opts ServerTokenOptions) (string, error) {
	now := normalizedNow(opts.Now)
	expires, err := resolveExpiry(now, opts.Days, opts.Expires)
	if err != nil {
		return "", err
	}
	cred := ServerCredential{Version: TokenVersion, ExpiresUnix: expires.Unix()}
	if _, err := rand.Read(cred.SessionID[:]); err != nil {
		return "", err
	}
	if _, err := rand.Read(cred.SigningSecret[:]); err != nil {
		return "", err
	}
	derpPrivate := key.NewNode()
	derpText, err := derpPrivate.MarshalText()
	if err != nil {
		return "", err
	}
	cred.DERPPrivate = string(derpText)
	_, quicPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	cred.QUICPrivate = append([]byte(nil), quicPrivate...)
	return encodeJSONToken(ServerTokenPrefix, cred)
}

func GenerateClientToken(opts ClientTokenOptions) (string, error) {
	now := normalizedNow(opts.Now)
	server, err := DecodeServerToken(opts.ServerToken, now)
	if err != nil {
		return "", err
	}
	expires, err := resolveExpiry(now, opts.Days, opts.Expires)
	if err != nil {
		return "", err
	}
	if expires.Unix() > server.ExpiresUnix {
		return "", fmt.Errorf("client expiry exceeds server expiry")
	}
	client := ClientCredential{
		Version:    TokenVersion,
		SessionID:  server.SessionID,
		ExpiresUnix: expires.Unix(),
	}
	if _, err := rand.Read(client.ClientID[:]); err != nil {
		return "", err
	}
	if _, err := rand.Read(client.TokenID[:]); err != nil {
		return "", err
	}
	client.ClientName = "client-" + hex.EncodeToString(client.ClientID[:4])
	serverTok, err := server.SessionToken()
	if err != nil {
		return "", err
	}
	client.DERPPublic = serverTok.DERPPublic
	client.QUICPublic = serverTok.QUICPublic
	client.BearerSecret = deriveClientBearerSecret(server.SigningSecret, client.ClientID)
	client.ProofMAC = computeClientProofMAC(server.SigningSecret, client)
	return encodeJSONToken(ClientTokenPrefix, client)
}

func DecodeServerToken(encoded string, now time.Time) (ServerCredential, error) {
	var cred ServerCredential
	if err := decodeJSONToken(encoded, ServerTokenPrefix, &cred); err != nil {
		return ServerCredential{}, err
	}
	if cred.Version != TokenVersion || cred.SessionID == ([16]byte{}) || cred.DERPPrivate == "" || len(cred.QUICPrivate) != ed25519.PrivateKeySize || cred.SigningSecret == ([32]byte{}) {
		return ServerCredential{}, ErrInvalidToken
	}
	if _, err := cred.DERPKey(); err != nil {
		return ServerCredential{}, ErrInvalidToken
	}
	if expired(now, cred.ExpiresUnix) {
		return ServerCredential{}, ErrExpired
	}
	return cred, nil
}

func DecodeClientToken(encoded string, now time.Time) (ClientCredential, error) {
	var cred ClientCredential
	if err := decodeJSONToken(encoded, ClientTokenPrefix, &cred); err != nil {
		return ClientCredential{}, err
	}
	if cred.Version != TokenVersion || cred.SessionID == ([16]byte{}) || cred.ClientID == ([16]byte{}) || cred.TokenID == ([16]byte{}) || cred.ClientName == "" || cred.DERPPublic == ([32]byte{}) || cred.QUICPublic == ([32]byte{}) || cred.BearerSecret == ([32]byte{}) || cred.ProofMAC == "" {
		return ClientCredential{}, ErrInvalidToken
	}
	if expired(now, cred.ExpiresUnix) {
		return ClientCredential{}, ErrExpired
	}
	return cred, nil
}
```

Add identity and conversion methods:

```go
func (cred ServerCredential) DERPKey() (key.NodePrivate, error) {
	var derpKey key.NodePrivate
	if err := derpKey.UnmarshalText([]byte(cred.DERPPrivate)); err != nil {
		return key.NodePrivate{}, err
	}
	return derpKey, nil
}

func (cred ServerCredential) QUICPrivateKey() (ed25519.PrivateKey, error) {
	if len(cred.QUICPrivate) != ed25519.PrivateKeySize {
		return nil, ErrInvalidToken
	}
	return ed25519.PrivateKey(append([]byte(nil), cred.QUICPrivate...)), nil
}

func (cred ServerCredential) SessionToken() (sessiontoken.Token, error) {
	derpKey, err := cred.DERPKey()
	if err != nil {
		return sessiontoken.Token{}, err
	}
	quicPrivate, err := cred.QUICPrivateKey()
	if err != nil {
		return sessiontoken.Token{}, err
	}
	var quicPublic [32]byte
	copy(quicPublic[:], quicPrivate.Public().(ed25519.PublicKey))
	var derpPublic [32]byte
	copy(derpPublic[:], derpKey.Public().AppendTo(nil))
	return sessiontoken.Token{
		Version:      sessiontoken.SupportedVersion,
		SessionID:    cred.SessionID,
		ExpiresUnix:  cred.ExpiresUnix,
		DERPPublic:   derpPublic,
		QUICPublic:   quicPublic,
		BearerSecret: deriveClientBearerSecret(cred.SigningSecret, [16]byte{}),
		Capabilities: sessiontoken.CapabilityDerptunTCP,
	}, nil
}

func (cred ClientCredential) SessionToken() (sessiontoken.Token, error) {
	return sessiontoken.Token{
		Version:      sessiontoken.SupportedVersion,
		SessionID:    cred.SessionID,
		ExpiresUnix:  cred.ExpiresUnix,
		DERPPublic:   cred.DERPPublic,
		QUICPublic:   cred.QUICPublic,
		BearerSecret: cred.BearerSecret,
		Capabilities: sessiontoken.CapabilityDerptunTCP,
	}, nil
}
```

Add common helpers:

```go
func normalizedNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}

func resolveExpiry(now time.Time, days int, expires time.Time) (time.Time, error) {
	if expires.IsZero() {
		if days == 0 {
			days = DefaultDays
		}
		if days < 1 {
			return time.Time{}, fmt.Errorf("days must be at least 1")
		}
		expires = now.Add(time.Duration(days) * 24 * time.Hour)
	}
	if !expires.After(now) {
		return time.Time{}, fmt.Errorf("expiry must be in the future")
	}
	return expires, nil
}

func expired(now time.Time, expiresUnix int64) bool {
	if now.IsZero() {
		now = time.Now()
	}
	return now.Unix() >= expiresUnix
}

func encodeJSONToken(prefix string, value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeJSONToken(encoded, prefix string, dst any) error {
	if len(encoded) <= len(prefix) || encoded[:len(prefix)] != prefix {
		return ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded[len(prefix):])
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return err
	}
	return nil
}

func deriveClientBearerSecret(secret [32]byte, clientID [16]byte) [32]byte {
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte("derptun-client-bearer-v1"))
	mac.Write(clientID[:])
	sum := mac.Sum(nil)
	var out [32]byte
	copy(out[:], sum)
	return out
}

func computeClientProofMAC(secret [32]byte, client ClientCredential) string {
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte("derptun-client-proof-v1"))
	mac.Write(client.SessionID[:])
	mac.Write(client.ClientID[:])
	mac.Write(client.TokenID[:])
	mac.Write([]byte(client.ClientName))
	mac.Write(client.DERPPublic[:])
	mac.Write(client.QUICPublic[:])
	mac.Write(client.BearerSecret[:])
	mac.Write([]byte(fmt.Sprintf("%d", client.ExpiresUnix)))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyClientCredential(secret [32]byte, client ClientCredential, now time.Time) error {
	if expired(now, client.ExpiresUnix) {
		return ErrExpired
	}
	if client.BearerSecret != deriveClientBearerSecret(secret, client.ClientID) {
		return ErrInvalidToken
	}
	if !hmac.Equal([]byte(client.ProofMAC), []byte(computeClientProofMAC(secret, client))) {
		return ErrInvalidToken
	}
	return nil
}
```

- [x] **Step 4: Run token tests**

Run: `go test ./pkg/derptun -count=1`

Expected: PASS.

- [x] **Step 5: Commit token split**

```bash
git add pkg/derptun/token.go pkg/derptun/token_test.go
git commit -m "feat: split derptun server and client tokens"
```

### Task 2: Add Client Claim Proof To Rendezvous

**Files:**
- Modify: `pkg/rendezvous/messages.go`
- Modify: `pkg/rendezvous/rendezvous_test.go`

- [x] **Step 1: Write failing claim serialization test**

Add this test to `pkg/rendezvous/rendezvous_test.go`:

```go
func TestClaimSerializesClientProof(t *testing.T) {
	claim := Claim{
		Version:      token.SupportedVersion,
		SessionID:    [16]byte{1, 2, 3, 4},
		BearerMAC:    "mac",
		DERPPublic:   [32]byte{5, 6, 7, 8},
		QUICPublic:   [32]byte{9, 10, 11, 12},
		Candidates:   []string{"udp4:127.0.0.1:1"},
		Capabilities: token.CapabilityDerptunTCP,
		Client: &ClientProof{
			ClientID:    [16]byte{7, 7, 7, 7},
			TokenID:     [16]byte{8, 8, 8, 8},
			ClientName:  "clienthost",
			ExpiresUnix: 1_700_000_600,
			ProofMAC:    "proof",
		},
	}
	encodedClaim, err := EncodeClaim(claim)
	if err != nil {
		t.Fatalf("EncodeClaim() error = %v", err)
	}
	decodedClaim, err := DecodeClaim(encodedClaim)
	if err != nil {
		t.Fatalf("DecodeClaim() error = %v", err)
	}
	if decodedClaim.Client == nil {
		t.Fatalf("Client = nil, want proof")
	}
	if decodedClaim.Client.ClientID != claim.Client.ClientID {
		t.Fatalf("ClientID = %x, want %x", decodedClaim.Client.ClientID, claim.Client.ClientID)
	}
	if decodedClaim.Client.TokenID != claim.Client.TokenID {
		t.Fatalf("TokenID = %x, want %x", decodedClaim.Client.TokenID, claim.Client.TokenID)
	}
	if decodedClaim.Client.ClientName != claim.Client.ClientName {
		t.Fatalf("ClientName = %q, want %q", decodedClaim.Client.ClientName, claim.Client.ClientName)
	}
	if decodedClaim.Client.ExpiresUnix != claim.Client.ExpiresUnix {
		t.Fatalf("ExpiresUnix = %d, want %d", decodedClaim.Client.ExpiresUnix, claim.Client.ExpiresUnix)
	}
	if decodedClaim.Client.ProofMAC != claim.Client.ProofMAC {
		t.Fatalf("ProofMAC = %q, want %q", decodedClaim.Client.ProofMAC, claim.Client.ProofMAC)
	}
}
```

- [x] **Step 2: Run rendezvous test and verify it fails**

Run: `go test ./pkg/rendezvous -run TestClaimSerializesClientProof -count=1`

Expected: FAIL with `undefined: ClientProof` or `Claim has no field or method Client`.

- [x] **Step 3: Add optional claim proof fields**

In `pkg/rendezvous/messages.go`, add this type and field to `Claim`:

```go
type ClientProof struct {
	ClientID    [16]byte `json:"client_id"`
	TokenID     [16]byte `json:"token_id"`
	ClientName  string   `json:"client_name"`
	ExpiresUnix int64   `json:"expires_unix"`
	ProofMAC    string  `json:"proof_mac"`
}

type Claim struct {
	Version      uint8            `json:"version"`
	SessionID    [16]byte         `json:"session_id"`
	BearerMAC    string           `json:"bearer_mac"`
	DERPPublic   [32]byte         `json:"derp_public"`
	QUICPublic   [32]byte         `json:"quic_public"`
	Parallel     int              `json:"parallel,omitempty"`
	Candidates   []string         `json:"candidates,omitempty"`
	Capabilities uint32           `json:"capabilities,omitempty"`
	Client       *ClientProof    `json:"client,omitempty"`
}
```

Keep `Client` optional. Do not change `validateClaimForToken` in `pkg/rendezvous/state.go`; non-derptun flows must continue to ignore absent or present optional proof fields.

- [x] **Step 4: Run rendezvous tests**

Run: `go test ./pkg/rendezvous -count=1`

Expected: PASS.

- [x] **Step 5: Commit claim proof**

```bash
git add pkg/rendezvous/messages.go pkg/rendezvous/rendezvous_test.go
git commit -m "feat: carry derptun client proof in claims"
```

### Task 3: Validate Client Proof In Derptun Session Flow

**Files:**
- Modify: `pkg/session/derptun.go`
- Modify: `pkg/session/derptun_test.go`

- [x] **Step 1: Update session tests for role-specific tokens**

At the top of `pkg/session/derptun_test.go`, add this helper:

```go
func derptunServerAndClientTokens(t *testing.T) (string, string) {
	t.Helper()
	now := time.Now()
	server, err := derptun.GenerateServerToken(derptun.ServerTokenOptions{Now: now, Days: 1})
	if err != nil {
		t.Fatalf("GenerateServerToken() error = %v", err)
	}
	client, err := derptun.GenerateClientToken(derptun.ClientTokenOptions{
		Now:             now,
		ServerToken:     server,
		Days:            1,
	})
	if err != nil {
		t.Fatalf("GenerateClientToken() error = %v", err)
	}
	return server, client
}
```

Replace each session test token setup:

```go
tokenValue, err := derptun.GenerateToken(derptun.TokenOptions{Now: time.Now()})
if err != nil {
	t.Fatalf("GenerateToken() error = %v", err)
}
```

with:

```go
serverToken, clientToken := derptunServerAndClientTokens(t)
```

Replace serve config calls:

```go
DerptunServe(ctx, DerptunServeConfig{Token: tokenValue, TargetAddr: backend})
```

with:

```go
DerptunServe(ctx, DerptunServeConfig{ServerToken: serverToken, TargetAddr: backend})
```

Replace open/connect config calls:

```go
DerptunOpen(ctx, DerptunOpenConfig{Token: tokenValue, ListenAddr: "127.0.0.1:0", BindAddrSink: bindCh})
DerptunConnect(ctx, DerptunConnectConfig{Token: tokenValue, StdioIn: strings.NewReader(line), StdioOut: &out})
```

with:

```go
DerptunOpen(ctx, DerptunOpenConfig{ClientToken: clientToken, ListenAddr: "127.0.0.1:0", BindAddrSink: bindCh})
DerptunConnect(ctx, DerptunConnectConfig{ClientToken: clientToken, StdioIn: strings.NewReader(line), StdioOut: &out})
```

Add this negative test:

```go
func TestDerptunRejectsWrongTokenRoles(t *testing.T) {
	serverToken, clientToken := derptunServerAndClientTokens(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := DerptunServe(ctx, DerptunServeConfig{ServerToken: clientToken, TargetAddr: "127.0.0.1:22"}); !errors.Is(err, derptun.ErrInvalidToken) {
		t.Fatalf("DerptunServe(client) error = %v, want ErrInvalidToken", err)
	}
	if err := DerptunConnect(ctx, DerptunConnectConfig{ClientToken: serverToken, StdioIn: strings.NewReader("x"), StdioOut: io.Discard}); !errors.Is(err, derptun.ErrInvalidToken) {
		t.Fatalf("DerptunConnect(server) error = %v, want ErrInvalidToken", err)
	}
}
```

- [x] **Step 2: Run derptun session tests and verify they fail**

Run: `go test ./pkg/session -run 'TestDerptun' -count=1`

Expected: FAIL with unknown config fields `ServerToken` and `ClientToken`.

- [x] **Step 3: Rename session config fields**

In `pkg/session/derptun.go`, replace config structs with:

```go
type DerptunServeConfig struct {
	ServerToken string
	TargetAddr      string
	Emitter         *telemetry.Emitter
	ForceRelay      bool
	UsePublicDERP   bool
}

type DerptunOpenConfig struct {
	ClientToken string
	ListenAddr      string
	BindAddrSink    chan<- string
	Emitter         *telemetry.Emitter
	ForceRelay      bool
	UsePublicDERP   bool
}

type DerptunConnectConfig struct {
	ClientToken string
	StdioIn         io.Reader
	StdioOut        io.Writer
	Emitter         *telemetry.Emitter
	ForceRelay      bool
	UsePublicDERP   bool
}
```

Replace `decodeDerptunCredential` with:

```go
func decodeDerptunServer(raw string) (derptun.ServerCredential, error) {
	return derptun.DecodeServerToken(raw, time.Now())
}

func decodeDerptunClient(raw string) (derptun.ClientCredential, error) {
	return derptun.DecodeClientToken(raw, time.Now())
}
```

- [x] **Step 4: Decode server in `DerptunServe`**

In `DerptunServe`, replace:

```go
cred, err := decodeDerptunCredential(cfg.Token)
```

with:

```go
cred, err := decodeDerptunServer(cfg.ServerToken)
```

Keep the existing `SessionToken`, `DERPKey`, and `QUICPrivateKey` flow, now called on `derptun.ServerCredential`.

- [x] **Step 5: Decode client in dial path and attach proof**

Change `dialDerptunMux` signature:

```go
func dialDerptunMux(ctx context.Context, tokenValue string, emitter *telemetry.Emitter, forceRelay bool) (*derptun.Mux, func(), error)
```

to:

```go
func dialDerptunMux(ctx context.Context, clientToken string, emitter *telemetry.Emitter, forceRelay bool) (*derptun.Mux, func(), error)
```

Inside it, replace:

```go
cred, err := decodeDerptunCredential(tokenValue)
if err != nil {
	return nil, nil, err
}
tok, err := cred.SessionToken()
```

with:

```go
cred, err := decodeDerptunClient(clientToken)
if err != nil {
	return nil, nil, err
}
tok, err := cred.SessionToken()
```

When constructing the claim, add the proof:

```go
claim := rendezvous.Claim{
	Version:      tok.Version,
	SessionID:    tok.SessionID,
	DERPPublic:   derpPublicKeyRaw32(derpClient.PublicKey()),
	QUICPublic:   clientIdentity.Public,
	Candidates:   localCandidates,
	Capabilities: tok.Capabilities,
	Client: &rendezvous.ClientProof{
		ClientID:    cred.ClientID,
		TokenID:     cred.TokenID,
		ClientName:  cred.ClientName,
		ExpiresUnix: cred.ExpiresUnix,
		ProofMAC:    cred.ProofMAC,
	},
}
```

Update `DerptunOpen` and `DerptunConnect` to pass `cfg.ClientToken` to `dialDerptunMux`.

- [x] **Step 6: Verify client proof before accepting a claim**

Add this import to `pkg/session/derptun.go`:

```go
sessiontoken "github.com/shayne/derphole/pkg/token"
```

Add helper in `pkg/session/derptun.go`:

```go
func derptunServerTokenForClaim(server derptun.ServerCredential, claim rendezvous.Claim, now time.Time) (sessiontoken.Token, rendezvous.Decision, error) {
	if claim.Client == nil {
		return sessiontoken.Token{}, rendezvous.Decision{Accepted: false, Reject: &rendezvous.RejectInfo{Code: rendezvous.RejectClaimMalformed, Reason: "client proof missing"}}, rendezvous.ErrDenied
	}
	client := derptun.ClientCredential{
		Version:      derptun.TokenVersion,
		SessionID:    claim.SessionID,
		ClientID:     claim.Client.ClientID,
		TokenID:      claim.Client.TokenID,
		ClientName:   claim.Client.ClientName,
		ExpiresUnix:  claim.Client.ExpiresUnix,
		ProofMAC:     claim.Client.ProofMAC,
	}
	serverTok, err := server.SessionToken()
	if err != nil {
		return sessiontoken.Token{}, rendezvous.Decision{}, err
	}
	client.DERPPublic = serverTok.DERPPublic
	client.QUICPublic = serverTok.QUICPublic
	client.BearerSecret = derptun.DeriveClientBearerSecretForClaim(server.SigningSecret, client.ClientID)
	if err := derptun.VerifyClientCredential(server.SigningSecret, client, now); err != nil {
		reason := "client proof invalid"
		code := rendezvous.RejectBadMAC
		if errors.Is(err, derptun.ErrExpired) {
			reason = "client token expired"
			code = rendezvous.RejectExpired
		}
		return sessiontoken.Token{}, rendezvous.Decision{Accepted: false, Reject: &rendezvous.RejectInfo{Code: code, Reason: reason}}, err
	}
	serverTok.BearerSecret = client.BearerSecret
	serverTok.ExpiresUnix = client.ExpiresUnix
	return serverTok, rendezvous.Decision{}, nil
}
```

Export this wrapper from `pkg/derptun/token.go` so `pkg/session` can derive the claim bearer without exposing the signing secret:

```go
func DeriveClientBearerSecretForClaim(secret [32]byte, clientID [16]byte) [32]byte {
	return deriveClientBearerSecret(secret, clientID)
}
```

Update `serveDerptunClaims` to receive `server derptun.ServerCredential` and create the durable gate after each verified claim instead of once from the server token. The top of `handleDerptunServeClaim` should do:

```go
claim := *env.Claim
peerDERP := key.NodePublicFromRaw32(mem.B(claim.DERPPublic[:]))
claimToken, rejectDecision, err := derptunServerTokenForClaim(server, claim, time.Now())
if err != nil {
	if sendErr := sendEnvelope(ctx, derpClient, peerDERP, envelope{Type: envelopeDecision, Decision: &rejectDecision}); sendErr != nil {
		return active, sendErr
	}
	return active, nil
}
decision, _ := gate.Accept(time.Now(), claimToken, claim)
```

Because `rendezvous.DurableGate` currently stores one token, replace it with a derptun-specific gate in this file:

```go
type derptunClientGate struct {
	active *rendezvous.Claim
}

func (g *derptunClientGate) Accept(now time.Time, tok sessiontoken.Token, claim rendezvous.Claim) (rendezvous.Decision, error) {
	// V1 intentionally allows one active client. Multi-client support should replace
	// this single active claim with a map keyed by client ID and independent tunnel state.
	if g.active != nil {
		if sameDerptunConnector(*g.active, claim) {
			return rendezvous.NewGate(tok).Accept(now, claim)
		}
		return rendezvous.Decision{Accepted: false, Reject: &rendezvous.RejectInfo{Code: rendezvous.RejectClaimed, Reason: "session already claimed"}}, rendezvous.ErrClaimed
	}
	decision, err := rendezvous.NewGate(tok).Accept(now, claim)
	if err != nil {
		return decision, err
	}
	stored := claim
	stored.Candidates = append([]string(nil), claim.Candidates...)
	g.active = &stored
	return decision, nil
}

func (g *derptunClientGate) Release(derpPublic [32]byte) {
	if g.active == nil || g.active.DERPPublic != derpPublic {
		return
	}
	g.active = nil
}

func sameDerptunConnector(a, b rendezvous.Claim) bool {
	return a.DERPPublic == b.DERPPublic && a.QUICPublic == b.QUICPublic
}
```

Use `*derptunClientGate` in `serveDerptunClaims` and `handleDerptunServeClaim` in place of `*rendezvous.DurableGate`.

- [x] **Step 7: Run focused session tests**

Run: `go test ./pkg/session -run 'TestDerptun' -count=1`

Expected: PASS.

- [x] **Step 8: Commit session split**

```bash
git add pkg/session/derptun.go pkg/session/derptun_test.go pkg/derptun/token.go
git commit -m "feat: enforce derptun token roles in sessions"
```

### Task 4: Update Derptun CLI For Server And Client Tokens

**Files:**
- Modify: `cmd/derptun/root.go`
- Modify: `cmd/derptun/token.go`
- Modify: `cmd/derptun/serve.go`
- Modify: `cmd/derptun/open.go`
- Modify: `cmd/derptun/connect.go`
- Modify: `cmd/derptun/root_test.go`
- Modify: `cmd/derptun/token_test.go`
- Modify: `cmd/derptun/command_test.go`

- [x] **Step 1: Update CLI command tests**

Replace `cmd/derptun/token_test.go` with:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunTokenServerPrintsServerToken(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"token", "server", "--days", "7"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "dts1_") {
		t.Fatalf("stdout = %q, want server token", stdout.String())
	}
}

func TestRunTokenClientPrintsClientToken(t *testing.T) {
	var serverOut bytes.Buffer
	code := run([]string{"token", "server", "--days", "7"}, strings.NewReader(""), &serverOut, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("server code = %d", code)
	}
	var stdout, stderr bytes.Buffer
	code = run([]string{"token", "client", "--token", strings.TrimSpace(serverOut.String()), "--days", "1"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "dtc1_") {
		t.Fatalf("stdout = %q, want client token", stdout.String())
	}
}

func TestRunTokenRequiresRole(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"token", "--days", "7"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "server") || !strings.Contains(stderr.String(), "client") {
		t.Fatalf("stderr = %q, want role help", stderr.String())
	}
}
```

In `cmd/derptun/command_test.go`, update expectations:

```go
func TestRunServePassesServerTokenAndTCP(t *testing.T) {
	oldServe := derptunServe
	defer func() { derptunServe = oldServe }()
	derptunServe = func(ctx context.Context, cfg session.DerptunServeConfig) error {
		if cfg.ServerToken != "dts1_test" {
			t.Fatalf("ServerToken = %q, want dts1_test", cfg.ServerToken)
		}
		if cfg.TargetAddr != "127.0.0.1:22" {
			t.Fatalf("TargetAddr = %q, want 127.0.0.1:22", cfg.TargetAddr)
		}
		if !cfg.UsePublicDERP {
			t.Fatal("UsePublicDERP = false, want true")
		}
		return nil
	}
	code := run([]string{"serve", "--token", "dts1_test", "--tcp", "127.0.0.1:22"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
}

func TestRunOpenPrintsBindAddress(t *testing.T) {
	oldOpen := derptunOpen
	defer func() { derptunOpen = oldOpen }()
	derptunOpen = func(ctx context.Context, cfg session.DerptunOpenConfig) error {
		if cfg.ClientToken != "dtc1_test" {
			t.Fatalf("ClientToken = %q, want dtc1_test", cfg.ClientToken)
		}
		if cfg.ListenAddr != "127.0.0.1:2222" {
			t.Fatalf("ListenAddr = %q, want 127.0.0.1:2222", cfg.ListenAddr)
		}
		cfg.BindAddrSink <- "127.0.0.1:2222"
		return nil
	}
	var stderr bytes.Buffer
	code := run([]string{"open", "--token", "dtc1_test", "--listen", "127.0.0.1:2222"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "listening on 127.0.0.1:2222") {
		t.Fatalf("stderr = %q, want bind address", stderr.String())
	}
}

func TestRunConnectRequiresStdio(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"connect", "--token", "dtc1_test"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--stdio") {
		t.Fatalf("stderr = %q, want stdio usage", stderr.String())
	}
}
```

- [x] **Step 2: Run CLI tests and verify they fail**

Run: `go test ./cmd/derptun -count=1`

Expected: FAIL because `token` has no role subcommands and command configs do not yet expose role-specific session fields.

- [x] **Step 3: Update root help**

In `cmd/derptun/root.go`, replace examples and descriptions:

```go
Examples: []string{
	"derptun token server --days 365 > server.dts",
	"derptun token client --token \"$(cat server.dts)\" --days 7 > client.dtc",
	"derptun serve --token \"$(cat server.dts)\" --tcp 127.0.0.1:22",
	"derptun open --token \"$(cat client.dtc)\" --listen 127.0.0.1:2222",
	"ssh -o ProxyCommand='derptun connect --token ~/.config/derptun/client.dtc --stdio' foo@serverhost",
},
```

Update subcommand descriptions:

```go
"token":   {Info: yargs.SubCommandInfo{Name: "token", Description: "Generate server credentials or client access tokens."}},
"serve":   {Info: yargs.SubCommandInfo{Name: "serve", Description: "Serve a local TCP target using a server token."}},
"open":    {Info: yargs.SubCommandInfo{Name: "open", Description: "Open a local TCP listener using a client token."}},
"connect": {Info: yargs.SubCommandInfo{Name: "connect", Description: "Connect one client tunnel stream over stdin/stdout."}},
```

- [x] **Step 4: Implement token role parsing**

Replace `cmd/derptun/token.go` with:

```go
package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/shayne/derphole/pkg/derptun"
	"github.com/shayne/yargs"
)

type tokenCommonFlags struct {
	Days    int    `flag:"days" help:"Token lifetime in days"`
	Expires string `flag:"expires" help:"Absolute expiry as RFC3339 or YYYY-MM-DD"`
}

type tokenClientFlags struct {
	Token   string `flag:"token" help:"Server token used to mint a client token"`
	Days    int    `flag:"days" help:"Token lifetime in days"`
	Expires string `flag:"expires" help:"Absolute expiry as RFC3339 or YYYY-MM-DD"`
}

var tokenHelpConfig = yargs.HelpConfig{
	Command: yargs.CommandInfo{
		Name:        "derptun",
		Description: "Generate derptun server and client tokens.",
		Examples: []string{
			"derptun token server --days 365",
			"derptun token client --token <dts1_token> --days 7",
		},
	},
	SubCommands: map[string]yargs.SubCommandInfo{
		"token": {
			Name:        "token",
			Description: "Generate a server credential or client access token.",
			Usage:       "server [--days N|--expires DATE] | client --token TOKEN [--days N|--expires DATE]",
			Examples: []string{
				"derptun token server --days 365",
				"derptun token client --token <dts1_token> --days 7",
			},
		},
	},
}

func runToken(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stderr, tokenHelpText())
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "server":
		return runTokenServer(args[1:], stdout, stderr)
	case "client":
		return runTokenClient(args[1:], stdout, stderr)
	default:
		fmt.Fprint(stderr, tokenHelpText())
		return 2
	}
}

func runTokenServer(args []string, stdout, stderr io.Writer) int {
	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, tokenCommonFlags, struct{}](append([]string{"server"}, args...), tokenHelpConfig)
	if err != nil {
		return handleTokenParseError(parsed, err, stderr)
	}
	if len(parsed.Parser.Args) != 0 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, tokenHelpText())
		return 2
	}
	expires, ok := parseOptionalTokenExpires(parsed.SubCommandFlags.Expires, stderr)
	if !ok {
		fmt.Fprint(stderr, tokenHelpText())
		return 2
	}
	tokenValue, err := derptun.GenerateServerToken(derptun.ServerTokenOptions{Days: parsed.SubCommandFlags.Days, Expires: expires})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, tokenValue)
	return 0
}

func runTokenClient(args []string, stdout, stderr io.Writer) int {
	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, tokenClientFlags, struct{}](append([]string{"client"}, args...), tokenHelpConfig)
	if err != nil {
		return handleTokenParseError(parsed, err, stderr)
	}
	if parsed.SubCommandFlags.Token == "" || len(parsed.Parser.Args) != 0 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, tokenHelpText())
		return 2
	}
	expires, ok := parseOptionalTokenExpires(parsed.SubCommandFlags.Expires, stderr)
	if !ok {
		fmt.Fprint(stderr, tokenHelpText())
		return 2
	}
	tokenValue, err := derptun.GenerateClientToken(derptun.ClientTokenOptions{
		ServerToken: parsed.SubCommandFlags.Token,
		Days:        parsed.SubCommandFlags.Days,
		Expires:     expires,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, tokenValue)
	return 0
}

func handleTokenParseError[S any](parsed *yargs.TypedParseResult[struct{}, S, struct{}], err error, stderr io.Writer) int {
	if errors.Is(err, yargs.ErrHelp) || errors.Is(err, yargs.ErrSubCommandHelp) || errors.Is(err, yargs.ErrHelpLLM) {
		if parsed != nil && parsed.HelpText != "" {
			fmt.Fprint(stderr, parsed.HelpText)
		} else {
			fmt.Fprint(stderr, tokenHelpText())
		}
		return 0
	}
	fmt.Fprintln(stderr, err)
	fmt.Fprint(stderr, tokenHelpText())
	return 2
}

func parseOptionalTokenExpires(value string, stderr io.Writer) (time.Time, bool) {
	if value == "" {
		return time.Time{}, true
	}
	expires, err := parseTokenExpires(value)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return time.Time{}, false
	}
	return expires, true
}

func parseTokenExpires(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --expires value %q; use RFC3339 or YYYY-MM-DD", value)
	}
	return t, nil
}

func tokenHelpText() string {
	return yargs.GenerateSubCommandHelp(tokenHelpConfig, "token", struct{}{}, tokenCommonFlags{}, struct{}{})
}
```

- [x] **Step 5: Keep serve/open/connect `--token` flags and wire role-specific configs**

In `cmd/derptun/serve.go`, change:

```go
type serveFlags struct {
	Token      string `flag:"token" help:"Durable derptun token"`
	TCP        string `flag:"tcp" help:"Local TCP target to expose, for example 127.0.0.1:22"`
	ForceRelay bool   `flag:"force-relay" help:"Disable direct probing"`
}
```

Leave the flag name as `--token`. Change config construction to:

```go
ServerToken: parsed.SubCommandFlags.Token,
```

In `cmd/derptun/open.go`, keep:

```go
Token string `flag:"token" help:"Client token for tunnel access"`
```

Change config construction to:

```go
ClientToken: parsed.SubCommandFlags.Token,
```

In `cmd/derptun/connect.go`, keep:

```go
Token string `flag:"token" help:"Client token for tunnel access"`
```

Change config construction to:

```go
ClientToken: parsed.SubCommandFlags.Token,
```

- [x] **Step 6: Run CLI tests**

Run: `go test ./cmd/derptun -count=1`

Expected: PASS.

- [x] **Step 7: Build CLI**

Run: `mise run build`

Expected: PASS and creates `dist/derphole` plus `dist/derptun`.

- [x] **Step 8: Commit CLI split**

```bash
git add cmd/derptun
git commit -m "feat: update derptun cli token roles"
```

### Task 5: Update Smoke Test And README After Implementation

**Files:**
- Modify: `scripts/smoke-remote-derptun.sh`
- Modify: `README.md`

Do this after the token, session, and CLI implementation tasks so the README examples match the final working commands.

- [x] **Step 1: Update remote smoke script token workflow**

In `scripts/smoke-remote-derptun.sh`, replace all-in-one token generation:

```bash
token_file="${tmp}/token"
```

with:

```bash
server_token_file="${tmp}/server.dts"
client_token_file="${tmp}/client.dtc"
```

Replace:

```bash
"${root_dir}/dist/derptun" token --days 1 > "$token_file"
```

with:

```bash
"${root_dir}/dist/derptun" token server --days 1 > "$server_token_file"
"${root_dir}/dist/derptun" token client --token "$(cat "$server_token_file")" --days 1 > "$client_token_file"
```

Replace remote upload of the old token file with upload of `server.dts`:

```bash
scp "${server_token_file}" "${remote_user}@${target}:${remote_base}/server.dts" >/dev/null
```

Replace remote serve command token usage with:

```bash
--token "$(cat '${remote_base}/server.dts')"
```

Replace every local client-side token read in the script, including `connect`, active/contender `connect`, and `open`, with:

```bash
--token "$(cat "$client_token_file")"
```

- [x] **Step 2: Run smoke script shell syntax check**

Run: `bash -n scripts/smoke-remote-derptun.sh`

Expected: PASS with no output.

- [x] **Step 3: Update README derptun section**

Use the `caveman` skill for tight README prose before editing. Replace the current `derptun` examples with:

```markdown
### Durable SSH Tunnels with `derptun`

`derptun` is the durable TCP tunnel companion to `derphole`. Use it when `serverhost` is behind NAT and you want a stable server credential that can survive process restarts.

On `serverhost`:

```bash
npx -y derptun@latest token server --days 365 > server.dts
npx -y derptun@latest token client --token "$(cat server.dts)" --days 7 > client.dtc
npx -y derptun@latest serve --token "$(cat server.dts)" --tcp 127.0.0.1:22
```

Copy only `client.dtc` to `clienthost`.

On `clienthost`:

```bash
npx -y derptun@latest open --token "$(cat client.dtc)" --listen 127.0.0.1:2222
ssh -p 2222 user@127.0.0.1
```

For SSH without a separate local listener, use `ProxyCommand`:

```sshconfig
Host serverhost-derptun
  HostName serverhost
  User foo
  ProxyCommand derptun connect --token ~/.config/derptun/client.dtc --stdio
```

The server token is secret serving authority. Keep it on the serving machine or in its secret manager. The client token can connect until it expires, but it cannot serve or mint more tokens.
```

Update the security section sentence to:

```markdown
`derphole` session tokens expire after one hour. `derptun` server tokens default to seven days and can mint shorter-lived client tokens; only server tokens can serve.
```

- [x] **Step 4: Run docs-related checks**

Run: `rg -n 'dt1_|derptun token --days|server-token|client-token|client-name' README.md cmd/derptun scripts/smoke-remote-derptun.sh`

Expected: No matches except references in tests that intentionally verify legacy rejection. If any non-test match remains, update it to the V1 `--token` workflow.

- [x] **Step 5: Commit docs and smoke update**

```bash
git add README.md scripts/smoke-remote-derptun.sh
git commit -m "docs: document derptun token roles"
```

### Task 6: Full Verification And Cleanup

**Files:**
- Verify all modified files.

- [x] **Step 1: Run focused package tests**

Run: `go test ./pkg/derptun ./pkg/rendezvous ./pkg/session ./cmd/derptun -count=1`

Expected: PASS.

- [x] **Step 2: Run full test suite through mise**

Run: `mise run test`

Expected: PASS.

- [x] **Step 3: Run vet**

Run: `mise run vet`

Expected: PASS.

- [x] **Step 4: Run build**

Run: `mise run build`

Expected: PASS and `dist/derptun` exists.

- [x] **Step 5: Run command smoke locally**

Run:

```bash
server_token="$(./dist/derptun token server --days 1)"
client_token="$(./dist/derptun token client --token "$server_token" --days 1)"
test "${server_token#dts1_}" != "$server_token"
test "${client_token#dtc1_}" != "$client_token"
```

Expected: PASS with no output.

- [x] **Step 6: Review public help text**

Run: `./dist/derptun --help`

Expected: Help examples use `token server`, `token client`, and `--token`.

Run: `./dist/derptun serve --help`

Expected: Usage includes `--token`.

Run: `./dist/derptun open --help`

Expected: Usage includes `--token`.

Run: `./dist/derptun connect --help`

Expected: Usage includes `--token`.

- [x] **Step 7: Run required live validation against `ktzlxc`**

This live validation is required before the implementation can be considered complete. The machine is accessible with `ssh root@ktzlxc`.

First, verify SSH access:

```bash
ssh root@ktzlxc 'echo derptun-live-ok'
```

Expected: prints `derptun-live-ok`.

Then run the remote derptun smoke test against that machine:

```bash
REMOTE_HOST=ktzlxc mise run smoke-remote-derptun
```

Expected: PASS. This matches the existing smoke task convention: `REMOTE_HOST` is the host name, and `DERPHOLE_REMOTE_USER` defaults to `root`. The test must exercise the new `derptun token server`, `derptun token client --token ...`, `derptun serve --token ...`, `derptun connect --token ...`, and `derptun open --token ...` workflow against `ktzlxc`.

- [x] **Step 8: Final repository check**

Run: `git status --short`

Expected: clean after commits.

Run: `git log --oneline --max-count=6`

Expected: recent commits include:

```text
docs: document derptun token roles
feat: update derptun cli token roles
feat: enforce derptun token roles in sessions
feat: carry derptun client proof in claims
feat: split derptun server and client tokens
```

## Self-Review Notes

- Spec coverage: The plan covers breaking removal of `dt1_`, server-only serving, client-only connecting, no filesystem persistence, restart durability through persisted server token, single active client, future multi-client comments, CLI workflow, docs, smoke script, and verification.
- Placeholder scan: The plan was checked for unfinished-marker language and vague implementation steps.
- Type consistency: Public names are `ServerCredential`, `ClientCredential`, `ServerTokenOptions`, `ClientTokenOptions`, `GenerateServerToken`, `GenerateClientToken`, `DecodeServerToken`, `DecodeClientToken`, `ServerTokenPrefix`, `ClientTokenPrefix`, `ServerToken`, and `ClientToken`.
