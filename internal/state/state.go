package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	Root      string
	ClaimsDir string
	RunsDir   string
	LogsDir   string
	KeysDir   string
}

type Claim struct {
	ID              string    `json:"id"`
	Target          string    `json:"target"`
	Backend         string    `json:"backend"`
	VMName          string    `json:"vm_name"`
	RepoRoot        string    `json:"repo_root"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastRunLog      string    `json:"last_run_log,omitempty"`
	LastKnownIP     string    `json:"last_known_ip,omitempty"`
	SyncFingerprint string    `json:"sync_fingerprint,omitempty"`
	LastSyncAt      time.Time `json:"last_sync_at,omitempty"`
}

type Run struct {
	ID        string    `json:"id"`
	ClaimID   string    `json:"claim_id"`
	Target    string    `json:"target"`
	VMName    string    `json:"vm_name"`
	RepoRoot  string    `json:"repo_root"`
	Command   []string  `json:"command"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	ExitCode  int       `json:"exit_code"`
	StdoutLog string    `json:"stdout_log"`
	StderrLog string    `json:"stderr_log"`
	EventsLog string    `json:"events_log"`
}

func DefaultStore() (Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	root := filepath.Join(configDir, "trybox")
	return Store{
		Root:      root,
		ClaimsDir: filepath.Join(root, "claims"),
		RunsDir:   filepath.Join(root, "runs"),
		LogsDir:   filepath.Join(root, "logs"),
		KeysDir:   filepath.Join(root, "keys"),
	}, nil
}

func (s Store) Init() error {
	for _, dir := range []string{s.Root, s.ClaimsDir, s.RunsDir, s.LogsDir, s.KeysDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) ClaimPath(id string) string {
	return filepath.Join(s.ClaimsDir, id+".json")
}

func (s Store) SaveClaim(claim Claim) error {
	claim.UpdatedAt = time.Now().UTC()
	if claim.CreatedAt.IsZero() {
		claim.CreatedAt = claim.UpdatedAt
	}
	data, err := json.MarshalIndent(claim, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.ClaimPath(claim.ID), append(data, '\n'), 0o644)
}

func (s Store) LoadClaim(id string) (Claim, error) {
	data, err := os.ReadFile(s.ClaimPath(id))
	if err != nil {
		return Claim{}, err
	}
	var claim Claim
	if err := json.Unmarshal(data, &claim); err != nil {
		return Claim{}, err
	}
	return claim, nil
}

func (s Store) RemoveClaim(id string) error {
	err := os.Remove(s.ClaimPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s Store) RunDir(id string) string {
	return filepath.Join(s.RunsDir, id)
}

func (s Store) KeyDir(claimID string) string {
	return filepath.Join(s.KeysDir, claimID)
}

func (s Store) NewRun(claim Claim, command []string) (Run, error) {
	id := "run_" + time.Now().UTC().Format("20060102T150405")
	dir := s.RunDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Run{}, err
	}
	return Run{
		ID:        id,
		ClaimID:   claim.ID,
		Target:    claim.Target,
		VMName:    claim.VMName,
		RepoRoot:  claim.RepoRoot,
		Command:   command,
		StartedAt: time.Now().UTC(),
		ExitCode:  -1,
		StdoutLog: filepath.Join(dir, "stdout.log"),
		StderrLog: filepath.Join(dir, "stderr.log"),
		EventsLog: filepath.Join(dir, "events.ndjson"),
	}, nil
}

func (s Store) SaveRun(run Run) error {
	dir := s.RunDir(run.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), append(data, '\n'), 0o644)
}

func (s Store) AppendEvent(run Run, event string, payload any) error {
	f, err := os.OpenFile(run.EventsLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"event": event,
	}
	if payload != nil {
		entry["payload"] = payload
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, string(data))
	return err
}

func ClaimID(targetName, repoRoot string) string {
	return "claim_" + slug(targetName) + "_" + slug(filepath.Base(repoRoot))
}

func slug(input string) string {
	out := make([]rune, 0, len(input))
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+'a'-'A')
		case r >= '0' && r <= '9':
			out = append(out, r)
		default:
			if len(out) == 0 || out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return "x"
	}
	return string(out)
}
