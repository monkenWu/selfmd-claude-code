package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/monkenwu/selfmd/internal/config"
)

// Scan walks the project directory and returns a ScanResult.
func Scan(cfg *config.Config, rootDir string) (*ScanResult, error) {
	var files []string
	totalDirs := 0

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if rel == "." {
			return nil
		}

		// check excludes
		for _, pattern := range cfg.Targets.Exclude {
			matched, _ := doublestar.Match(pattern, rel)
			if matched {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			totalDirs++
			return nil
		}

		// check includes
		if len(cfg.Targets.Include) > 0 {
			included := false
			for _, pattern := range cfg.Targets.Include {
				matched, _ := doublestar.Match(pattern, rel)
				if matched {
					included = true
					break
				}
			}
			if !included {
				return nil
			}
		}

		files = append(files, rel)
		return nil
	})

	if err != nil {
		return nil, err
	}

	projectName := filepath.Base(rootDir)
	tree := BuildTree(projectName, files)

	// read README
	readmeContent := readFileIfExists(rootDir, "README.md")
	if readmeContent == "" {
		readmeContent = readFileIfExists(rootDir, "readme.md")
	}
	if readmeContent == "" {
		readmeContent = readFileIfExists(rootDir, "README")
	}

	// read entry points
	entryPointContents := make(map[string]string)
	for _, ep := range cfg.Targets.EntryPoints {
		content := readFileIfExists(rootDir, ep)
		if content != "" {
			entryPointContents[ep] = content
		}
	}

	return &ScanResult{
		RootDir:            rootDir,
		Tree:               tree,
		FileList:           files,
		TotalFiles:         len(files),
		TotalDirs:          totalDirs,
		ReadmeContent:      readmeContent,
		EntryPointContents: entryPointContents,
	}, nil
}

func readFileIfExists(rootDir, relPath string) string {
	data, err := os.ReadFile(filepath.Join(rootDir, relPath))
	if err != nil {
		return ""
	}
	content := string(data)
	// truncate very large files
	if len(content) > 50000 {
		content = content[:50000] + "\n... (truncated)"
	}
	return content
}

// KeyFiles returns a comma-separated list of notable files found in the scan.
func (s *ScanResult) KeyFiles() string {
	notable := []string{}
	patterns := []string{
		"main.go", "main.py", "main.rs", "main.ts", "main.js",
		"index.ts", "index.js", "app.go", "app.py", "app.ts",
		"Makefile", "Dockerfile", "docker-compose.yml", "compose.yaml",
		"package.json", "go.mod", "Cargo.toml", "pom.xml",
		"README.md", "CHANGELOG.md",
	}

	for _, f := range s.FileList {
		base := filepath.Base(f)
		for _, p := range patterns {
			if strings.EqualFold(base, p) {
				notable = append(notable, f)
				break
			}
		}
	}

	if len(notable) > 20 {
		notable = notable[:20]
	}
	return strings.Join(notable, ", ")
}

// EntryPointsFormatted returns entry point contents formatted for prompts.
func (s *ScanResult) EntryPointsFormatted() string {
	if len(s.EntryPointContents) == 0 {
		return "(no entry points specified)"
	}

	var sb strings.Builder
	for path, content := range s.EntryPointContents {
		sb.WriteString("### " + path + "\n```\n")
		// truncate large files
		if len(content) > 10000 {
			content = content[:10000] + "\n... (truncated)"
		}
		sb.WriteString(content)
		sb.WriteString("\n```\n\n")
	}
	return sb.String()
}
