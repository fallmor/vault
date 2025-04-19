package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupMockGitLabServer() (*httptest.Server, func()) {
	mux := http.NewServeMux()

	// Mock token validation endpoint
	mux.HandleFunc("/api/v4/user", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("PRIVATE-TOKEN")
		if token != "valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Mock projects endpoint
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("PRIVATE-TOKEN")
		if token != "valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		projects := []map[string]interface{}{
			{
				"id":   1,
				"name": "project1",
			},
			{
				"id":   2,
				"name": "project2",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects)
	})

	server := httptest.NewServer(mux)
	return server, server.Close
}

func TestInitgitlab(t *testing.T) {
	server, cleanup := setupMockGitLabServer()
	defer cleanup()

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   "valid-token",
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GitlabInfo{
				Token:   tt.token,
				BaseURL: server.URL + "/api/v4",
			}
			client, err := g.Initgitlab(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if client != nil {
					t.Error("Expected nil client but got one")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if client == nil {
					t.Error("Expected client but got nil")
				}
			}
		})
	}
}

func TestListProject(t *testing.T) {
	server, cleanup := setupMockGitLabServer()
	defer cleanup()

	tests := []struct {
		name      string
		token     string
		namespace string
		wantErr   bool
		wantCount int
	}{
		{
			name:      "valid credentials",
			token:     "valid-token",
			namespace: "test-namespace",
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:      "invalid token",
			token:     "invalid-token",
			namespace: "test-namespace",
			wantErr:   true,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GitlabInfo{
				Token:    tt.token,
				GitlabNs: tt.namespace,
				BaseURL:  server.URL + "/api/v4",
			}
			projects, err := g.ListProject()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if len(projects) != 0 {
					t.Errorf("Expected 0 projects but got %d", len(projects))
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(projects) != tt.wantCount {
					t.Errorf("Expected %d projects but got %d", tt.wantCount, len(projects))
				}
				for _, project := range projects {
					if project.ProjectName == "" {
						t.Error("Project name should not be empty")
					}
					if project.ProjectId == "" {
						t.Error("Project ID should not be empty")
					}
				}
			}
		})
	}
}

func ListVariables(t *testing.T) {

}

func CreateVariable(t *testing.T) {

}
func UpdateVariable(t *testing.T) {

}
