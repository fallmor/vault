package vault

import (
	"context"
	"log"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
)

type Creds struct {
	vault_addr  string
	vault_path  string
	vault_token string
}
type CredsApprole struct {
	vault_addr       string
	vault_path       string
	approle_roleid   string
	approle_secretid string
}
type VaultRespone struct {
	token      map[string]interface{}
	expireTime string
}

func NewCreds(addr, path, token string) *Creds {
	return &Creds{
		vault_addr:  addr,
		vault_path:  path,
		vault_token: token,
	}
}

func NewCredsApprole(addr, path, roleid, secretid string) *CredsApprole {
	return &CredsApprole{
		vault_addr:       addr,
		vault_path:       path,
		approle_roleid:   roleid,
		approle_secretid: secretid,
	}
}

type GetCreds interface {
	RetrieveCreds() (*VaultRespone, error)
}

func (c *Creds) InitVault(ctx context.Context) (vault.Client, error) {
	client, err := vault.New(
		vault.WithAddress(c.vault_addr),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Print("could not initialize vault")
		return vault.Client{}, err
	}
	if err := client.SetToken(c.vault_token); err != nil {
		log.Print("Could not connect to vault")
		return vault.Client{}, err
	}
	return *client.Clone(), nil
}

func (c *CredsApprole) InitVault(ctx context.Context) (vault.Client, error) {
	client, err := vault.New(
		vault.WithAddress(c.vault_addr),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Print("could not initialize vault")
		return vault.Client{}, err
	}

	vaultoken, err := client.Auth.AppRoleLogin(ctx, schema.AppRoleLoginRequest{
		RoleId:   c.approle_roleid,
		SecretId: c.approle_secretid,
	}, vault.WithMountPath("auth/approle/login"))
	if err != nil {
		log.Fatalf("Could not retrieve the token because of the error %v", err)
	}

	if vaultoken == nil || vaultoken.Auth == nil {
		log.Fatal("Login success but no authentication infos received")
	}
	if err := client.SetToken(vaultoken.Auth.ClientToken); err != nil {
		log.Print("Could not connect to vault")
		return vault.Client{}, err
	}
	return *client.Clone(), nil
}

func (c *Creds) RetrieveCreds() (*VaultRespone, error) {
	ctx := context.Background()
	client, err := c.InitVault(ctx)
	if err != nil {
		log.Println("Could not set the vault")
		return nil, err
	}

	resp, err := client.Secrets.KvV2Read(ctx, c.vault_path, vault.WithMountPath("secret"))
	if err != nil {
		log.Printf("Could not retrieve the secret %s", c.vault_path)
		return nil, err
	}
	return &VaultRespone{
		token:      resp.Data.Data,
		expireTime: "",
	}, nil
}

func (c *CredsApprole) RetrieveCreds() (*VaultRespone, error) {
	ctx := context.Background()
	client, err := c.InitVault(ctx)
	if err != nil {
		log.Println("Could not set the vault")
		return nil, err
	}
	resp, err := client.Secrets.KvV2Read(ctx, c.vault_path, vault.WithMountPath("secret"))
	if err != nil {
		log.Printf("Could not retrieve the secret %s", c.vault_path)
		return nil, err
	}
	return &VaultRespone{
		token:      resp.Data.Data,
		expireTime: "",
	}, nil
}

func GetSecret(gt GetCreds) (*VaultRespone, error) {
	resp, err := gt.RetrieveCreds()
	if err != nil {
		log.Println("Could not get the secrets")
		return nil, err
	}
	return resp, nil
}
