# Derpcat Pre-Commit Checks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `pre-commit`-managed local git hooks plus a CI workflow that enforces the same repository checks in `derpcat`.

**Architecture:** Reuse the `viberun` structure: `mise` installs tools, `pre-commit` orchestrates hook execution, small shell scripts under `tools/hooks/` hold repository-specific checks, and a dedicated `checks.yml` workflow runs the same `mise run check` entrypoint in CI. The hook set is intentionally narrower than `viberun` and only covers checks that make sense for this Go CLI repository.

**Tech Stack:** `mise`, `pre-commit`, `staticcheck`, Bash, GitHub Actions, Go toolchain

---

## File Map

- Modify: `/Users/shayne/code/derpcat/.mise.toml`
- Create: `/Users/shayne/code/derpcat/.pre-commit-config.yaml`
- Create: `/Users/shayne/code/derpcat/tools/hooks/gofmt-check`
- Create: `/Users/shayne/code/derpcat/tools/hooks/go-vet`
- Create: `/Users/shayne/code/derpcat/tools/hooks/go-mod-tidy-check`
- Create: `/Users/shayne/code/derpcat/tools/hooks/staticcheck`
- Create: `/Users/shayne/code/derpcat/tools/hooks/prepare-commit-msg`
- Create: `/Users/shayne/code/derpcat/.github/workflows/checks.yml`
- Modify: `/Users/shayne/code/derpcat/README.md`
- Modify: `/Users/shayne/code/derpcat/AGENTS.md`

### Task 1: Add `mise` Tooling And Check Entry Points

**Files:**
- Modify: `/Users/shayne/code/derpcat/.mise.toml`

- [ ] **Step 1: Verify the current `check` entrypoint is missing**

Run:

```bash
mise run check
```

Expected: failure with a message equivalent to `task not found: check`.

- [ ] **Step 2: Add `pre-commit` and `staticcheck` plus new tasks**

Update `/Users/shayne/code/derpcat/.mise.toml` to add:

```toml
[tools]
go = "1.26.1"
node = "24"
pre-commit = "latest"
staticcheck = "latest"
```

Add tasks:

```toml
[tasks."check:hooks"]
run = "pre-commit run --all-files"

[tasks.check]
shell = "bash -c"
run = """
set -euo pipefail
mise run check:hooks
mise run build
mise run test
"""

[tasks.install-githooks]
description = "Install git hooks via pre-commit"
run = '''
#!/usr/bin/env bash
set -euo pipefail

git config --local --unset-all core.hooksPath || true

pre-commit install --install-hooks \
  --hook-type pre-commit \
  --hook-type prepare-commit-msg
'''
```

- [ ] **Step 3: Verify the new tasks are registered**

Run:

```bash
mise tasks ls | rg "check|install-githooks"
```

Expected output includes:

```text
check
check:hooks
install-githooks
```

- [ ] **Step 4: Commit**

```bash
git add /Users/shayne/code/derpcat/.mise.toml
git commit -m "build: add pre-commit tooling tasks"
```

### Task 2: Add Pre-Commit Configuration And Hook Scripts

**Files:**
- Create: `/Users/shayne/code/derpcat/.pre-commit-config.yaml`
- Create: `/Users/shayne/code/derpcat/tools/hooks/gofmt-check`
- Create: `/Users/shayne/code/derpcat/tools/hooks/go-vet`
- Create: `/Users/shayne/code/derpcat/tools/hooks/go-mod-tidy-check`
- Create: `/Users/shayne/code/derpcat/tools/hooks/staticcheck`
- Create: `/Users/shayne/code/derpcat/tools/hooks/prepare-commit-msg`

- [ ] **Step 1: Confirm `pre-commit` is not yet configured**

Run:

```bash
mise exec -- pre-commit run --all-files
```

Expected: failure with a message equivalent to `No .pre-commit-config.yaml file was found`.

- [ ] **Step 2: Add the local-hook configuration**

Create `/Users/shayne/code/derpcat/.pre-commit-config.yaml`:

```yaml
repos:
  - repo: local
    hooks:
      - id: derpcat-gofmt-check
        name: derpcat-gofmt-check
        entry: tools/hooks/gofmt-check
        language: script
        files: \.go$
      - id: derpcat-go-vet
        name: derpcat-go-vet
        entry: tools/hooks/go-vet
        language: script
        pass_filenames: false
      - id: derpcat-go-mod-tidy
        name: derpcat-go-mod-tidy
        entry: tools/hooks/go-mod-tidy-check
        language: script
        pass_filenames: false
      - id: derpcat-staticcheck
        name: derpcat-staticcheck
        entry: tools/hooks/staticcheck
        language: script
        pass_filenames: false
      - id: derpcat-prepare-commit-msg
        name: derpcat-prepare-commit-msg
        entry: tools/hooks/prepare-commit-msg
        language: script
        stages: [prepare-commit-msg]
        pass_filenames: true
```

- [ ] **Step 3: Add the hook scripts adapted from `viberun`**

Create `/Users/shayne/code/derpcat/tools/hooks/gofmt-check`:

```bash
#!/usr/bin/env bash
set -euo pipefail

files="$(git diff --cached --name-only --diff-filter=ACMR | rg '\.go$' || true)"
if [ -z "$files" ]; then
  files="$(git ls-files '*.go')"
fi

[ -z "$files" ] && exit 0

gofmt -w $files

if ! git diff --quiet -- $files; then
  echo "gofmt made changes; re-stage formatted files" >&2
  exit 1
fi
```

Create `/Users/shayne/code/derpcat/tools/hooks/go-vet`:

```bash
#!/usr/bin/env bash
set -euo pipefail

go vet ./...
```

Create `/Users/shayne/code/derpcat/tools/hooks/go-mod-tidy-check`:

```bash
#!/usr/bin/env bash
set -euo pipefail

go mod tidy

if ! git diff --quiet -- go.mod go.sum; then
  echo "go mod tidy changed go.mod or go.sum" >&2
  exit 1
fi
```

Create `/Users/shayne/code/derpcat/tools/hooks/staticcheck`:

```bash
#!/usr/bin/env bash
set -euo pipefail

if ! command -v mise >/dev/null 2>&1; then
  echo "mise is required to run staticcheck" >&2
  exit 1
fi

mise exec -- staticcheck ./...
```

Create `/Users/shayne/code/derpcat/tools/hooks/prepare-commit-msg`:

```bash
#!/usr/bin/env bash
set -euo pipefail

if command -v uvx >/dev/null 2>&1; then
  exec uvx cmtr@latest prepare-commit-msg "$@"
fi
```

Then make them executable:

```bash
chmod +x \
  /Users/shayne/code/derpcat/tools/hooks/gofmt-check \
  /Users/shayne/code/derpcat/tools/hooks/go-vet \
  /Users/shayne/code/derpcat/tools/hooks/go-mod-tidy-check \
  /Users/shayne/code/derpcat/tools/hooks/staticcheck \
  /Users/shayne/code/derpcat/tools/hooks/prepare-commit-msg
```

- [ ] **Step 4: Verify hook configuration and hook execution**

Run:

```bash
mise exec -- pre-commit validate-config
mise run check:hooks
```

Expected: both commands pass.

- [ ] **Step 5: Commit**

```bash
git add \
  /Users/shayne/code/derpcat/.pre-commit-config.yaml \
  /Users/shayne/code/derpcat/tools/hooks
git commit -m "build: add repository pre-commit hooks"
```

### Task 3: Add CI Enforcement

**Files:**
- Create: `/Users/shayne/code/derpcat/.github/workflows/checks.yml`

- [ ] **Step 1: Confirm the repo has no dedicated checks workflow**

Run:

```bash
test ! -f /Users/shayne/code/derpcat/.github/workflows/checks.yml
```

Expected: success with no output.

- [ ] **Step 2: Add a CI workflow that runs the shared `check` task**

Create `/Users/shayne/code/derpcat/.github/workflows/checks.yml`:

```yaml
name: Checks

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  checks:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v5
      - name: Setup mise
        uses: jdx/mise-action@v4
        with:
          install: true
          cache: true
      - name: Run repository checks
        run: mise run check
```

- [ ] **Step 3: Validate the workflow file is structurally sound**

Run:

```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/checks.yml")'
git diff --check
```

Expected: both commands exit successfully.

- [ ] **Step 4: Commit**

```bash
git add /Users/shayne/code/derpcat/.github/workflows/checks.yml
git commit -m "ci: add repository checks workflow"
```

### Task 4: Update Contributor-Facing Documentation

**Files:**
- Modify: `/Users/shayne/code/derpcat/README.md`
- Modify: `/Users/shayne/code/derpcat/AGENTS.md`

- [ ] **Step 1: Add local hook setup and check commands to the docs**

Update `/Users/shayne/code/derpcat/README.md` build and test sections to include:

```md
## Development

```bash
mise install
mise run install-githooks
mise run check
```
```

Update `/Users/shayne/code/derpcat/AGENTS.md` command lists to include:

```md
- `mise run install-githooks` installs the local `pre-commit` and `prepare-commit-msg` hooks
- `mise run check:hooks` runs the full `pre-commit` hook set across the repo
- `mise run check` runs hooks, build, and tests in the same order CI uses
```

- [ ] **Step 2: Verify the docs mention the new entrypoints**

Run:

```bash
rg -n "install-githooks|check:hooks|mise run check" README.md AGENTS.md
```

Expected: matches in both files.

- [ ] **Step 3: Commit**

```bash
git add /Users/shayne/code/derpcat/README.md /Users/shayne/code/derpcat/AGENTS.md
git commit -m "docs: document repository checks workflow"
```

### Task 5: End-To-End Verification And Final Cleanup

**Files:**
- Modify: `/Users/shayne/code/derpcat/.mise.toml`
- Modify: `/Users/shayne/code/derpcat/.pre-commit-config.yaml`
- Modify: `/Users/shayne/code/derpcat/tools/hooks/*`
- Modify: `/Users/shayne/code/derpcat/.github/workflows/checks.yml`
- Modify: `/Users/shayne/code/derpcat/README.md`
- Modify: `/Users/shayne/code/derpcat/AGENTS.md`

- [ ] **Step 1: Install hooks and verify they are present**

Run:

```bash
mise run install-githooks
ls -l .git/hooks/pre-commit .git/hooks/prepare-commit-msg
```

Expected: both hook files exist and are executable.

- [ ] **Step 2: Run the full local verification stack**

Run:

```bash
mise run check:hooks
mise run build
mise run test
mise run check
```

Expected: all commands pass.

- [ ] **Step 3: Confirm only intended files changed**

Run:

```bash
git status --short
```

Expected: only the planned `mise`, hook, workflow, and documentation files are listed.

- [ ] **Step 4: Final commit**

```bash
git add \
  /Users/shayne/code/derpcat/.mise.toml \
  /Users/shayne/code/derpcat/.pre-commit-config.yaml \
  /Users/shayne/code/derpcat/tools/hooks \
  /Users/shayne/code/derpcat/.github/workflows/checks.yml \
  /Users/shayne/code/derpcat/README.md \
  /Users/shayne/code/derpcat/AGENTS.md
git commit -m "build: add pre-commit checks and ci"
```

## Self-Review

- Spec coverage: local hooks, `mise` integration, CI enforcement, and docs are all covered by Tasks 1 through 4; full verification is covered by Task 5.
- Placeholder scan: no `TODO`, `TBD`, or undefined “write tests later” steps remain.
- Type consistency: task names, file paths, and command names are consistent with the approved design (`check`, `check:hooks`, `install-githooks`).
