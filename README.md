# Bore

A self-managed SSH tunnel daemon in Go with automatic reconnection, group-based tunnel management, and full monitoring.

## Features

- **Local and Remote Forwarding**: Support for both `-L` (local) and `-R` (remote) SSH port forwarding
- **Automatic Reconnection**: Exponential backoff with network status awareness for fast recovery
- **Group Management**: Organize tunnels into groups and enable/disable them together
- **Port Conflict Detection**: Prevents starting tunnels with conflicting local ports
- **SSH Config Integration**: Reads host settings from `~/.ssh/config`
- **Traffic Statistics**: Track bytes sent/received, connection counts, and uptime
- **Interactive Mode**: TUI-based tunnel and group selection using [huh](https://github.com/charmbracelet/huh)
- **State Persistence**: Automatically restores tunnels after daemon restart

## Installation

```bash
go install github.com/pjtatlow/bore@latest
```

Or build from source:

```bash
git clone https://github.com/pjtatlow/bore.git
cd bore
go build -o bore .
```

## Quick Start

1. Create a configuration file at `~/.bore/config.yaml`:

```yaml
hosts:
  bastion:
    hostname: bastion.example.com
    user: admin

tunnels:
  web-app:
    type: local
    local_port: 8080
    remote_host: localhost
    remote_port: 80

  dev-server:
    type: remote
    local_port: 3000
    remote_port: 9000

groups:
  development:
    description: "Dev environment tunnels"
    tunnels: [web-app, dev-server]
```

2. Start the daemon:

```bash
bore start
```

3. Enable a tunnel group (specify which host to connect through):

```bash
bore group enable development --host bastion
```

4. Check status:

```bash
bore status
```

## Commands

| Command | Description |
|---------|-------------|
| `bore start` | Start the daemon in the background |
| `bore stop` | Stop the daemon and all tunnels |
| `bore status` | Show daemon and tunnel status with statistics |
| `bore group enable <name> --host <host>` | Start all tunnels in a group via host |
| `bore group disable <name>` | Stop all tunnels in a group |
| `bore tunnel up <name> --host <host>` | Start an individual tunnel via host |
| `bore tunnel down <name>` | Stop an individual tunnel |
| `bore config validate` | Validate configuration syntax |
| `bore config edit` | Open config in $EDITOR |
| `bore config path` | Show configuration file path |
| `bore logs [-f] [-n N]` | View daemon logs (-f to follow) |
| `bore` | Interactive tunnel/group selector |
| `bore completion <shell>` | Generate shell completions |

## Configuration

Configuration is stored at `~/.bore/config.yaml`.

### Full Example

```yaml
defaults:
  reconnect:
    enabled: true
    max_backoff: 30s
    initial_backoff: 1s
    multiplier: 2.0
  keep_alive:
    interval: 30s

hosts:
  bastion:
    hostname: bastion.example.com
    user: admin
    port: 22
    identity_file: ~/.ssh/id_ed25519
    proxy_jump: jump-host

  production:
    hostname: prod.example.com
    user: deploy

tunnels:
  # Local forwarding: listen locally, forward to remote
  web-app:
    type: local
    local_port: 8080
    remote_host: internal-web.local
    remote_port: 80

  database:
    type: local
    local_port: 5432
    remote_host: db.internal
    remote_port: 5432

  # Remote forwarding: listen on remote, forward to local
  dev-server:
    type: remote
    local_port: 3000
    remote_port: 9000

groups:
  development:
    description: "Dev environment"
    tunnels: [web-app, database]

  expose-local:
    description: "Expose local services"
    tunnels: [dev-server]
```

### Host Configuration

Hosts can be configured in bore's config or inherited from `~/.ssh/config`. Bore checks both, with bore's config taking precedence.

| Field | Description |
|-------|-------------|
| `hostname` | Server hostname or IP |
| `user` | SSH username |
| `port` | SSH port (default: 22) |
| `identity_file` | Path to private key |
| `proxy_jump` | Jump host for ProxyJump |

### Tunnel Configuration

Tunnels are host-agnostic — they define the port forwarding but not which SSH host to connect through. The host is specified at runtime with the `--host` flag when starting tunnels or enabling groups. This lets you reuse the same tunnel definitions with different hosts.

### Tunnel Types

**Local Forwarding** (`type: local`):
- Listens on `local_port` on your machine
- Forwards connections through SSH to `remote_host:remote_port`
- Equivalent to `ssh -L local_port:remote_host:remote_port`

**Remote Forwarding** (`type: remote`):
- Listens on `remote_port` on the SSH server
- Forwards connections back to `local_port` on your machine
- Equivalent to `ssh -R remote_port:localhost:local_port`

## Authentication

Bore supports authentication via:

1. **SSH Agent** (recommended): If `SSH_AUTH_SOCK` is set, bore will use your SSH agent
2. **Key Files**: Specify `identity_file` in host config, or bore will try default locations:
   - `~/.ssh/id_ed25519`
   - `~/.ssh/id_rsa`
   - `~/.ssh/id_ecdsa`

## Reconnection

When a connection is lost, bore will:

1. Check network availability (uses OS-native APIs on macOS/Windows, DNS polling on Linux)
2. If network is unavailable, wait for it to come back
3. Attempt reconnection with exponential backoff:
   - Start at 1 second
   - Double each attempt (with 0-25% jitter)
   - Cap at 30 seconds
4. On success, reset backoff timer

When network is restored, bore immediately attempts to reconnect all failed tunnels.

## Files

| Path | Description |
|------|-------------|
| `~/.bore/config.yaml` | Configuration file |
| `~/.bore/bore.pid` | Daemon PID file |
| `~/.bore/bore.sock` | Unix socket for IPC |
| `~/.bore/bore.log` | Daemon log file |
| `~/.bore/state.json` | Persisted state for restart recovery |

## Shell Completions

Generate completions for your shell:

```bash
# Bash
bore completion bash > /etc/bash_completion.d/bore

# Zsh
bore completion zsh > "${fpath[1]}/_bore"

# Fish
bore completion fish > ~/.config/fish/completions/bore.fish

# PowerShell
bore completion powershell > bore.ps1
```

## Development

```bash
# Run tests
go test ./...

# Build
go build -o bore .

# Run directly
go run . start
```

## License

MIT
