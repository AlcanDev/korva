# Installation

> Every supported platform, every supported package manager.

*Last updated: 2026-04-30*

---

## TL;DR

```bash
# macOS / Linux
curl -fsSL https://korva.dev/install | bash

# Homebrew
brew install alcandev/tap/korva

# Windows
iwr -useb https://korva.dev/install.ps1 | iex
```

---

## What gets installed

Three binaries on your PATH:

| Binary | Purpose |
|--------|---------|
| `korva` | CLI |
| `korva-vault` | Local memory + MCP server (HTTP on `localhost:7437`) |
| `korva-sentinel` | Pre-commit architecture validator |

Plus shell completions (bash / zsh / fish) and a config skeleton at `~/.korva/`.

---

## macOS

### Homebrew (recommended)

```bash
brew tap alcandev/tap
brew install korva
```

Updates: `brew upgrade korva`.

### Curl installer

```bash
curl -fsSL https://korva.dev/install | bash
```

Drops binaries into `/usr/local/bin/` (Intel) or `/opt/homebrew/bin/` (Apple Silicon).

### Manual

1. Visit [github.com/AlcanDev/korva/releases/latest](https://github.com/AlcanDev/korva/releases/latest)
2. Download `korva_X.Y.Z_darwin_arm64.tar.gz` (Apple Silicon) or `korva_X.Y.Z_darwin_amd64.tar.gz` (Intel)
3. Extract and move binaries onto your PATH:
   ```bash
   tar -xzf korva_*.tar.gz
   sudo mv korva korva-vault korva-sentinel /usr/local/bin/
   ```

---

## Linux

### Curl installer

```bash
curl -fsSL https://korva.dev/install | bash
```

The installer detects your architecture (amd64 / arm64) and drops binaries into `/usr/local/bin/`. If you don't have sudo, set `INSTALL_DIR`:

```bash
INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://korva.dev/install | bash
```

### Manual

```bash
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
VERSION=$(curl -s https://api.github.com/repos/AlcanDev/korva/releases/latest | grep tag_name | cut -d'"' -f4 | sed 's/^v//')
curl -L "https://github.com/AlcanDev/korva/releases/download/v${VERSION}/korva_${VERSION}_linux_${ARCH}.tar.gz" -o korva.tar.gz
tar -xzf korva.tar.gz
sudo mv korva korva-vault korva-sentinel /usr/local/bin/
```

### Distro packages

Coming soon (.deb, .rpm, AUR). Track [issue #X](https://github.com/AlcanDev/korva/issues).

---

## Windows

### PowerShell installer

```powershell
iwr -useb https://korva.dev/install.ps1 | iex
```

Drops binaries into `%LOCALAPPDATA%\korva\bin\` and adds it to your user PATH.

### Manual

1. Download `korva_X.Y.Z_windows_amd64.zip` from releases
2. Extract to `C:\Program Files\korva\` (or anywhere on your PATH)
3. Open a new terminal — `korva --version` should respond

---

## From source

Requires Go 1.26+.

```bash
git clone https://github.com/AlcanDev/korva.git
cd korva
make all              # sync workspace, build all binaries, run all tests

ls bin/
# korva  korva-vault  korva-sentinel
```

For the embedded Beacon dashboard (Node 22+ required):

```bash
make vault-full       # builds korva-vault with Beacon SPA embedded
```

---

## Verify the install

```bash
korva --version
korva-vault --help | head
korva-sentinel --version
```

If any of these are not found, your PATH is wrong. Add the install location:

```bash
# bash / zsh
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

---

## Shell completions

The release archives include completion scripts:

```bash
# bash
korva completion bash > /usr/local/etc/bash_completion.d/korva

# zsh — make sure your .zshrc has `fpath+=(/usr/local/share/zsh/site-functions)`
korva completion zsh > /usr/local/share/zsh/site-functions/_korva

# fish
korva completion fish > ~/.config/fish/completions/korva.fish
```

---

## Updating

| Method | Command |
|--------|---------|
| Homebrew | `brew upgrade korva` |
| Built-in | `korva update` |
| Manual | re-run the curl/PS1 installer |

`korva update` performs an SHA256-verified download + atomic binary swap. It's the safest path on systems where the binary is owned by your user.

---

## Uninstalling

```bash
brew uninstall korva                       # Homebrew
sudo rm /usr/local/bin/korva*              # Manual
rm -rf ~/.korva                            # Vault data + config
```

`~/.korva/` is yours — back it up if you might come back.

---

## Troubleshooting

### "command not found: korva"
Your PATH doesn't include the install directory. Open a new terminal first; if still missing, see "Verify the install" above.

### "korva-vault: cannot bind 0.0.0.0:7437"
Port 7437 is already in use. Either stop the conflicting process or set `KORVA_VAULT_PORT` to a free port:
```bash
KORVA_VAULT_PORT=8437 korva-vault --mode http
```

### "license: heartbeat failed"
Your machine is offline or `licensing.korva.dev` is unreachable. The license still works offline — heartbeat is a soft check.

### "permission denied" on update
`korva update` needs write access to the directory containing the binary. Either:
- run with sudo: `sudo korva update`, or
- reinstall via Homebrew: `brew upgrade korva`
