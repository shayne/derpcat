# derpcat

`derpcat` is a standalone Go CLI for moving one bidirectional byte stream between two hosts using the public Tailscale DERP network for bootstrap and relay fallback, with direct UDP promotion when possible.

## Planned npm usage

The npm package and release workflow are being added separately. The commands below describe the intended public interface once packaging lands.

### Planned production install

```bash
npx derpcat --version
```

### Planned dev channel install

```bash
npx derpcat@dev --version
```

## Build

```bash
mise run build
```

## Test

```bash
mise run test
mise run smoke-local
```

## Release plan

`main` is intended to publish the `dev` npm dist-tag.
Version tags like `v0.1.0` are intended to publish production releases.
The first `0.0.1` npm publish is intended to be performed manually from `dist/npm`.
