package localvpn

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	clientAddress = "10.8.0.2/24"
	serverAddress = "10.8.0.1"
	listenPort    = 51820
)

// Keys contains the local WireGuard identity. PrivateKey must never be sent to
// the cloud provider; PublicKey is safe to add to the server configuration.
type Keys struct {
	PrivateKey string
	PublicKey  string
}

// Prepare installs WireGuard when needed and creates or reuses a local key pair.
func Prepare(ctx context.Context) (Keys, error) {
	if runtime.GOOS != "linux" {
		return Keys{}, fmt.Errorf("automatic laptop setup currently supports Linux only")
	}

	if err := ensureWireGuardInstalled(ctx); err != nil {
		return Keys{}, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Keys{}, fmt.Errorf("get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".wiregam", "wireguard")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return Keys{}, fmt.Errorf("create local WireGuard directory: %w", err)
	}
	if err := os.Chmod(configDir, 0700); err != nil {
		return Keys{}, fmt.Errorf("secure local WireGuard directory: %w", err)
	}

	privateKeyPath := filepath.Join(configDir, "client-private.key")
	publicKeyPath := filepath.Join(configDir, "client-public.key")

	privateKey, err := readOrGeneratePrivateKey(ctx, privateKeyPath)
	if err != nil {
		return Keys{}, err
	}

	publicKey, err := derivePublicKey(ctx, privateKey)
	if err != nil {
		return Keys{}, err
	}
	if err := writeAtomic(publicKeyPath, []byte(publicKey+"\n"), 0600); err != nil {
		return Keys{}, fmt.Errorf("write client public key: %w", err)
	}

	return Keys{PrivateKey: privateKey, PublicKey: publicKey}, nil
}

// ConfigureAndStart writes the completed local configuration to
// /etc/wireguard/wg0.conf, starts the tunnel, and verifies a handshake.
func ConfigureAndStart(ctx context.Context, keys Keys, serverPublicKey, serverPublicIP string) error {
	if err := validateKey(keys.PrivateKey); err != nil {
		return fmt.Errorf("client private key: %w", err)
	}
	if err := validateKey(serverPublicKey); err != nil {
		return fmt.Errorf("server public key: %w", err)
	}

	endpointIP := net.ParseIP(serverPublicIP)
	if endpointIP == nil || endpointIP.To4() == nil {
		return fmt.Errorf("invalid EC2 public IPv4 address %q", serverPublicIP)
	}

	config := renderConfig(keys.PrivateKey, serverPublicKey, endpointIP.String())
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	localConfigPath := filepath.Join(homeDir, ".wiregam", "wireguard", "wg0.conf")
	if err := writeAtomic(localConfigPath, []byte(config), 0600); err != nil {
		return fmt.Errorf("write local WireGuard configuration: %w", err)
	}

	if err := runInteractive(ctx, "sudo", "install", "-d", "-m", "700", "/etc/wireguard"); err != nil {
		return fmt.Errorf("create system WireGuard directory: %w", err)
	}

	systemConfigPath := "/etc/wireguard/wg0.conf"
	backupPath := ""
	exists, err := privilegedPathExists(ctx, systemConfigPath)
	if err != nil {
		return fmt.Errorf("check existing WireGuard configuration: %w", err)
	}
	if exists {
		backupPath = systemConfigPath + ".wiregam-backup-" + time.Now().Format("20060102T150405")
		if err := runInteractive(ctx, "sudo", "cp", "-a", "--", systemConfigPath, backupPath); err != nil {
			return fmt.Errorf("back up existing WireGuard configuration: %w", err)
		}
		fmt.Println("Backed up the previous WireGuard configuration to", backupPath)
	}

	wasActive := interfaceExists(ctx, "wg0")
	if wasActive {
		if err := runInteractive(ctx, "sudo", "wg-quick", "down", "wg0"); err != nil {
			return fmt.Errorf("stop existing WireGuard tunnel: %w", err)
		}
	}

	if err := runInteractive(ctx, "sudo", "install", "-m", "600", localConfigPath, systemConfigPath); err != nil {
		restorePreviousConfig(ctx, systemConfigPath, backupPath, wasActive)
		return fmt.Errorf("install local WireGuard configuration: %w", err)
	}
	if err := runInteractive(ctx, "sudo", "wg-quick", "up", "wg0"); err != nil {
		_ = runInteractive(ctx, "sudo", "wg-quick", "down", "wg0")
		restorePreviousConfig(ctx, systemConfigPath, backupPath, wasActive)
		return fmt.Errorf("start WireGuard tunnel: %w", err)
	}

	if err := waitForHandshake(ctx, 30*time.Second); err != nil {
		_ = runInteractive(ctx, "sudo", "wg-quick", "down", "wg0")
		restorePreviousConfig(ctx, systemConfigPath, backupPath, wasActive)
		return err
	}
	if err := runInteractive(ctx, "sudo", "systemctl", "enable", "wg-quick@wg0"); err != nil {
		return fmt.Errorf("enable WireGuard at boot: %w", err)
	}

	fmt.Println("WireGuard tunnel is connected.")
	return nil
}

func ensureWireGuardInstalled(ctx context.Context) error {
	if _, err := exec.LookPath("wg"); err == nil {
		if _, err := exec.LookPath("wg-quick"); err == nil {
			if _, err := exec.LookPath("resolvconf"); err == nil {
				return nil
			}
		}
	}

	if _, err := exec.LookPath("dnf"); err != nil {
		return fmt.Errorf("required WireGuard or DNS tools are missing and dnf is unavailable; install wireguard-tools and openresolv, then retry")
	}

	fmt.Println("Installing WireGuard tools on this laptop...")
	if err := runInteractive(ctx, "sudo", "dnf", "install", "-y", "wireguard-tools", "openresolv"); err != nil {
		return fmt.Errorf("install local WireGuard tools: %w", err)
	}
	for _, required := range []string{"wg", "wg-quick", "resolvconf"} {
		if _, err := exec.LookPath(required); err != nil {
			return fmt.Errorf("required command %q is unavailable after package installation", required)
		}
	}
	return nil
}

func readOrGeneratePrivateKey(ctx context.Context, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if err := os.Chmod(path, 0600); err != nil {
			return "", fmt.Errorf("secure client private key: %w", err)
		}
		key := strings.TrimSpace(string(data))
		if err := validateKey(key); err != nil {
			return "", fmt.Errorf("existing client private key is invalid: %w", err)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read client private key: %w", err)
	}

	output, err := exec.CommandContext(ctx, "wg", "genkey").Output()
	if err != nil {
		return "", fmt.Errorf("generate client private key: %w", err)
	}
	key := strings.TrimSpace(string(output))
	if err := validateKey(key); err != nil {
		return "", fmt.Errorf("generated client private key is invalid: %w", err)
	}
	if err := writeAtomic(path, []byte(key+"\n"), 0600); err != nil {
		return "", fmt.Errorf("write client private key: %w", err)
	}

	return key, nil
}

func derivePublicKey(ctx context.Context, privateKey string) (string, error) {
	command := exec.CommandContext(ctx, "wg", "pubkey")
	command.Stdin = strings.NewReader(privateKey + "\n")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("derive client public key: %w", err)
	}
	key := strings.TrimSpace(string(output))
	if err := validateKey(key); err != nil {
		return "", fmt.Errorf("derived client public key is invalid: %w", err)
	}
	return key, nil
}

func renderConfig(privateKey, serverPublicKey, serverPublicIP string) string {
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = 1.1.1.1

[Peer]
PublicKey = %s
Endpoint = %s:%d
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`, privateKey, clientAddress, serverPublicKey, serverPublicIP, listenPort)
}

func validateKey(key string) error {
	if strings.TrimSpace(key) != key {
		return fmt.Errorf("contains surrounding whitespace")
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("must be a base64-encoded 32-byte WireGuard key")
	}
	return nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".wiregam-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func runInteractive(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func interfaceExists(ctx context.Context, name string) bool {
	return exec.CommandContext(ctx, "ip", "link", "show", name).Run() == nil
}

func privilegedPathExists(ctx context.Context, path string) (bool, error) {
	command := exec.CommandContext(ctx, "sudo", "test", "-e", path)
	command.Stdin = os.Stdin
	command.Stderr = os.Stderr
	err := command.Run()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func restorePreviousConfig(ctx context.Context, systemConfigPath, backupPath string, wasActive bool) {
	if backupPath == "" {
		return
	}
	if err := runInteractive(ctx, "sudo", "install", "-m", "600", backupPath, systemConfigPath); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not restore the previous WireGuard configuration:", err)
		return
	}
	fmt.Fprintln(os.Stderr, "Restored the previous WireGuard configuration after setup failed.")
	if wasActive {
		if err := runInteractive(ctx, "sudo", "wg-quick", "up", "wg0"); err != nil {
			fmt.Fprintln(os.Stderr, "Warning: could not restart the previous WireGuard tunnel:", err)
		}
	}
}

func waitForHandshake(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if pingPath, err := exec.LookPath("ping"); err == nil {
			_ = exec.CommandContext(ctx, pingPath, "-c", "1", "-W", "1", serverAddress).Run()
		}

		command := exec.CommandContext(ctx, "sudo", "wg", "show", "wg0", "latest-handshakes")
		var stderr bytes.Buffer
		command.Stderr = &stderr
		output, err := command.Output()
		if err != nil {
			return fmt.Errorf("check WireGuard handshake: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			latest, parseErr := strconv.ParseInt(fields[1], 10, 64)
			if parseErr == nil && latest > 0 {
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("WireGuard started but no server handshake was received within %s", timeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func Disconnect(ctx context.Context) error {
	if err := runInteractive(ctx, "sudo", "wg-quick", "down", "wg0"); err != nil {
		return fmt.Errorf("stop WireGuard tunnel: %w", err)
	}
	if err := runInteractive(ctx, "sudo", "systemctl", "disable", "wg-quick@wg0"); err != nil {
		return fmt.Errorf("disable WireGuard at boot: %w", err)
	}
	fmt.Println("WireGuard tunnel is disconnected.")
	return nil
}

func Connect(ctx context.Context) error {
	if err := runInteractive(ctx, "sudo", "wg-quick", "up", "wg0"); err != nil {
		return fmt.Errorf("start WireGuard tunnel: %w", err)
	}
	if err := runInteractive(ctx, "sudo", "systemctl", "enable", "wg-quick@wg0"); err != nil {
		return fmt.Errorf("enable WireGuard at boot: %w", err)
	}
	fmt.Println("WireGuard tunnel is connected.")
	return nil
}
