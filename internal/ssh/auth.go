package ssh

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// AuthMethods returns SSH authentication methods in priority order:
// 1. SSH Agent
// 2. Key file (if provided)
func AuthMethods(identityFile string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try SSH Agent first
	if agentAuth, err := agentAuthMethod(); err == nil {
		methods = append(methods, agentAuth)
	}

	// Try key file if provided
	if identityFile != "" {
		if keyAuth, err := keyFileAuthMethod(identityFile); err == nil {
			methods = append(methods, keyAuth)
		}
	}

	// Try default key locations
	defaultKeys := []string{
		expandPath("~/.ssh/id_ed25519"),
		expandPath("~/.ssh/id_rsa"),
		expandPath("~/.ssh/id_ecdsa"),
	}
	for _, keyPath := range defaultKeys {
		if keyAuth, err := keyFileAuthMethod(keyPath); err == nil {
			methods = append(methods, keyAuth)
		}
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no authentication methods available")
	}

	return methods, nil
}

// agentAuthMethod returns an AuthMethod that uses the SSH agent
func agentAuthMethod() (ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// keyFileAuthMethod returns an AuthMethod that uses a private key file
func keyFileAuthMethod(path string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		// Key might be encrypted, try with empty passphrase
		// In production, you'd want to prompt for passphrase
		return nil, fmt.Errorf("failed to parse key file: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

// expandPath expands ~ to the home directory
func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}
