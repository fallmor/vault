package main

import (
	"fmt"
	"gitlab-vault/gitlab"
	"gitlab-vault/vault"
	"log"
	"os"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type SecretInfo struct {
	name string
	req  vault.GetCreds
}

type GitopsInfo struct {
	ClusterName  string
	Product_Line string
}

var k = koanf.New(".")

func main() {
	secret_channel := make(chan interface{}, 10)
	gitlab_channel := make(chan *gitlab.GitlabResp, 10)
	done := make(chan struct{})

	gi := loadConfig()
	fmt.Printf("Gitops Info: %+v\n", gi)

	if err := validateEnvVars(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	vault_addr := os.Getenv("vault_url")
	// vault_token := os.Getenv("vault_token")
	role_id := os.Getenv("role_id")
	secret_id := os.Getenv("secret_id")
	gitlab_url := os.Getenv("gitlab_url")
	gitlab_token := os.Getenv("gitlab_token")

	// reqCreds := vault.NewCreds(vault_addr, "mor/apps", vault_token)
	// reqCreds1 := vault.NewCreds(vault_addr, "mor/apps1", vault_token)
	// reqApprole := vault.NewCredsApprole(vault_addr, "mor/apps2", role_id, secret_id)
	reqApprole1 := vault.NewCredsApprole(vault_addr, "mor/prod/gitlab", role_id, secret_id)

	secrets := []SecretInfo{
		{"mor/prod/gitlab", reqApprole1},
	}

	gitlab_info := &gitlab.GitlabInfo{
		Token:    gitlab_token,
		BaseURL:  gitlab_url,
		GitlabNs: "my-group",
	}

	// Start processing secrets
	go func() {
		for _, s := range secrets {
			go func(s SecretInfo) {
				resp, err := vault.GetSecret(s.req)
				if err != nil {
					log.Printf("Could not get credentials for path %s: %v", s.name, err)
					return
				}

				secret_channel <- resp.Token
			}(s)
		}
	}()

	// Start processing GitLab projects
	go func() {
		projects, err := gitlab_info.ListProject()
		if err != nil {
			log.Printf("Could not list projects: %v", err)
			return
		}

		for _, project := range projects {
			gitlab_channel <- project
		}
		close(gitlab_channel) // Close channel when done
	}()

	// Process results
	go func() {
		secretCount := 0
		projectCount := 0
		totalSecrets := len(secrets)

		for {
			select {
			case secret, ok := <-secret_channel:
				if !ok {
					secretCount = totalSecrets
				} else {
					fmt.Printf("Received secret: %s\n", secret)
					secretCount++
				}
			case project, ok := <-gitlab_channel:
				if !ok {
					projectCount = -1
				} else {
					fmt.Printf("Processing project: %s\n", project.ProjectName)
					log.Printf("Adding Gitlab CI file for project %s", project.ProjectName)
					if err := gitlab_info.AddGitlabCiFile(project, k.String("gitlab-ci-content")); err != nil {
						log.Printf("Could not add Gitlab CI file for project %s: %v", project.ProjectName, err)
					}

					vars, err := gitlab_info.ListVariables(project)
					if err != nil {
						log.Printf("Could not list variables for project %s: %v", project.ProjectName, err)
						continue
					}
					for _, v := range vars {
						fmt.Printf("Variable: %s\n", v.Key)
						for _, cluster := range gi {
							log.Printf("Updating variable %s for project %s", v.Key, project.ProjectName)
							if err := gitlab_info.UpdateVariable(project, v.Key, cluster.ClusterName); err != nil {
								log.Printf("Could not update variable for project %s: %v", project.ProjectName, err)
							}
						}
					}
					projectCount++
				}
			case <-time.After(30 * time.Second):
				log.Println("Timeout reached")
				close(done)
				return
			}

			// Check if we've processed all items
			if secretCount == totalSecrets && projectCount == -1 {
				close(done)
				return
			}
		}
	}()

	// Wait for all goroutines to finish
	<-done
}

func validateEnvVars() error {
	required := []string{"vault_url", "gitlab_url", "gitlab_token"}
	required_only_one := []string{"vault_token", "role_id", "secret_id"}
	for _, env := range required {
		if os.Getenv(env) == "" {
			return fmt.Errorf("required environment variable %s is not set", env)
		}
	}
	for _, env := range required_only_one {
		if os.Getenv(required_only_one[0]) == "" && os.Getenv(required_only_one[1]) == "" || os.Getenv(required_only_one[2]) == "" {
			return fmt.Errorf("required environment variable %s or approle are not set", env)
		}
	}
	return nil
}
func loadConfig() []*GitopsInfo {
	f := file.Provider("conf/config.yaml")
	log.Printf("Loading config from %v", f)
	if err := k.Load(f, yaml.Parser()); err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	zone_prod := k.Strings("zone.production")
	zone_dev := k.Strings("zone.development")

	gi := []*GitopsInfo{}
	for _, v := range zone_prod {
		gi = append(gi, &GitopsInfo{
			ClusterName:  v,
			Product_Line: "production",
		})
	}
	for _, v := range zone_dev {
		gi = append(gi, &GitopsInfo{
			ClusterName:  v,
			Product_Line: "development",
		})
	}
	return gi
}
