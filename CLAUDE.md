# Claude Context for Bore

## Project Overview

Bore is a self-managed SSH tunnel daemon written in Go. It manages SSH tunnels with automatic reconnection, group-based organization, and monitoring capabilities.

## Architecture

### Package Structure

```
cmd/bore/main.go          - Entry point, calls cli.Execute()
internal/
  cli/                    - Cobra CLI commands
  daemon/                 - Background daemon process
  config/                 - YAML config parsing and validation
  tunnel/                 - Tunnel implementations (local/remote)
  ssh/                    - SSH client wrapper and authentication
  reconnect/              - Backoff logic and network monitoring
  ipc/                    - Unix socket client/server communication
  state/                  - Persistent state for restart recovery
```

### Key Design Decisions

1. **Daemon Architecture**: Uses fork pattern with `BORE_DAEMON=1` env var. Parent exits after forking, child runs as daemon.

2. **IPC**: JSON over Unix socket at `~/.bore/bore.sock`. Request/response pattern.

3. **SSH Connection Sharing**: One SSH connection per host, shared by all tunnels to that host.

4. **Network Monitoring**: Uses `netstatus` library on macOS/Windows for native network change notifications. Falls back to DNS polling on Linux.

5. **Tunnel Types**:
   - Local (`-L`): `net.Listen()` locally, `sshClient.Dial()` to remote
   - Remote (`-R`): `sshClient.Listen()` remotely, `net.Dial()` to local

### Important Files

- `internal/daemon/daemon.go` - Main daemon loop, request handling, reconnection logic
- `internal/tunnel/manager.go` - Manages tunnel lifecycle, SSH connections, port conflicts
- `internal/config/config.go` - Config types and YAML parsing
- `internal/reconnect/monitor.go` - Network status monitoring with platform-specific code

## Common Tasks

### Adding a New CLI Command

1. Create file in `internal/cli/` (e.g., `mycommand.go`)
2. Define command with `cobra.Command`
3. Add to root command in `internal/cli/root.go`

### Adding a New IPC Request Type

1. Add constant to `internal/ipc/protocol.go` (e.g., `ReqMyAction = "my_action"`)
2. Add request/response structs if needed
3. Handle in `daemon.HandleRequest()` switch statement
4. Add client method in `internal/ipc/client.go`

### Modifying Config Structure

1. Update types in `internal/config/config.go`
2. Add validation in `internal/config/validate.go`
3. Update tests in `internal/config/config_test.go` and `validate_test.go`

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run specific package tests
go test ./internal/config/...
```

## Building

```bash
# Build binary
go build -o bore ./cmd/bore

# Install to GOPATH/bin
go install ./cmd/bore
```

## Debugging

### Daemon Logs

```bash
# View recent logs
bore logs -n 50

# Follow logs in real-time
bore logs -f

# Direct file access
tail -f ~/.bore/bore.log
```

### Check Daemon Status

```bash
# Via CLI
bore status

# Check PID file
cat ~/.bore/bore.pid

# Check if process is running
ps aux | grep bore
```

### Force Stop Daemon

```bash
# Normal stop
bore stop

# If unresponsive, kill directly
kill $(cat ~/.bore/bore.pid)

# Clean up stale files
rm ~/.bore/bore.pid ~/.bore/bore.sock
```

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/charmbracelet/huh` - Interactive TUI prompts
- `github.com/iamcalledrob/netstatus` - Network status monitoring (macOS/Windows)
- `golang.org/x/crypto/ssh` - SSH connections
- `github.com/kevinburke/ssh_config` - Parse ~/.ssh/config
- `gopkg.in/yaml.v3` - Config parsing

## Code Patterns

### Error Handling

Return errors up the call stack. Log at daemon level. CLI displays to user.

```go
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

### Concurrency

- Tunnels use goroutines for accept loops and connection handling
- Manager uses mutex for tunnel/client maps
- Stats use atomic operations for counters

### Context Usage

Pass context through for cancellation. Daemon creates root context, tunnels receive it.

```go
func (t *LocalTunnel) Start(ctx context.Context) error {
    t.ctx, t.cancel = context.WithCancel(ctx)
    // ...
}
```

## Known Limitations

1. **Host Key Verification**: Currently uses `InsecureIgnoreHostKey()`. Should implement proper verification.

2. **Encrypted Keys**: Keys with passphrases not fully supported (need interactive prompt or agent).

3. **ProxyJump**: Only supports single-hop proxy jump, not chained jumps.

4. **Windows**: Daemon fork pattern uses Unix-specific syscalls. Would need different approach for Windows service.
