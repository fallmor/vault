package main

import (
	"fmt"
	"gitlab-vault/gitlab"
	"gitlab-vault/vault"
	"log"
	"os"
	"sync"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

type GitopsInfo struct {
	ClusterName string
	ProductLine string
	GitlabNs    string
	VaultAddr   string
}

var k = koanf.New(".")

func main() {
	gi := loadConfig()
	fmt.Printf("Gitops Info: %+v\n", gi)

	if err := validateEnvVars(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	vault_addr := gi.VaultAddr
	role_id := os.Getenv("role_id")
	secret_id := os.Getenv("secret_id")
	gitlab_url := os.Getenv("gitlab_url")

	var reqApprole vault.GetCreds
	if gi.ProductLine == "prd" {
		reqApprole = vault.NewCredsApprole(vault_addr, "mor/prod/gitlab", role_id, secret_id)
	} else {
		reqApprole = vault.NewCredsApprole(vault_addr, "mor/stg/gitlab", role_id, secret_id)
	}

	gitlab_info := &gitlab.GitlabInfo{
		BaseURL:  gitlab_url,
		GitlabNs: gi.GitlabNs,
	}

	log.Println("Getting Vault token...")
	resp, err := vault.GetSecret(reqApprole)
	if err != nil {
		log.Fatalf("Could not get credentials: %v", err)
	}

	log.Printf("Vault response: %+v", resp)
	if resp.Token == nil {
		log.Fatal("No token received from Vault")
	}

	token, ok := resp.Token["token"].(string)
	if !ok || token == "" {
		log.Fatal("Invalid or empty token received from Vault")
	}
	gitlab_info.Token = token
	log.Println("Successfully got Vault token")

	// List GitLab projects
	log.Println("Listing GitLab projects...")
	projects, err := gitlab_info.ListProject()
	if err != nil {
		log.Fatalf("Could not list projects: %v", err)
	}
	log.Printf("Found %d projects", len(projects))

	// Create channel for projects and errors
	projectChan := make(chan *gitlab.GitlabResp, len(projects))
	errorChan := make(chan error, len(projects))
	doneChan := make(chan struct{})

	var wg sync.WaitGroup

	// Start worker goroutines
	for i := range 5 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for project := range projectChan {
				log.Printf("Worker %d processing project: %s", workerID, project.ProjectName)

				log.Printf("Adding Gitlab CI file for project %s", project.ProjectName)
				if err := gitlab_info.AddGitlabCiFile(project, k.String("gitlab-ci-content")); err != nil {
					errorChan <- fmt.Errorf("could not add Gitlab CI file for project %s: %v", project.ProjectName, err)
					continue
				}

				log.Printf("Adding Gitlab README file for project %s", project.ProjectName)
				if err := gitlab_info.AddGitlabReadmeFile(project, k.String("gitlab-readme-content")); err != nil {
					errorChan <- fmt.Errorf("could not add Gitlab README file for project %s: %v", project.ProjectName, err)
					continue
				}

				log.Printf("Processing variables for project %s", project.ProjectName)
				vars, err := gitlab_info.ListVariables(project)
				if err != nil {
					errorChan <- fmt.Errorf("could not list variables for project %s: %v", project.ProjectName, err)
					continue
				}

				for _, v := range vars {
					if err := gitlab_info.UpdateVariable(project, v.Key, gi.ClusterName); err != nil {
						errorChan <- fmt.Errorf("could not update variable %s for project %s: %v", v.Key, project.ProjectName, err)
					}
				}
			}
		}(i)
	}

	// Send projects to workers
	go func() {
		for _, project := range projects {
			projectChan <- project
		}
		close(projectChan)
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(doneChan)

	// Process errors
	var errors []error
	for {
		select {
		case err := <-errorChan:
			errors = append(errors, err)
		case <-doneChan:
			if len(errors) > 0 {
				log.Printf("Completed with %d errors:", len(errors))
				for _, err := range errors {
					log.Println(err)
				}
			} else {
				log.Println("Successfully processed all projects")
			}
			return
		}
	}
}

func validateEnvVars() error {
	required_only_one := []string{"vault_token", "role_id", "secret_id"}

	if os.Getenv("gitlab_url") == "" {
		return fmt.Errorf("required environment variable gitlab_url is not set")
	}

	for _, env := range required_only_one {
		if os.Getenv(required_only_one[0]) == "" && os.Getenv(required_only_one[1]) == "" || os.Getenv(required_only_one[2]) == "" {
			return fmt.Errorf("required environment variable %s or approle are not set", env)
		}
	}
	return nil
}

func loadConfig() *GitopsInfo {
	f := file.Provider("conf/config.yaml")
	log.Printf("Loading config from %v", f)
	if err := k.Load(f, yaml.Parser()); err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	cmd := pflag.NewFlagSet("config", pflag.ExitOnError)
	cmd.Usage = func() {
		fmt.Println(cmd.FlagUsages())
		os.Exit(0)
	}
	cmd.String("product_line", "stg", "product line to deploy (prd, stg)")
	cmd.String("cluster_name", "test1", "the cluster name to deploy")
	cmd.Parse(os.Args[1:])

	cFiles, _ := cmd.GetStringSlice("conf")
	for _, c := range cFiles {
		if err := k.Load(file.Provider(c), toml.Parser()); err != nil {
			log.Fatalf("error loading file: %v", err)
		}
	}

	if err := k.Load(posflag.Provider(cmd, ".", k), nil); err != nil {
		log.Fatalf("error loading config: %v", err)
	}
	gi := &GitopsInfo{}
	switch k.String("product_line") {
	case "prd":
		gi = &GitopsInfo{
			ProductLine: k.String("product_line"),
			ClusterName: k.String("cluster_name"),
			GitlabNs:    k.String("zone.production.gitlab_namespace"),
			VaultAddr:   k.String("zone.production.vault_addr"),
		}
	case "stg":
		gi = &GitopsInfo{
			ProductLine: k.String("product_line"),
			ClusterName: k.String("cluster_name"),
			GitlabNs:    k.String("zone.development.gitlab_namespace"),
			VaultAddr:   k.String("zone.development.vault_addr"),
		}
	}
	fmt.Printf("Gitops Info: %+v\n", gi)
	return gi
}
