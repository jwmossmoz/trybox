package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || rel == "" {
			continue
		}
		if isExcluded(rel, excludes) {
			excluded = append(excluded, rel)
			continue
		}
		info, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return Plan{}, err
		}
		if info.IsDir() {
			continue
		}
		files = append(files, File{Path: rel, Size: info.Size(), ModTimeUnixNano: info.ModTime().UnixNano()})
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
	plan.Fingerprint = fingerprint(files)
	plan.LargestFiles = largestFiles(files, limit)
	plan.LargestDirs = largestDirs(files, limit)
	plan.ManifestPreview = preview(files, limit)
	return plan, nil
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
		".git/",
		".hg/",
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
