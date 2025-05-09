package main

import (
	"context"
	"fmt"
	"gitlab-vault/gitlab"
	"gitlab-vault/vault"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"

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
	AuthType    string
}

type ProfilingInfo struct {
	CpuProfile string
	MemProfile string
}

var k = koanf.New(".")

func main() {

	gi, prof := loadConfig()

	if prof.CpuProfile != "" || prof.MemProfile != "" {
		go Profiling(prof)
	}
	fmt.Printf("Gitops Info: %+v\n", gi)

	if err := validateEnvVars(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	vault_addr := gi.VaultAddr
	role_id := os.Getenv("role_id")
	secret_id := os.Getenv("secret_id")
	token := os.Getenv("vault_token")
	gitlab_url := os.Getenv("gitlab_url")

	var reqApprole vault.GetCreds

	switch gi.AuthType {
	case "approle":
		if gi.ProductLine == "prd" {
			reqApprole = vault.NewCredsApprole(vault_addr, "mor/prod/gitlab", role_id, secret_id)
		} else {
			reqApprole = vault.NewCredsApprole(vault_addr, "mor/stg/gitlab", role_id, secret_id)
		}
	case "token":
		if gi.ProductLine == "prd" {
			reqApprole = vault.NewCreds(vault_addr, "mor/prod/gitlab", token)
		} else {
			reqApprole = vault.NewCreds(vault_addr, "mor/stg/gitlab", token)
		}
	}

	gitlab_info := &gitlab.GitlabInfo{
		BaseURL:  gitlab_url,
		GitlabNs: gi.GitlabNs,
	}

	log.Println("Getting Vault token...")
	// interript signal chan
	mychan := make(chan os.Signal, 1)
	signal.Notify(mychan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := vault.GetSecret(reqApprole, ctx)
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
	projects, err := gitlab_info.ListProject(ctx)
	if err != nil {
		log.Fatalf("Could not list projects: %v", err)
	}
	log.Printf("Found %d projects", len(projects))

	// Create channel for projects and errors
	projectChan := make(chan *gitlab.GitlabResp, len(projects))
	errorChan := make(chan error, len(projects))
	doneChan := make(chan struct{})

	var wg sync.WaitGroup

	for i := range projects {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for project := range projectChan {
				log.Printf("Worker %d processing project: %s", workerID, project.ProjectName)

				log.Printf("Adding Gitlab CI file for project %s", project.ProjectName)
				if err := gitlab_info.AddGitlabCiFile(ctx, project, k.String("gitlab-ci-content")); err != nil {
					errorChan <- fmt.Errorf("could not add Gitlab CI file for project %s: %v", project.ProjectName, err)
					continue
				}

				log.Printf("Adding Gitlab README file for project %s", project.ProjectName)
				if err := gitlab_info.AddGitlabReadmeFile(ctx, project, k.String("gitlab-readme-content")); err != nil {
					errorChan <- fmt.Errorf("could not add Gitlab README file for project %s: %v", project.ProjectName, err)
					continue
				}

				log.Printf("Processing variables for project %s", project.ProjectName)
				vars, err := gitlab_info.ListVariables(ctx, project)
				if err != nil {
					errorChan <- fmt.Errorf("could not list variables for project %s: %v", project.ProjectName, err)
					continue
				}

				for _, v := range vars {
					if err := gitlab_info.UpdateVariable(ctx, project, v); err != nil {
						errorChan <- fmt.Errorf("could not update variable %s for project %s: %v", v.Key, project.ProjectName, err)
					}
				}
			}
		}(i)
	}

	go func() {
		for _, project := range projects {
			projectChan <- project
		}
		close(projectChan)
	}()

	// Attendre que toutes les goroutines se terminent
	go func() {
		wg.Wait()
		close(doneChan)
		close(errorChan)
	}()

	// Gestion des erreurs et arrêt
	var errors []error
	for {
		select {
		case err, ok := <-errorChan:
			if !ok {
				errorChan = nil // Évite de bloquer sur errorChan s'il est fermé
				continue
			}
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
		case sig := <-mychan:
			log.Printf("Received signal: %s. Exiting...", sig)
			cancel()
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

func loadConfig() (*GitopsInfo, *ProfilingInfo) {
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
	// set command line flags
	cmd.String("product_line", "stg", "product line to deploy (prd, stg)")
	cmd.String("cluster_name", "test1", "the cluster name to deploy")
	cmd.String("auth_type", " ", "the authentication type (vault token or approle)")
	cmd.String("cpu_profile", "cpu.pprof", "the cpu profile")
	cmd.String("mem_profile", "mem.pprof", "the memory profile")
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
			AuthType:    k.String("auth_type"),
		}
	case "stg":
		gi = &GitopsInfo{
			ProductLine: k.String("product_line"),
			ClusterName: k.String("cluster_name"),
			GitlabNs:    k.String("zone.development.gitlab_namespace"),
			VaultAddr:   k.String("zone.development.vault_addr"),
			AuthType:    k.String("auth_type"),
		}
	}

	profiling := &ProfilingInfo{
		CpuProfile: k.String("cpu_profile"),
		MemProfile: k.String("mem_profile"),
	}
	return gi, profiling
}

func Profiling(prof *ProfilingInfo) {

	// Check if profiling is enabled for CPU and memory
	if prof.CpuProfile != "" {
		log.Printf("CPU profiling is enabled, writing to %s", prof.CpuProfile)
		f, err := os.Create(prof.CpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	if prof.MemProfile != "" {
		log.Printf("Memory profiling is enabled, writing to %s", prof.MemProfile)
		f, err := os.Create(prof.MemProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
			log.Fatal(err)
		}
	}
}
