package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type TaskclusterClient struct {
	RootURL string
	HTTP    *http.Client
}

type TaskDefinition struct {
	ProvisionerID string            `json:"provisionerId,omitempty"`
	WorkerType    string            `json:"workerType,omitempty"`
	TaskQueueID   string            `json:"taskQueueId,omitempty"`
	Dependencies  []string          `json:"dependencies,omitempty"`
	Routes        []string          `json:"routes,omitempty"`
	Metadata      TaskMetadata      `json:"metadata,omitempty"`
	Payload       TaskPayload       `json:"payload,omitempty"`
	Extra         map[string]any    `json:"extra,omitempty"`
	Scopes        []string          `json:"scopes,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

type TaskMetadata struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Source      string `json:"source,omitempty"`
}

type TaskPayload struct {
	Command   json.RawMessage   `json:"command,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Artifacts json.RawMessage   `json:"artifacts,omitempty"`
}

func (c TaskclusterClient) Task(ctx context.Context, taskID string) (TaskDefinition, error) {
	root := strings.TrimRight(strings.TrimSpace(c.RootURL), "/")
	if root == "" {
		return TaskDefinition{}, fmt.Errorf("missing Taskcluster root URL; pass --root-url or set TASKCLUSTER_ROOT_URL")
	}
	endpoint := root + "/api/queue/v1/task/" + url.PathEscape(taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return TaskDefinition{}, err
	}
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return TaskDefinition{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail != "" {
			return TaskDefinition{}, fmt.Errorf("Taskcluster task lookup failed: %s: %s", resp.Status, detail)
		}
		return TaskDefinition{}, fmt.Errorf("Taskcluster task lookup failed: %s", resp.Status)
	}
	var task TaskDefinition
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return TaskDefinition{}, err
	}
	return task, nil
}

func CommandArgs(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err == nil {
		return args, nil
	}
	var command string
	if err := json.Unmarshal(raw, &command); err == nil {
		return []string{"sh", "-lc", command}, nil
	}
	var nested [][]string
	if err := json.Unmarshal(raw, &nested); err == nil && len(nested) > 0 {
		return nested[0], nil
	}
	return nil, fmt.Errorf("unsupported task payload command shape")
}

func ArtifactNames(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var byName map[string]json.RawMessage
	if err := json.Unmarshal(raw, &byName); err == nil {
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}
	var list []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &list); err == nil {
		names := make([]string, 0, len(list))
		for _, artifact := range list {
			if artifact.Name != "" {
				names = append(names, artifact.Name)
			} else if artifact.Path != "" {
				names = append(names, artifact.Path)
			}
		}
		sort.Strings(names)
		return names
	}
	return nil
}
