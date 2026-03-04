package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// IsGitRepo checks if the given directory is a git repository.
func IsGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}

// GetCurrentCommit returns the current HEAD commit hash.
func GetCurrentCommit(dir string) (string, error) {
	return runGit(dir, "rev-parse", "HEAD")
}

// GetMergeBase finds the merge base between the current branch and the given base branch.
func GetMergeBase(dir, baseBranch string) (string, error) {
	return runGit(dir, "merge-base", baseBranch, "HEAD")
}

// GetChangedFiles returns the list of changed files between two commits.
// Uses --relative so paths are relative to the working directory (dir), not the git repo root.
func GetChangedFiles(dir, fromCommit, toCommit string) (string, error) {
	return runGit(dir, "diff", "--relative", "--name-status", fromCommit+".."+toCommit)
}

// GetChangedFilesSince returns changed files since the given commit.
// Uses --relative so paths are relative to the working directory (dir), not the git repo root.
func GetChangedFilesSince(dir, sinceCommit string) (string, error) {
	return runGit(dir, "diff", "--relative", "--name-status", sinceCommit+"..HEAD")
}

// GetLastCommitForFile returns the last commit hash that modified the given file.
func GetLastCommitForFile(dir, filePath string) (string, error) {
	return runGit(dir, "log", "-1", "--format=%H", "--", filePath)
}

// ChangedFile represents a single file from git diff --name-status output.
type ChangedFile struct {
	Status string // "M", "A", "D", "R"
	Path   string
}

// ParseChangedFiles parses git diff --name-status output into structured ChangedFile list.
func ParseChangedFiles(changedFiles string) []ChangedFile {
	var result []ChangedFile
	for _, line := range strings.Split(changedFiles, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		status := string(parts[0][0]) // "M", "A", "D", or "R" (R100 → R)
		path := parts[len(parts)-1]   // for renames, use destination path
		result = append(result, ChangedFile{Status: status, Path: path})
	}
	return result
}

// FilterChangedFiles filters git diff --name-status output using include/exclude glob patterns.
func FilterChangedFiles(changedFiles string, includes, excludes []string) string {
	lines := strings.Split(changedFiles, "\n")
	var filtered []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// git diff --name-status format: "M\tpath/to/file" or "R100\told\tnew"
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}

		// For renames, check the destination path (last element)
		filePath := parts[len(parts)-1]

		// Check excludes
		excluded := false
		for _, pattern := range excludes {
			if matched, _ := doublestar.Match(pattern, filePath); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Check includes (if configured)
		if len(includes) > 0 {
			included := false
			for _, pattern := range includes {
				if matched, _ := doublestar.Match(pattern, filePath); matched {
					included = true
					break
				}
			}
			if !included {
				continue
			}
		}

		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n")
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}
