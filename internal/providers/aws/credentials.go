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

// StoreCredentials saves AWS credentials in WireGam's private credentials directory.
func StoreCredentials(credentials Credentials) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	credentialsDir := filepath.Join(homeDir, ".wiregam", "creds", ID)
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

	if err := os.WriteFile(filepath.Join(credentialsDir, "aws.json"), data, 0600); err != nil {
		return fmt.Errorf("write AWS credentials: %w", err)
	}

	return nil
}
