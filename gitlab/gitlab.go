package gitlab

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"text/template"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitlabInfo struct {
	Token    string
	GitlabNs string
	BaseURL  string
}
type GitlabVariable struct {
	Key   string
	Value string
}

type GitlabResp struct {
	ProjectName string
	ProjectId   string
}

type GitlabClient struct {
	*gitlab.Client
}

func (g *GitlabInfo) Initgitlab(ctx context.Context) (*GitlabClient, error) {
	if g.Token == "" {
		return nil, errors.New("token cannot be empty")
	}

	baseURL := g.BaseURL
	if baseURL == "" {
		baseURL = "http://127.0.1:8080/api/v4"
	}

	client, err := gitlab.NewClient(g.Token, gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &GitlabClient{
		Client: client,
	}, nil
}

func (g *GitlabInfo) ListProject() ([]*GitlabResp, error) {
	respList := []*GitlabResp{}
	ctx := context.Background()
	git, err := g.Initgitlab(ctx)
	if err != nil {
		return []*GitlabResp{}, err
	}

	projList, _, err := git.Groups.ListGroupProjects(g.GitlabNs, &gitlab.ListGroupProjectsOptions{
		Archived: gitlab.Ptr(false),
	})
	if err != nil {
		return []*GitlabResp{}, err
	}

	for _, repo := range projList {
		resp := &GitlabResp{
			ProjectName: repo.Name,
			ProjectId:   strconv.Itoa(repo.ID),
		}
		respList = append(respList, resp)
	}

	return respList, nil
}

func (g *GitlabInfo) AddGitlabCiFile(gr *GitlabResp, content string) error {
	ctx := context.Background()
	git, err := g.Initgitlab(ctx)
	if err != nil {
		return err
	}
	if exists, _ := g.CheckFileExists(gr, ".gitlab-ci.yaml"); !exists {
		_, _, err = git.RepositoryFiles.CreateFile(gr.ProjectId, ".gitlab-ci.yaml", &gitlab.CreateFileOptions{
			Branch:        gitlab.Ptr("main"),
			CommitMessage: gitlab.Ptr("Add .gitlab-ci.yaml"),
			Content:       gitlab.Ptr(content),
		})
	} else {
		_, _, err = git.RepositoryFiles.UpdateFile(gr.ProjectId, ".gitlab-ci.yaml", &gitlab.UpdateFileOptions{
			Branch:        gitlab.Ptr("main"),
			CommitMessage: gitlab.Ptr("Update .gitlab-ci.yaml"),
			Content:       gitlab.Ptr(content),
		})
	}
	if err != nil {
		return err
	}

	return nil
}

func (g *GitlabInfo) AddGitlabReadmeFile(gr *GitlabResp, content string) error {
	ctx := context.Background()
	git, err := g.Initgitlab(ctx)
	if err != nil {
		return err
	}
	tpl, err := template.New("readme").Parse(content)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = tpl.Execute(&buf, map[string]interface{}{
		"ProjectName": gr.ProjectName,
	})
	if err != nil {
		return err
	}
	if exists, _ := g.CheckFileExists(gr, "README.md"); !exists {
		_, _, err = git.RepositoryFiles.CreateFile(gr.ProjectId, "README.md", &gitlab.CreateFileOptions{
			Branch:        gitlab.Ptr("main"),
			CommitMessage: gitlab.Ptr("Add README.md"),
			Content:       gitlab.Ptr(buf.String()),
		})
	} else {
		_, _, err = git.RepositoryFiles.UpdateFile(gr.ProjectId, "README.md", &gitlab.UpdateFileOptions{
			Branch:        gitlab.Ptr("main"),
			CommitMessage: gitlab.Ptr("Update README.md"),
			Content:       gitlab.Ptr(buf.String()),
		})
	}
	if err != nil {
		return err
	}

	return nil
}
func (g *GitlabInfo) CheckFileExists(gr *GitlabResp, filePath string) (bool, error) {
	ctx := context.Background()
	git, err := g.Initgitlab(ctx)
	if err != nil {
		return false, err
	}
	_, _, err = git.RepositoryFiles.GetFile(gr.ProjectId, filePath, &gitlab.GetFileOptions{
		Ref: gitlab.Ptr("main"),
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (g *GitlabInfo) ListVariables(gr *GitlabResp) ([]*GitlabVariable, error) {
	variables := []*GitlabVariable{}
	ctx := context.Background()
	git, err := g.Initgitlab(ctx)
	if err != nil {
		return nil, err
	}

	vars, _, err := git.ProjectVariables.ListVariables(gr.ProjectId, &gitlab.ListProjectVariablesOptions{})
	if err != nil {
		return nil, err
	}

	for _, v := range vars {
		variables = append(variables, &GitlabVariable{
			Key:   v.Key,
			Value: v.Value,
		})
	}

	return variables, nil
}

func (g *GitlabInfo) CreateVariable(gr *GitlabResp, v *GitlabVariable) error {
	ctx := context.Background()
	git, err := g.Initgitlab(ctx)
	if err != nil {
		return err
	}

	_, _, err = git.ProjectVariables.CreateVariable(gr.ProjectId, &gitlab.CreateProjectVariableOptions{
		Key:       &v.Key,
		Value:     &v.Value,
		Protected: gitlab.Ptr(false),
	})
	if err != nil {
		return err
	}

	return nil
}

func (g *GitlabInfo) UpdateVariable(gr *GitlabResp, key, val string) error {
	ctx := context.Background()
	git, err := g.Initgitlab(ctx)
	if err != nil {
		return err
	}
	// TODO: Check variable type and if file concatenate content
	_, _, err = git.ProjectVariables.UpdateVariable(gr.ProjectId, key, &gitlab.UpdateProjectVariableOptions{
		Value: &val,
	})
	if err != nil {
		return err
	}

	return nil
}
