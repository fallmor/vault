package main

import (
	"fmt"
	"gitlab-vault/vault"
	"log"
	"os"
)

func main() {
	vault_addr := os.Getenv("vault_url")
	vault_token := os.Getenv("vault_token")
	role_id := os.Getenv("role_id")
	secret_id := os.Getenv("secret_id")

	reqCreds := vault.NewCreds(vault_addr, "mor/apps", vault_token)
	reqApprole := vault.NewCredsApprole(vault_addr, "mor/apps2", role_id, secret_id)

	Respcreds, err := vault.GetSecret(reqCreds)
	if err != nil {
		log.Fatalf("Could not get the credentials with simple token because %v", err)
	}
	RespApprole, err := vault.GetSecret(reqApprole)
	if err != nil {
		log.Fatalf("Could not get the credentials with approle because %v", err)
	}

	fmt.Println(Respcreds)
	fmt.Println(RespApprole)
}
