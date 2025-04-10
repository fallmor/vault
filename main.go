package main

import (
	"fmt"
	"gitlab-vault/gitlab"
	"gitlab-vault/vault"
	"log"
	"os"
	"time"
)

type Secret struct {
	User        string
	Pass        string
	UnknownData string
}
type SecretInfo struct {
	name string
	req  vault.GetCreds
}

func main() {
	secret_channel := make(chan *Secret)
	gitlab_channel := make(chan *gitlab.GitlabResp)
	done := make(chan bool)

	if err := validateEnvVars(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	vault_addr := os.Getenv("vault_url")
	vault_token := os.Getenv("vault_token")
	role_id := os.Getenv("role_id")
	secret_id := os.Getenv("secret_id")
	gitlab_url := os.Getenv("gitlab_url")
	gitlab_token := os.Getenv("gitlab_token")

	reqCreds := vault.NewCreds(vault_addr, "mor/apps", vault_token)
	reqCreds1 := vault.NewCreds(vault_addr, "mor/apps1", vault_token)
	reqApprole := vault.NewCredsApprole(vault_addr, "mor/apps2", role_id, secret_id)

	secrets := []SecretInfo{
		{"mor/apps", reqCreds},
		{"mor/apps1", reqCreds1},
		{"mor/apps2", reqApprole},
	}

	gitlab_info := &gitlab.GitlabInfo{
		Token:    gitlab_token,
		BaseURL:  gitlab_url,
		GitlabNs: "my-group",
	}

	go func() {
		for _, s := range secrets {
			go func(s SecretInfo) {
				resp, err := vault.GetSecret(s.req)
				if err != nil {
					log.Printf("Could not get credentials for path %s: %v", s.name, err)
					return
				}

				secret, err := ParseSecret(resp)
				if err != nil {
					log.Printf("Could not parse secret for path %s: %v", s.name, err)
					return
				}

				secret_channel <- secret
			}(s)
		}
	}()

	go func() {

		projects, err := gitlab_info.ListProject()
		if err != nil {
			log.Printf("Could not list projects: %v", err)
			return
		}

		for _, project := range projects {
			gitlab_channel <- project
		}
	}()

	go func() {
		for {
			select {
			case secret := <-secret_channel:
				fmt.Printf("Received secret: %s\n", secret.String())
			case project := <-gitlab_channel:
				fmt.Printf("Processing project: %s\n", project.ProjectName)
				vars, err := gitlab_info.ListVariables(project)
				if err != nil {
					log.Printf("Could not list variables for project %s: %v", project.ProjectName, err)
					continue
				}
				for _, v := range vars {
					fmt.Printf("Variable: %s\n", v.Key)
				}
			case <-time.After(10 * time.Second):
				done <- true
				return
			}
		}
	}()

	<-done
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
	required := []string{"vault_url", "vault_token", "gitlab_url", "gitlab_token"}
	for _, env := range required {
		if os.Getenv(env) == "" {
			return fmt.Errorf("required environment variable %s is not set", env)
		}
	}
	return nil
}
