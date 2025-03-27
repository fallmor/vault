package main

import (
	"fmt"
	"gitlab-vault/vault"
	"os"
)

func main() {
	vault_addr := os.Getenv("vault_url")
	// vault_token := os.Getenv("vault_token")
	role_id := os.Getenv("role_id")
	secret_id := os.Getenv("secret_id")

	// req := vault.NewCreds(vault_addr, "mor/apps", vault_token)
	req := vault.NewCredsApprole(vault_addr, "mor/apps", role_id, secret_id)

	resp := req.RetrieveCreds()

	fmt.Println(resp)
}
