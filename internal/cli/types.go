package cli

import "time"

type options struct {
	Target    string
	TargetSet bool
	Repo      string
	JSON      bool
	Headless  bool
	VNC       bool
	CPU       int
	MemoryMB  int
	DiskGB    int
	Resources bool
}

type syncResult struct {
	RepoRoot    string   `json:"repo_root"`
	RemotePath  string   `json:"remote_path"`
	Fingerprint string   `json:"fingerprint"`
	FileCount   int      `json:"file_count"`
	TotalBytes  int64    `json:"total_bytes"`
	Warnings    []string `json:"warnings,omitempty"`
	Skipped     bool     `json:"skipped"`
	Duration    string   `json:"duration"`
}

type targetView struct {
	Name         string `json:"name"`
	OS           string `json:"os"`
	Version      string `json:"version"`
	Arch         string `json:"arch"`
	Runnable     bool   `json:"runnable"`
	ImageName    string `json:"image_name,omitempty"`
	SourceImage  string `json:"source_image,omitempty"`
	ImagePresent bool   `json:"image_present"`
	CloneCommand string `json:"clone_command,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

type workspaceView struct {
	ID              string     `json:"id"`
	Target          string     `json:"target"`
	RepoRoot        string     `json:"repo_root"`
	VMName          string     `json:"vm_name"`
	CPU             int        `json:"cpu,omitempty"`
	MemoryMB        int        `json:"memory_mb,omitempty"`
	DiskGB          int        `json:"disk_gb,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastRunLog      string     `json:"last_run_log,omitempty"`
	LastKnownIP     string     `json:"last_known_ip,omitempty"`
	SyncFingerprint string     `json:"sync_fingerprint,omitempty"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
}

type runView struct {
	ID        string    `json:"id"`
	Target    string    `json:"target"`
	RepoRoot  string    `json:"repo_root"`
	VMName    string    `json:"vm_name"`
	Command   []string  `json:"command"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	ExitCode  int       `json:"exit_code"`
	OutputLog string    `json:"output_log"`
	StdoutLog string    `json:"stdout_log"`
	StderrLog string    `json:"stderr_log"`
	EventsLog string    `json:"events_log"`
}
