package gitlab

import (
	"context"
	"errors"
	"strconv"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitlabInfo struct {
	Token    string
	GitlabNs string
	BaseURL  string
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

	projList, _, err := git.Projects.ListProjects(&gitlab.ListProjectsOptions{
		Archived: gitlab.Ptr(false),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
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
