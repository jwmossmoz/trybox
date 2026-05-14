package ci

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type ReplayPlan struct {
	Provider        string            `json:"provider"`
	RootURL         string            `json:"root_url"`
	TaskID          string            `json:"task_id"`
	Name            string            `json:"name,omitempty"`
	Source          string            `json:"source,omitempty"`
	ProvisionerID   string            `json:"provisioner_id,omitempty"`
	WorkerType      string            `json:"worker_type,omitempty"`
	TaskQueueID     string            `json:"task_queue_id,omitempty"`
	Target          string            `json:"target,omitempty"`
	TargetSupported bool              `json:"target_supported"`
	Unsupported     string            `json:"unsupported,omitempty"`
	Command         []string          `json:"command,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Dependencies    []string          `json:"dependencies,omitempty"`
	Artifacts       []string          `json:"artifacts,omitempty"`
	Warnings        []string          `json:"warnings,omitempty"`
}

func NewReplayPlan(rootURL, taskID string, task TaskDefinition) (ReplayPlan, error) {
	command, err := CommandArgs(task.Payload.Command)
	if err != nil {
		return ReplayPlan{}, err
	}
	target, supported, unsupported := TargetForTask(task)
	warnings := []string{}
	if len(command) == 0 {
		warnings = append(warnings, "task payload has no command")
	}
	if len(task.Dependencies) > 0 {
		warnings = append(warnings, "dependency build artifacts are summarized but not downloaded automatically yet")
	}
	env := copyEnv(task.Payload.Env)
	return ReplayPlan{
		Provider:        "taskcluster",
		RootURL:         strings.TrimRight(rootURL, "/"),
		TaskID:          taskID,
		Name:            task.Metadata.Name,
		Source:          task.Metadata.Source,
		ProvisionerID:   task.ProvisionerID,
		WorkerType:      task.WorkerType,
		TaskQueueID:     queueID(task),
		Target:          target,
		TargetSupported: supported,
		Unsupported:     unsupported,
		Command:         command,
		Env:             env,
		Dependencies:    append([]string(nil), task.Dependencies...),
		Artifacts:       ArtifactNames(task.Payload.Artifacts),
		Warnings:        warnings,
	}, nil
}

func TargetForTask(task TaskDefinition) (string, bool, string) {
	queue := strings.ToLower(queueID(task))
	major := macOSMajor(queue)
	if major == "" {
		if strings.Contains(queue, "win") || strings.Contains(queue, "windows") {
			return "", false, "Windows tasks do not have a compatible Trybox target yet"
		}
		if strings.Contains(queue, "linux") || strings.Contains(queue, "ubuntu") {
			return "", false, "Linux tasks do not have a compatible Trybox target yet"
		}
		return "", false, "no built-in Trybox target mapping for task queue"
	}
	arch := "arm64"
	if strings.Contains(queue, "x86_64") || strings.Contains(queue, "amd64") || strings.Contains(queue, "x64") {
		arch = "x64-rosetta"
	}
	return "macos" + major + "-" + arch, true, ""
}

func queueID(task TaskDefinition) string {
	if task.TaskQueueID != "" {
		return task.TaskQueueID
	}
	switch {
	case task.ProvisionerID != "" && task.WorkerType != "":
		return task.ProvisionerID + "/" + task.WorkerType
	case task.WorkerType != "":
		return task.WorkerType
	default:
		return task.ProvisionerID
	}
}

func macOSMajor(queue string) string {
	patterns := []struct {
		re    *regexp.Regexp
		major string
	}{
		{regexp.MustCompile(`macosx?[-_]?2600|macos[-_]?26|osx[-_]?2600`), "26"},
		{regexp.MustCompile(`macosx?[-_]?1500|macos[-_]?15|osx[-_]?1500`), "15"},
		{regexp.MustCompile(`macosx?[-_]?1400|macos[-_]?14|osx[-_]?1400`), "14"},
		{regexp.MustCompile(`macosx?[-_]?1300|macos[-_]?13|osx[-_]?1300`), "13"},
		{regexp.MustCompile(`macosx?[-_]?1200|macos[-_]?12|osx[-_]?1200`), "12"},
	}
	for _, pattern := range patterns {
		if pattern.re.MatchString(queue) {
			return pattern.major
		}
	}
	return ""
}

func EnvCommand(env map[string]string, command []string) []string {
	if len(env) == 0 {
		return append([]string(nil), command...)
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []string{"env"}
	for _, key := range keys {
		out = append(out, fmt.Sprintf("%s=%s", key, env[key]))
	}
	out = append(out, command...)
	return out
}

func ShellPrelude(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, "export "+shellAssign(key, env[key]))
	}
	return strings.Join(parts, " && ")
}

func shellAssign(key, value string) string {
	return key + "=" + shellQuote(value)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func copyEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[key] = value
	}
	return out
}
