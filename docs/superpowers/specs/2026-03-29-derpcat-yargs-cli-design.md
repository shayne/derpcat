# Derpcat Yargs CLI Design

## Summary

`derpcat` should replace its current hand-rolled root parsing plus `flag.FlagSet`
subcommand parsing with `github.com/shayne/yargs`, following the command-tree
pattern used effectively in `~/code/yeet`.

This is a parsing and help-system migration, not a transport redesign. The
session behavior, transport behavior, and flag surface should stay substantially
the same unless a change is clearly justified by the `yargs` model.

## Goals

- Replace manual CLI parsing with typed `yargs` parsing.
- Move `derpcat` to a proper subcommand-oriented command tree.
- Improve root and subcommand help output with explicit descriptions and
  examples.
- Preserve the existing `listen` and `send` operational behavior.
- Keep global verbosity flags available before subcommands.

## Non-Goals

- Changing `derpcat` session semantics.
- Renaming `listen` or `send`.
- Adding new transport modes or new networking behavior.
- Preserving byte-for-byte compatibility with current help or error text.

## Current State

Today the CLI is split across:

- `/Users/shayne/code/derpcat/cmd/derpcat/root.go`
- `/Users/shayne/code/derpcat/cmd/derpcat/listen.go`
- `/Users/shayne/code/derpcat/cmd/derpcat/send.go`
- `/Users/shayne/code/derpcat/cmd/derpcat/version.go`

Problems with the current structure:

- root parsing is manual and special-cased
- subcommand help text is handwritten
- `listen` and `send` use `flag.FlagSet`, which makes help and parsing behavior
  inconsistent with the desired `yargs` style
- tests are tightly coupled to exact usage strings

## Desired CLI Surface

The root command should expose:

- `derpcat listen`
- `derpcat send <token>`
- `derpcat version`

The root command should continue to support these global flags:

- `-v`, `--verbose`
- `-q`, `--quiet`
- `-s`, `--silent`

The subcommand-local flags should remain:

### `listen`

- `--print-token-only`
- `--force-relay`
- `--tcp-listen <addr>`
- `--tcp-connect <addr>`

### `send`

- positional `<token>`
- `--force-relay`
- `--tcp-listen <addr>`
- `--tcp-connect <addr>`

The legacy root `--version` flag should be removed in favor of the `version`
subcommand. Breaking this is acceptable.

## Parsing Model

The migration should follow the `yeet` pattern:

### 1. Root global parsing

Use `yargs.ParseKnownFlags` for root-global flags so the parser can resolve:

- verbosity flags
- remaining args for subcommand dispatch

The root should define a typed struct, for example:

- `rootFlagsParsed`

This phase should only parse global flags and leave subcommand arguments
unconsumed.

### 2. Root command dispatch

Use a `yargs` command tree for dispatch and help configuration.

The root should define:

- a `yargs.HelpConfig`
- a subcommand handler map

No command groups are required for `derpcat`; the tree is small enough to use
plain subcommands only.

### 3. Subcommand-local parsing

Each handler should parse its own local flags using typed structs and
`yargs.ParseFlags`.

Recommended types:

- `listenFlagsParsed`
- `sendFlagsParsed`

This should replace all `flag.NewFlagSet` usage in:

- `/Users/shayne/code/derpcat/cmd/derpcat/listen.go`
- `/Users/shayne/code/derpcat/cmd/derpcat/send.go`

### 4. Validation

Business validation should remain explicit after parsing:

- `--tcp-listen` and `--tcp-connect` remain mutually exclusive
- `send` requires exactly one token positional
- `listen` takes no positional arguments

These validations should continue to map to the current exit-code model:

- parse or usage problems -> `2`
- runtime failures -> `1`
- success -> `0`

## Help And Examples

The migration should improve help quality, not just change formatting.

The root help should include:

- a concise description of what `derpcat` does
- short examples for `listen`, `send`, and `version`

Examples to include at minimum:

- `derpcat listen`
- `cat file | derpcat send <token>`
- `derpcat listen --tcp-connect 127.0.0.1:9000`
- `derpcat send <token> --tcp-listen 127.0.0.1:7000`
- `derpcat version`

Subcommand help should include:

- one-line descriptions
- usage strings
- representative examples for `stdio` mode and TCP bridging mode

## File-Level Design

### `/Users/shayne/code/derpcat/cmd/derpcat/root.go`

Should become the root command orchestrator:

- parse global flags with `yargs.ParseKnownFlags`
- resolve telemetry level
- build the `yargs.HelpConfig`
- dispatch to `listen`, `send`, and `version` handlers

### `/Users/shayne/code/derpcat/cmd/derpcat/listen.go`

Should keep the runtime behavior but replace `flag.FlagSet` with:

- a typed parsed flag struct
- `yargs.ParseFlags`
- `yargs`-driven help and usage integration

### `/Users/shayne/code/derpcat/cmd/derpcat/send.go`

Should do the same for `send`, including positional token handling through the
parsed result.

### `/Users/shayne/code/derpcat/cmd/derpcat/version.go`

Should remain simple but be exposed through a `version` subcommand handler.

## Testing Strategy

The tests should be updated to validate behavior rather than exact old usage
strings.

Keep coverage for:

- missing subcommand behavior
- unknown subcommand behavior
- root help behavior
- `version` subcommand behavior
- `listen` and `send` dispatch behavior
- global verbosity flags before subcommands
- mutual exclusion for `--tcp-listen` and `--tcp-connect`

Tests should explicitly stop asserting the exact old handwritten usage text
where `yargs` formatting is expected to change.

At least one test should prove that:

- `derpcat -q listen ...`
- `derpcat -s send ...`
- `derpcat -v listen ...`

still apply verbosity correctly after the root parser migration.

## Acceptance Criteria

- `flag.FlagSet` is no longer used by the `derpcat` CLI entrypoint.
- `derpcat version` works and `--version` is removed.
- Root and subcommand help come from the `yargs` configuration.
- `listen` and `send` keep their existing operational flag surface unless a
  justified `yargs`-specific improvement is made.
- Tests pass with updated expectations for the new help and error formatting.
