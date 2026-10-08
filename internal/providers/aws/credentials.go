package aws

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ID = "aws"

type Credentials struct {
	AccessKey string
	SecretKey string
}

type credentialsFile struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

func credentialsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".wiregam", "creds", ID, "aws.json"), nil
}

// StoreCredentials saves AWS credentials in WireGam's private credentials directory.
func StoreCredentials(credentials Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	credentialsDir := filepath.Dir(path)
	if err := os.MkdirAll(credentialsDir, 0700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}

	data, err := json.MarshalIndent(credentialsFile{
		AccessKey: credentials.AccessKey,
		SecretKey: credentials.SecretKey,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode AWS credentials: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write AWS credentials: %w", err)
	}

	return nil
}

// LoadCredentials reads the static AWS credentials saved by the auth command.
func LoadCredentials() (Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return Credentials{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, fmt.Errorf("AWS credentials not found; run wiregam auth --provider aws")
		}
		return Credentials{}, fmt.Errorf("read AWS credentials: %w", err)
	}

	var stored credentialsFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return Credentials{}, fmt.Errorf("decode AWS credentials: %w", err)
	}
	if stored.AccessKey == "" || stored.SecretKey == "" {
		return Credentials{}, fmt.Errorf("AWS credentials file is missing access_key or secret_key; run wiregam auth --provider aws")
	}

	return Credentials{
		AccessKey: stored.AccessKey,
		SecretKey: stored.SecretKey,
	}, nil
}
