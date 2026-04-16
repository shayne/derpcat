# `derphole` single-product design

## Context

The repository is moving to one public product: `derphole`. This is a breaking change. The previous lower-level CLI, npm package, release assets, module path, package metadata, documentation, environment variables, and user-facing examples are retired in place through normal forward commits. Git history is not rewritten, commits are not removed, and old releases remain historical artifacts outside the source tree.

The current `derphole` CLI already owns human-oriented text, file, directory, browser, and SSH invite workflows. The retiring CLI still owns raw stdin/stdout byte streaming, temporary TCP service sharing, profiling hooks, benchmark naming, and much of the dual-product release and npm pipeline. Those useful features should move into `derphole` so there is no product split.

## Goals

`derphole` becomes the only user-facing binary, npm package, GitHub repository name, documentation subject, and release artifact prefix.

The codebase should have no source-tree references to the retired name after the migration, including historical planning documents under `docs/superpowers/`. A clean verification command should be able to prove that with:

```bash
rg -i "retired-name-literal" -g '!dist'
```

The existing structured transfer flows stay intact:

- `derphole send [what]` sends text, a file, a directory, or stdin as a structured transfer.
- `derphole receive [code]` receives structured transfers.
- `derphole tx` remains an alias for `send`.
- `derphole rx`, `derphole recv`, and `derphole recieve` remain aliases for `receive`.
- `derphole ssh invite` and `derphole ssh accept` remain SSH invite workflows.

The retired lower-level features move forward as first-class `derphole` commands:

- `derphole listen` receives one raw byte stream to stdout.
- `derphole pipe <token>` sends stdin as one raw byte stream.
- `derphole share <addr>` exposes a local TCP service.
- `derphole open <token> [bind]` opens a shared service locally.

The npm release surface becomes one package: `derphole`.

## Non-Goals

This migration will not preserve command aliases using the retired binary or package name. The requested result is a clean single-product surface, so compatibility shims are intentionally out of scope.

This migration will not rewrite Git history, delete existing commits, or mutate past release records. It is a forward-only source change.

This migration will not replace the existing session, relay, direct UDP, direct TCP, WebRTC, or web relay internals with a new transport stack. It only renames and consolidates product surfaces while preserving proven transport behavior.

## CLI Design

`derphole send` keeps its current structured meaning because it is already the main user-facing workflow. Reusing `send` for raw stdin would create an avoidable conflict, so raw stream sending uses the new `pipe` command.

The root command registry should expose this command set:

```text
derphole send [what]
derphole receive [code]
derphole listen
derphole pipe <token>
derphole share <addr>
derphole open <token> [bind]
derphole ssh invite
derphole ssh accept <token>
derphole version
```

`listen`, `pipe`, `share`, and `open` should keep the same behavior and flags they had before migration, with names and help text updated:

- `listen --print-token-only`
- `listen --force-relay`
- `pipe --force-relay`
- `pipe --parallel <n|auto>`
- `share --print-token-only`
- `share --force-relay`
- `open --force-relay`
- `open --parallel <n|auto>`

`pipe` should read only from stdin and should not interpret payloads as structured `derphole` transfer headers. `listen` should write raw bytes to stdout and should not create files or directories. The structured `send` and `receive` commands remain responsible for transfer metadata, progress, output path resolution, and browser/WebRTC receive paths.

Unknown command, help, `--help-llm`, and verbosity behavior should remain consistent with the existing CLI style, but all examples and messages must name `derphole`.

## Code Organization

The Go module path changes to:

```text
github.com/shayne/derphole
```

All internal imports should use that module path.

`cmd/derphole` becomes the only main native CLI. The retiring CLI directory should be removed after its useful command implementations and tests are moved or rewritten under `cmd/derphole`.

The probe tool should be renamed so it does not carry the retired product name. The preferred target is:

```text
cmd/derphole-probe
```

Probe package code can remain under `pkg/probe` because that package name describes its function and is not product-branded.

Existing shared packages such as `pkg/session`, `pkg/transport`, `pkg/quicpath`, `pkg/derpbind`, `pkg/derphole`, and web subpackages should remain in place unless a name contains the retired product literal. Behavior should be preserved unless tests expose a rename-specific issue.

## Environment Variables and Wire Names

Public and test environment variables should move to the `DERPHOLE_` prefix. Examples include profiling, test relay selection, DERP overrides, direct transport flags, qlog, metrics, benchmark, and probe variables.

Internal strings that include the retired product literal should be renamed to equivalent `derphole` strings where safe:

- qlog and metrics filenames
- temporary file prefixes
- QUIC ALPN and server name
- transport probe payloads
- crypto domain labels
- in-memory bus client names

Because this is a breaking change, old environment variable aliases are not required. Tests and scripts should move directly to the new names.

Wire protocol constants already branded as `DERPHOLE` should stay as they are unless tests identify inconsistent naming.

## Release and Packaging

Build tasks should produce only `dist/derphole` for the native CLI. Cross-build tasks should produce only `dist/derphole-linux-amd64` and equivalent target artifacts.

Release packaging should emit only these native binary tarballs:

```text
derphole-linux-amd64.tar.gz
derphole-linux-arm64.tar.gz
derphole-darwin-amd64.tar.gz
derphole-darwin-arm64.tar.gz
```

The web artifact remains `derphole-web.zip`.

The npm staging tree should build only:

```text
dist/npm-derphole
```

The `derphole` npm manifest should point at the renamed GitHub repository URL. The retired package template directory should be removed.

GitHub Actions release jobs should build, verify, dry-run publish, and publish only the `derphole` npm package. The npm environment URL should point at the `derphole` package page.

## GitHub Repository Rename

The GitHub repository should be renamed forward with:

```bash
gh repo rename derphole --yes
```

After the remote rename, the local `origin` URL should be updated to the new repository path. The module path, package metadata, README, release runbooks, and workflow references should all match the renamed repository.

## Documentation

The README should become a `derphole` README and describe one product. It should document:

- structured send/receive text, file, directory, browser, and SSH flows
- raw byte stream piping with `listen` and `pipe`
- temporary TCP service sharing with `share` and `open`
- transport model, security model, and performance claims using only `derphole` terminology
- single npm package installation and usage
- single release artifact set

Release docs, benchmark docs, security docs, scripts, and historical planning docs should be edited so the retired product literal no longer appears in the source tree. Where history would be awkward to rewrite in prose, use neutral phrasing such as "the previous lower-level CLI" or rewrite the document around the current `derphole` product.

## Testing and Verification

Focused tests should be added or moved before implementation for the migrated commands:

- root help lists `listen`, `pipe`, `share`, and `open`
- `derphole listen --help` shows updated help
- `derphole pipe --help` shows updated help
- `derphole share --help` shows updated help
- `derphole open --help` shows updated help
- raw command wrappers call the existing session APIs with the expected flags
- structured `send` and `receive` tests continue to pass
- release packaging scripts produce only `derphole` npm and binary artifacts

The final verification set should include:

```bash
go test ./cmd/derphole ./pkg/derphole ./pkg/session -count=1
mise run build
mise run test
VERSION=v0.0.1 COMMIT=$(git rev-parse HEAD) BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) mise run release:build-all
VERSION=v0.0.1-dev.$(date -u +%Y%m%d%H%M%S) mise run release:npm-dry-run
rg -i "retired-name-literal" -g '!dist'
```

The final `rg` command should be run with the actual retired product literal during implementation. It should return no matches.

## Rollout Notes

This is intentionally a breaking release. Release notes should state that the project is now `derphole`, that the old package is no longer published by this repository, and that raw stream users should move from the old `send` command to `derphole pipe`.

Existing users with old npm installs or old binary downloads are not migrated in place. The new supported path is to install and use `derphole`.
