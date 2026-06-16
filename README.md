# jtp

`jtp` is a Jujutsu (`jj`) workspace management fork of
[`wtp`](https://github.com/satococoa/wtp).

## Origin and Credit

This project exists because of the original
[`wtp` / Worktree Plus](https://github.com/satococoa/wtp) project by
[Satoshi Ebisawa](https://github.com/satococoa). The command structure,
configuration model, hook system, shell integration, tests, release scaffolding,
and most of the product thinking come from that upstream project.

The original project is MIT licensed, and its copyright notice is preserved in
[`LICENSE`](LICENSE). This fork should be understood as an adaptation of `wtp`
for Jujutsu workflows, not as a from-scratch replacement. Please credit and
support the upstream project whenever this fork is useful.

A powerful Jujutsu workspace management tool with automated setup, bookmark
tracking, and project-specific hooks.

## Features - Why jtp Instead of git-worktree?

### 🚀 No More Path Gymnastics

**🧩 Problem:**
`git worktree add ../project-worktrees/feature/auth feature/auth`<br>
**✨ jtp:** `jtp add feature/auth`

jtp automatically generates sensible paths based on branch names. Your
`feature/auth` branch goes to `../worktrees/feature/auth` - no redundant typing,
no path errors.

### 🧹 Clean Branch Management

**🧩 Problem:** Remove worktree, then manually delete the branch. Forget
the second step? Orphaned branches accumulate.<br>
**✨ jtp:**
`jtp remove --with-branch feature/done` - One command removes both

Keep your repository clean. When a feature is truly done, remove both the
worktree and its branch in one atomic operation. No more forgotten branches
cluttering your repo.

### 🛠️ Zero-Setup Development Environments

**🧩 Problem:** Create worktree → Copy .env → Install deps → Run
migrations → Finally start coding<br>
**✨ jtp:** Configure once in
`.wtp.yml`, then every `jtp add` runs your setup automatically

```yaml
hooks:
  post_create:
    # Copy real files from the MAIN worktree into the NEW worktree
    - type: copy
      from: ".env" # Allowed even if gitignored. 'from' is always relative to the MAIN worktree
      to: ".env" # Destination is relative to the NEW worktree

    # Share directories between the MAIN and NEW worktree
    - type: symlink
      from: ".bin"
      to: ".bin"

    # Prefer explicit, single-step setup commands
    - type: command
      command: "npm ci" # Example for Node.js (replace with your build/deps tool)
    - type: command
      command: "npm run db:setup"
    # Alternative: using make or a task runner
    # - type: command
    #   command: "make bootstrap"
```

Perfect for microservices, monorepos, or any project with complex setup
requirements.

### 📍 Instant Worktree Navigation

**🧩 Problem:** `cd ../../../worktrees/feature/auth` (if you remember the
path)<br>
**✨ jtp:** `jtp cd feature/auth` with tab completion

Jump between worktrees instantly. Use `jtp cd @` to return to your main
worktree (or just `jtp cd`). No more terminal tab confusion.

## Requirements

- Jujutsu (`jj`)
- Git, for Git-backed repositories and remotes
- One of the following operating systems:
  - Linux (x86_64 or ARM64)
  - macOS (Apple Silicon M1/M2/M3)
- One of the following shells (for completion support):
  - Bash (4+/5.x) with bash-completion v2
  - Zsh
  - Fish

## Releases

Release binaries have not been published for `jtp` yet. When they are
available, they will be listed on
[GitHub Releases](https://github.com/huntertdiamond/jtp/releases).

## Installation

### From Source

```bash
git clone https://github.com/huntertdiamond/jtp.git
cd jtp
go tool task build
install -m 0755 ./jtp ~/bin/jtp
```

Make sure `~/bin` is on your `PATH`.

### Manual Go Build

```bash
git clone https://github.com/huntertdiamond/jtp.git
cd jtp
go build -o jtp ./cmd/wtp
install -m 0755 ./jtp ~/bin/jtp
```

### Download Binary

When release binaries are published, download the latest binary from
[GitHub Releases](https://github.com/huntertdiamond/jtp/releases):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/huntertdiamond/jtp/releases/latest/download/jtp_Darwin_arm64.tar.gz | tar xz
sudo mv jtp /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/huntertdiamond/jtp/releases/latest/download/jtp_Linux_x86_64.tar.gz | tar xz
sudo mv jtp /usr/local/bin/

# Linux (ARM64)
curl -L https://github.com/huntertdiamond/jtp/releases/latest/download/jtp_Linux_arm64.tar.gz | tar xz
sudo mv jtp /usr/local/bin/
```

## Quick Start

### Automatic Path Generation (Recommended)

```bash
# Create worktree from existing branch (local or remote)
# → Creates worktree at ../worktrees/feature/auth
# Automatically tracks remote branch if not found locally
jtp add feature/auth

# Create worktree with new branch
# → Creates worktree at ../worktrees/feature/new-feature
jtp add -b feature/new-feature

# Create new branch from specific commit
# → Creates worktree at ../worktrees/hotfix/urgent
jtp add -b hotfix/urgent abc1234

# Create worktree and run a command inside it after hooks
# → Useful for bootstrap steps (supports interactive commands when TTY is available)
jtp add -b feature/new-feature --exec "npm test"

# Script-friendly output: print only the created absolute path
jtp add -b feature/new-feature --quiet

# Create new branch tracking a different remote branch
# → Creates worktree at ../worktrees/feature/test with branch tracking origin/main
jtp add -b feature/test origin/main

# Remote branch handling examples:

# Automatically tracks remote branch if not found locally
# → Creates worktree tracking origin/feature/remote-only
jtp add feature/remote-only

# If branch exists in multiple remotes, shows helpful error:
# Error: branch 'feature/shared' exists in multiple remotes: origin, upstream
# Create a local branch for the remote you want, then run jtp add again
jtp add feature/shared

# Example manual disambiguation:
git branch --track feature/shared upstream/feature/shared
jtp add feature/shared
```

### Management Commands

```bash
# List all worktrees
jtp list

# Example output:
# PATH                      BRANCH           HEAD
# ----                      ------           ----
# @ (main worktree)*        main             c72c7800
# feature/auth              feature/auth     def45678
# ../project-hotfix         hotfix/urgent    abc12345

# Remove worktree only (by worktree name)
jtp remove feature/auth
jtp remove --force feature/auth  # Force removal even if dirty

# Remove worktree and its branch
jtp remove --with-branch feature/auth              # Only if branch is merged
jtp remove --with-branch --force-branch feature/auth  # Force branch deletion

# Execute a command in an existing worktree (uses same target resolution as `jtp cd`)
jtp exec feature/auth -- go test ./...
jtp exec @ -- pwd
```

## Configuration

jtp uses `.wtp.yml` for project-specific configuration:

```yaml
version: "1.0"
defaults:
  # Base directory for worktrees (relative to project root)
  base_dir: "../worktrees"

hooks:
  post_create:
    # Copy gitignored files from main worktree to new worktree
    # Note: 'from' is relative to main worktree, 'to' is relative to new worktree
    # If 'to' is omitted, it defaults to the same value as 'from' (relative paths only)
    - type: copy
      from: ".env" # Copy actual .env file (gitignored)
      to: ".env"

    - type: copy
      from: ".claude" # Copy AI context file (gitignored)

    # Share directories between the main and new worktree
    - type: symlink
      from: ".bin"
      to: ".bin"

    # Execute commands in the new worktree
    - type: command
      command: "npm install"
      env:
        NODE_ENV: "development"

    - type: command
      command: "make db:setup"
      work_dir: "."
```

### Copy Hooks: Main Worktree Reference

Copy hooks are designed to help you bootstrap new worktrees using files from
your main worktree (even if they are gitignored):

- `from`: path is always resolved relative to the main worktree.
- `to`: path is resolved relative to the newly created worktree (defaults to `from` if omitted; absolute `from` requires explicit `to`).
- Supports files and directories, including entries ignored by Git (e.g.,
  `.env`, `.claude`, `.cursor/`).

Examples:

```yaml
hooks:
  post_create:
    # Copy local env and AI context from MAIN worktree into the new worktree
    - type: copy
      from: ".env"
      to: ".env"

    - type: copy
      from: ".claude"

    # Directory copy also works
    - type: copy
      from: ".cursor/"
      to: ".cursor/"
```

This behavior applies regardless of where you run `jtp add` from (main worktree
or any other worktree).

### Symlink Hooks: Shared Assets

Symlink hooks are useful for sharing large or mutable directories from the main
worktree (e.g. `.bin`, `.cache`, `node_modules`).

- `from`: path is resolved relative to the main worktree (or absolute).
- `to`: path is resolved relative to the newly created worktree (or absolute).

Example:

```yaml
hooks:
  post_create:
    - type: symlink
      from: ".bin"
      to: ".bin"
```

## Shell Integration

### Tab Completion Setup

#### Shell setup

After installing `jtp`, add a single line to your shell configuration file to
enable both completion and shell integration:

```bash
# Bash: Add to ~/.bashrc or ~/.bash_profile
eval "$(jtp shell-init bash)"

# Zsh: Add to ~/.zshrc
eval "$(jtp shell-init zsh)"

# Fish: Add to ~/.config/fish/config.fish
jtp shell-init fish | source
```

> **Note:** Bash completion requires bash-completion v2. On macOS, install
> Homebrew’s Bash 5.x and `bash-completion@2`, then
> `source /opt/homebrew/etc/profile.d/bash_completion.sh` (or the path shown
> after installation) before enabling the one-liner above.

Reload your shell after adding the line.

### Navigation with jtp cd

The `jtp cd` command outputs the absolute path to a worktree. You can use it in
two ways:

#### Direct Usage

```bash
# Change to a worktree using command substitution
cd "$(jtp cd feature/auth)"

# Change to the main worktree
cd "$(jtp cd)"

# Or explicitly:
cd "$(jtp cd @)"
```

#### With Shell Hook (Recommended)

For a more seamless experience, enable the shell hook. `jtp shell-init <shell>`
already bundles it. If you only want the hook without completions, you can still
run `jtp hook <shell>` manually.

Then use the simplified syntax:

```bash
# Change to a worktree by its name
jtp cd feature/auth

# Go to the main worktree (same as @)
jtp cd

# Change to the root worktree using the '@' shorthand
jtp cd @

# Tab completion works!
jtp cd <TAB>

# Create a worktree and switch to it automatically (interactive shell only)
jtp add -b feature/payment
```

When stdout is not a TTY (for example command substitution or pipes), `jtp add`
keeps standard CLI behavior and does not auto-switch directories.

## Worktree Structure

With the default configuration (`base_dir: "../worktrees"`):

```
<project-root>/
├── .git/
├── .wtp.yml
└── src/

../worktrees/
├── main/
├── feature/
│   ├── auth/          # jtp add feature/auth
│   └── payment/       # jtp add feature/payment
└── hotfix/
    └── bug-123/       # jtp add hotfix/bug-123
```

Branch names with slashes are preserved as directory structure, automatically
organizing worktrees by type/category.

## Error Handling

jtp provides clear error messages:

```bash
# Branch not found
Error: branch 'nonexistent' not found in local or remote branches

# Multiple remotes have same branch
Error: branch 'feature' exists in multiple remotes: origin, upstream

Solution: Create a local tracking branch for the remote you want without checking it out, then run jtp add again.
  • git branch --track feature origin/feature
  • git branch --track feature upstream/feature
  • jtp add feature

# Worktree already exists
Error: failed to create worktree: exit status 128

# Uncommitted changes
Error: Cannot remove worktree with uncommitted changes. Use --force to override
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md)
for details.

### Development Setup

```bash
# Clone repository
git clone https://github.com/huntertdiamond/jtp.git
cd jtp

# Install dependencies
go mod download

# Run tests
go tool task test

# Build
go tool task build

# Run locally
./jtp --help
```

### Formatting

Run `go tool task fmt` before sending changes. The formatter uses
`golangci-lint fmt` (gofmt + goimports) and automatically derives the
`goimports` `-local` prefix from `go list -m`, so forks and renamed modules
stay grouped correctly. `go tool golangci-lint fmt ./...` still works for
one-off runs, but the task is the authoritative workflow.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Inspired by git-worktree and the need for better multi-branch development
workflows.
