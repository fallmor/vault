package main

import (
	"fmt"
	"gitlab-vault/vault"
	"log"
	"os"
)

type Secret struct {
	User        string
	Pass        string
	UnknownData string
}

func ParseSecret(resp *vault.VaultRespone) (*Secret, error) {
	if resp == nil || resp.Token == nil {
		return nil, fmt.Errorf("invalid response: response or token is nil")
	}

	ss := &Secret{}

	if username, ok := resp.Token["username"].(string); ok {
		ss.User = username
	}
	if password, ok := resp.Token["password"].(string); ok {
		ss.Pass = password
	}

	for k, v := range resp.Token {
		if k != "username" && k != "password" {
			if strVal, ok := v.(string); ok {
				ss.UnknownData = strVal
				break
			}
		}
	}

	return ss, nil
}

func (s Secret) String() string {
	return fmt.Sprintf("User: %s, Pass: %s, UnknownData: %s", s.User, s.Pass, s.UnknownData)
}

func validateEnvVars() error {
	required := []string{"vault_url", "vault_token"}
	for _, env := range required {
		if os.Getenv(env) == "" {
			return fmt.Errorf("required environment variable %s is not set", env)
		}
	}
	return nil
}

func main() {
	if err := validateEnvVars(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	vault_addr := os.Getenv("vault_url")
	vault_token := os.Getenv("vault_token")
	role_id := os.Getenv("role_id")
	secret_id := os.Getenv("secret_id")

	// Initialize credential requests
	reqCreds := vault.NewCreds(vault_addr, "mor/apps", vault_token)
	reqCreds1 := vault.NewCreds(vault_addr, "mor/apps1", vault_token)
	reqApprole := vault.NewCredsApprole(vault_addr, "mor/apps2", role_id, secret_id)

	// Retrieve and parse secrets
	secrets := []struct {
		name string
		req  vault.GetCreds
	}{
		{"mor/apps", reqCreds},
		{"mor/apps1", reqCreds1},
		{"mor/apps2", reqApprole},
	}

	for _, s := range secrets {
		resp, err := vault.GetSecret(s.req)
		if err != nil {
			log.Printf("Could not get credentials for path %s: %v", s.name, err)
			continue
		}

		secret, err := ParseSecret(resp)
		if err != nil {
			log.Printf("Could not parse secret for path %s: %v", s.name, err)
			continue
		}

		fmt.Printf("Secret for %s: %s\n", s.name, secret.String())
	}
}
