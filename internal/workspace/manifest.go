package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Path            string `json:"path"`
	Size            int64  `json:"size"`
	ModTimeUnixNano int64  `json:"-"`
}

type Plan struct {
	RepoRoot        string   `json:"repo_root"`
	FileCount       int      `json:"file_count"`
	TotalBytes      int64    `json:"total_bytes"`
	Fingerprint     string   `json:"fingerprint"`
	ChangedTracked  []string `json:"changed_tracked"`
	Untracked       []string `json:"untracked"`
	Warnings        []string `json:"warnings,omitempty"`
	LargestFiles    []File   `json:"largest_files"`
	LargestDirs     []File   `json:"largest_dirs"`
	Excluded        []string `json:"excluded"`
	ManifestPreview []string `json:"manifest_preview,omitempty"`

	files []File
}

func BuildPlan(ctx context.Context, repo string, limit int) (Plan, error) {
	repo, err := filepath.Abs(repo)
	if err != nil {
		return Plan{}, err
	}
	if err := ValidateRepoRoot(repo); err != nil {
		return Plan{}, err
	}
	paths, err := gitNUL(ctx, repo, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return Plan{}, err
	}
	changed, _ := gitNUL(ctx, repo, "ls-files", "-z", "-m")
	untracked, _ := gitNUL(ctx, repo, "ls-files", "-z", "-o", "--exclude-standard")

	excludes, err := loadExcludes(repo)
	if err != nil {
		return Plan{}, err
	}

	files := make([]File, 0, len(paths))
	excluded := []string{}
	for _, rel := range paths {
		file, skipped, err := planFile(repo, rel, excludes)
		if err != nil {
			return Plan{}, err
		}
		if skipped != "" {
			excluded = append(excluded, skipped)
			continue
		}
		files = append(files, file)
	}
	for _, metadataDir := range []string{".git", ".hg"} {
		metadataFiles, metadataExcluded, err := metadataFiles(repo, metadataDir, excludes)
		if err != nil {
			return Plan{}, err
		}
		files = append(files, metadataFiles...)
		excluded = append(excluded, metadataExcluded...)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	plan := Plan{
		RepoRoot:       repo,
		FileCount:      len(files),
		ChangedTracked: changed,
		Untracked:      untracked,
		Excluded:       capList(excluded, limit),
		files:          files,
	}
	for _, file := range files {
		plan.TotalBytes += file.Size
	}
	plan.Warnings = warnings(plan.TotalBytes)
	plan.Fingerprint = fingerprint(files)
	plan.LargestFiles = largestFiles(files, limit)
	plan.LargestDirs = largestDirs(files, limit)
	plan.ManifestPreview = preview(files, limit)
	return plan, nil
}

func ValidateRepoRoot(repo string) error {
	gitPath := filepath.Join(repo, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return err
	}
	gitdir, ok := parseGitdir(data)
	if !ok {
		return nil
	}
	return fmt.Errorf("repo at %s is a git worktree (.git is a gitfile pointing to %s); trybox requires a standalone clone. Run:\n  git clone --no-local --no-hardlinks %s <new-location>\nand use that as the workspace repo", repo, gitdir, repo)
}

func parseGitdir(data []byte) (string, bool) {
	line, _, _ := strings.Cut(string(data), "\n")
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(line), "gitdir:")
	if !ok {
		return "", false
	}
	gitdir = strings.TrimSpace(gitdir)
	if gitdir == "" {
		return "", false
	}
	return gitdir, true
}

func (p Plan) NULManifest() []byte {
	var buf bytes.Buffer
	for _, file := range p.files {
		buf.WriteString(file.Path)
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

func (p Plan) Files() []File {
	out := make([]File, len(p.files))
	copy(out, p.files)
	return out
}

func gitNUL(ctx context.Context, repo string, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(out, []byte{0})
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		values = append(values, filepath.ToSlash(string(part)))
	}
	sort.Strings(values)
	return values, nil
}

func loadExcludes(repo string) ([]string, error) {
	excludes := []string{
		".trybox/",
		"node_modules/",
		"playwright-report/",
		"test-results/",
	}
	data, err := os.ReadFile(filepath.Join(repo, ".tryboxignore"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return excludes, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		excludes = append(excludes, filepath.ToSlash(line))
	}
	return excludes, nil
}

func metadataFiles(repo, relRoot string, excludes []string) ([]File, []string, error) {
	root := filepath.Join(repo, relRoot)
	info, err := os.Lstat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if !info.IsDir() {
		file, skipped, err := planFile(repo, relRoot, excludes)
		if err != nil || skipped != "" {
			return nil, nonEmpty(skipped), err
		}
		return []File{file}, nil, nil
	}

	files := []File{}
	excluded := []string{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		file, skipped, err := planFile(repo, rel, excludes)
		if err != nil {
			return err
		}
		if skipped != "" {
			excluded = append(excluded, skipped)
			return nil
		}
		files = append(files, file)
		return nil
	})
	return files, excluded, err
}

func planFile(repo, rel string, excludes []string) (File, string, error) {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return File{}, rel, nil
	}
	if isExcluded(rel, excludes) {
		return File{}, rel, nil
	}
	info, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(rel)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, rel, nil
		}
		return File{}, "", err
	}
	if info.IsDir() {
		return File{}, rel, nil
	}
	mode := info.Mode()
	if !mode.IsRegular() && mode&os.ModeSymlink == 0 {
		return File{}, rel, nil
	}
	return File{Path: rel, Size: info.Size(), ModTimeUnixNano: info.ModTime().UnixNano()}, "", nil
}

func warnings(totalBytes int64) []string {
	const largeSyncThreshold = 10 * 1024 * 1024 * 1024
	if totalBytes <= largeSyncThreshold {
		return nil
	}
	return []string{
		fmt.Sprintf("planned sync is %.1f GiB; first sync may take several minutes", float64(totalBytes)/(1024*1024*1024)),
	}
}

func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func isExcluded(path string, excludes []string) bool {
	for _, pattern := range excludes {
		pattern = strings.TrimSpace(filepath.ToSlash(pattern))
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/") {
			prefix := strings.TrimSuffix(pattern, "/")
			if path == prefix || strings.HasPrefix(path, pattern) {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if path == pattern || strings.HasPrefix(path, pattern+"/") {
			return true
		}
	}
	return false
}

func fingerprint(files []File) string {
	h := sha256.New()
	for _, file := range files {
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00", file.Path, file.Size, file.ModTimeUnixNano)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func largestFiles(files []File, limit int) []File {
	if limit < 0 {
		limit = 0
	}
	out := make([]File, len(files))
	copy(out, files)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size == out[j].Size {
			return out[i].Path < out[j].Path
		}
		return out[i].Size > out[j].Size
	})
	return capList(out, limit)
}

func largestDirs(files []File, limit int) []File {
	sizes := map[string]int64{}
	for _, file := range files {
		dir := topDir(file.Path)
		sizes[dir] += file.Size
	}
	dirs := make([]File, 0, len(sizes))
	for dir, size := range sizes {
		dirs = append(dirs, File{Path: dir, Size: size})
	}
	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].Size == dirs[j].Size {
			return dirs[i].Path < dirs[j].Path
		}
		return dirs[i].Size > dirs[j].Size
	})
	return capList(dirs, limit)
}

func topDir(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return "."
	}
	if len(parts) == 2 {
		return parts[0]
	}
	return parts[0] + "/" + parts[1]
}

func preview(files []File, limit int) []string {
	if limit < 0 {
		limit = 0
	}
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, file.Path)
	}
	return capList(out, limit)
}

func capList[T any](values []T, limit int) []T {
	if limit == 0 || len(values) <= limit {
		out := make([]T, len(values))
		copy(out, values)
		return out
	}
	out := make([]T, limit)
	copy(out, values[:limit])
	return out
}
