# Relay Payload Encryption Design

## Problem

The README security model says DERP relays forward encrypted session bytes, but the current one-shot relay fallback paths do not always meet that claim. Direct UDP payload packets use AEAD derived from the token bearer secret. `share/open` and `derptun` use authenticated QUIC streams. However, the one-shot DERP relay fallback and relay-prefix handoff path can put user bytes directly inside DERP frames.

DERP still uses TLS between the client and relay, so passive internet observers should not see plaintext. That is not enough for the intended security model. DERP must be treated as an untrusted relay: it may route packets and see minimal metadata, but it must not see plaintext user payload bytes after token authorization.

This is a breaking wire-format change. There is no need for compatibility with old plaintext relay frames.

## Goals

- Make every DERP-carried user-data path end-to-end encrypted and authenticated.
- Preserve current direct UDP and relay-to-direct promotion behavior.
- Keep relay metadata visible only where needed for routing, ordering, offsets, acknowledgements, EOF, and handoff.
- Fail closed on plaintext legacy relay frames, wrong keys, or tampered ciphertext.
- Update the Security Model only after tests prove the implementation.
- Include live verification against `ktzlxc` so the fix is checked against the real remote smoke environment.

## Non-Goals

- Do not redesign `share/open` or `derptun`; their QUIC/TLS transport already matches the target security model.
- Do not reintroduce the old WireGuard fallback path.
- Do not add a legacy compatibility mode for plaintext relay data.
- Do not hide all metadata from DERP. Frame type, offsets, and control signals may stay visible when needed for flow control and handoff.

## Recommended Approach

Reuse the existing one-shot session AEAD primitive. It is derived from `SessionID + BearerSecret`, which both authorized peers already have and DERP does not.

The helper currently named for direct UDP should be renamed to a transport-neutral helper such as `externalSessionPacketAEAD`. Existing direct UDP code and the new relay encryption code should both use that helper. The important property is that all one-shot user payload encryption uses the same token-bound key material.

## Architecture

### Force-Relay Probe Path

`sendExternalRelayUDP` and `receiveExternalRelayUDP` should accept the token or a precomputed AEAD. They should pass that AEAD into `probe.SendConfig.PacketAEAD` and `probe.ReceiveConfig.PacketAEAD`.

That makes force-relay use the same packet encryption path as direct UDP. Probe headers remain visible for sequencing and repair. Payload bytes are encrypted and authenticated.

### Relay-Prefix Handoff Path

Relay-prefix DERP `Data` frames should encrypt only the user payload. The frame header remains:

```text
magic || kind || offset
```

The payload becomes AEAD ciphertext. The header is used as associated data so any modification to frame kind or offset causes authentication failure.

Ack, EOF, handoff, and handoff-ack frames carry no user payload and will keep their existing payload-free shape in this fix. Additional control-frame authentication is out of scope for this change.

### Decode Behavior

Decode must fail closed:

- A plaintext legacy `Data` frame must not decode as user data.
- A frame encrypted with the wrong token must fail authentication.
- A tampered ciphertext or tampered associated-data header must fail authentication.
- There is no plaintext fallback.

Errors should abort the affected transfer instead of silently continuing.

## Data Flow

1. A token authorizes the session.
2. Both peers derive the one-shot session AEAD from `SessionID + BearerSecret`.
3. If direct UDP is available, existing AEAD packet protection remains in place.
4. If force-relay is used, probe packets carry encrypted payloads through DERP.
5. If relay-prefix handoff sends startup bytes over DERP, each `Data` frame carries ciphertext. The receiver authenticates and decrypts before writing to the output stream.
6. If direct promotion succeeds, direct UDP continues with the same end-to-end packet protection.

## Testing

Add red tests before implementation for the plaintext and tamper cases.

- Unit test that force-relay probe packets do not contain a known plaintext marker and still round-trip with the correct token.
- Unit test that relay-prefix DERP `Data` frames do not contain plaintext, decode with the correct AEAD, and reject a wrong AEAD.
- Unit test that tampering with relay-prefix ciphertext or associated-data header fails.
- Regression test that a plaintext legacy relay-prefix `Data` frame fails closed.
- Session-level test that captured DERP relay-prefix data frames never contain a transmitted marker while the receiver reconstructs the original bytes.
- Existing package tests should continue to pass.

Live verification must include `ktzlxc` and must not route over Tailscale:

- Run remote smoke against `ktzlxc` for normal transfer behavior using the non-Tailscale route.
- Run a forced-relay live transfer against `ktzlxc` and confirm it succeeds.
- Capture or instrument relay data for a known marker during the forced-relay path and confirm DERP-carried user-data frames do not contain that marker in plaintext.
- Run a non-forced live transfer against `ktzlxc` with verbose path logging and confirm it can promote to direct when the environment permits direct UDP.

## Documentation

After implementation and verification, update `README.md` Security Model to say:

- Tokens authorize sessions.
- DERP relays can see routing/session metadata needed to move packets.
- DERP relays do not receive plaintext user payload bytes.
- Direct UDP and relay fallback both encrypt and authenticate user payloads with token-derived session keys.
- `share/open` and `derptun` continue to use authenticated QUIC streams.

## Acceptance Criteria

- All one-shot user bytes sent through DERP are encrypted and authenticated end-to-end.
- DERP relay paths never require DERP to hold decrypt keys.
- Legacy plaintext relay-prefix frames fail closed.
- Unit and session tests cover direct, force-relay, relay-prefix, tamper, and wrong-key cases.
- Live `ktzlxc` smoke and forced-relay verification pass over the non-Tailscale route.
- README Security Model accurately describes the implemented behavior.
