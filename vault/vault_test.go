package vault

import (
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/vault"
)

const (
	testToken = "mor"
)

func TestRetrieveCreds(t *testing.T) {
	cluster := vault.NewTestCluster(t, &vault.CoreConfig{
		DevToken: testToken,
	}, &vault.TestClusterOptions{})
	cluster.Start()
	defer cluster.Cleanup()

	core := cluster.Cores[0].Core
	vault.TestWaitActive(t, core)
	client := cluster.Cores[0].Client
	client.SetToken(testToken)

	// Write a test secret to the vault
	_, err := client.Logical().Write("secret/data/test", map[string]interface{}{
		"data": map[string]interface{}{
			"key": "value",
		},
	})
	if err != nil {
		t.Fatalf("failed to write secret: %v", err)
	}

	// Initialize Creds and test RetrieveCreds
	creds := NewCreds(client.Address(), "test", testToken)
	resp, err := creds.RetrieveCreds()
	if err != nil {
		t.Fatalf("failed to retrieve creds: %v", err)
	}
	if resp == nil || resp.Token["key"] != "value" {
		t.Fatalf("expected key to be 'value', got: %v", resp.Token["key"])
	}
}

func TestRetrieveCredsApprole(t *testing.T) {
	cluster := vault.NewTestCluster(t, &vault.CoreConfig{
		DevToken: testToken,
		EnableUI: false,
	}, &vault.TestClusterOptions{
		NumCores: 1,
	})
	cluster.Start()
	defer cluster.Cleanup()

	core := cluster.Cores[0].Core
	vault.TestWaitActive(t, core)
	client := cluster.Cores[0].Client
	client.SetToken(testToken)

	// Enable AppRole auth method
	err := client.Sys().EnableAuthWithOptions("approle", &api.EnableAuthOptions{
		Type:        "approle",
		Description: "AppRole auth method for testing",
		Config: api.AuthConfigInput{
			DefaultLeaseTTL: "1h",
			MaxLeaseTTL:     "2h",
		},
	})
	if err != nil {
		t.Fatalf("failed to enable approle auth: %v", err)
	}

	// Create an AppRole role
	_, err = client.Logical().Write("auth/approle/role/test-role", map[string]interface{}{
		"token_policies": []string{"default"},
		"token_ttl":      "1h",
		"token_max_ttl":  "2h",
	})
	if err != nil {
		t.Fatalf("failed to create approle role: %v", err)
	}

	roleIDResp, err := client.Logical().Read("auth/approle/role/test-role/role-id")
	if err != nil {
		t.Fatalf("failed to get role ID: %v", err)
	}
	if roleIDResp == nil || roleIDResp.Data == nil {
		t.Fatal("role ID response is nil")
	}
	roleID := roleIDResp.Data["role_id"].(string)

	secretIDResp, err := client.Logical().Write("auth/approle/role/test-role/secret-id", nil)
	if err != nil {
		t.Fatalf("failed to generate secret ID: %v", err)
	}
	if secretIDResp == nil || secretIDResp.Data == nil {
		t.Fatal("secret ID response is nil")
	}
	secretID := secretIDResp.Data["secret_id"].(string)

	_, err = client.Logical().Write("secret/data/test", map[string]interface{}{
		"data": map[string]interface{}{
			"key": "value",
		},
	})
	if err != nil {
		t.Fatalf("failed to write secret: %v", err)
	}

	credsApprole := NewCredsApprole(client.Address(), "test", roleID, secretID)
	resp, err := credsApprole.RetrieveCreds()
	if err != nil {
		t.Fatalf("failed to retrieve creds with approle: %v", err)
	}
	if resp == nil || resp.Token["key"] != "value" {
		t.Fatalf("expected key to be 'value', got: %v", resp.Token["key"])
	}
}
