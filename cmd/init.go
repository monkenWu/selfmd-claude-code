package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monkenwu/selfmd/internal/config"
	"github.com/spf13/cobra"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize selfmd.yaml config file",
	Long:  "Scans the current directory, automatically detects the project type, and generates a selfmd.yaml config file.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&forceInit, "force", false, "Force overwrite of existing config file")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(cfgFile); err == nil && !forceInit {
		return fmt.Errorf("config file %s already exists, use --force to overwrite", cfgFile)
	}

	cfg := config.DefaultConfig()

	projectType, entryPoints := detectProject()
	cfg.Project.Type = projectType
	cfg.Project.Name = filepath.Base(mustCwd())
	cfg.Targets.EntryPoints = entryPoints

	if err := cfg.Save(cfgFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Config file created: %s\n", cfgFile)
	fmt.Printf("  Project name: %s\n", cfg.Project.Name)
	fmt.Printf("  Project type: %s\n", cfg.Project.Type)
	fmt.Printf("  Output dir: %s\n", cfg.Output.Dir)
	fmt.Printf("  Doc language: %s\n", cfg.Output.Language)
	if len(cfg.Output.SecondaryLanguages) > 0 {
		fmt.Printf("  Secondary languages: %s\n", strings.Join(cfg.Output.SecondaryLanguages, ", "))
	}

	if len(cfg.Targets.EntryPoints) > 0 {
		fmt.Printf("  Entry points: %s\n", strings.Join(cfg.Targets.EntryPoints, ", "))
	}

	fmt.Println("\nPlease edit the config file as needed, then run selfmd generate to generate documentation.")
	return nil
}

func detectProject() (projectType string, entryPoints []string) {
	checks := []struct {
		file    string
		pType   string
		entries []string
	}{
		{"go.mod", "backend", []string{"main.go", "cmd/root.go"}},
		{"Cargo.toml", "backend", []string{"src/main.rs", "src/lib.rs"}},
		{"package.json", "frontend", []string{"src/index.ts", "src/index.js", "src/main.ts", "src/App.tsx"}},
		{"pom.xml", "backend", []string{"src/main/java"}},
		{"build.gradle", "backend", []string{"src/main/java"}},
		{"requirements.txt", "backend", []string{"main.py", "app.py", "src/main.py"}},
		{"pyproject.toml", "backend", []string{"src/main.py", "main.py"}},
		{"composer.json", "backend", []string{"public/index.php", "src/Kernel.php"}},
		{"Gemfile", "backend", []string{"config/application.rb", "app/"}},
	}

	for _, c := range checks {
		if _, err := os.Stat(c.file); err == nil {
			var found []string
			for _, ep := range c.entries {
				if _, err := os.Stat(ep); err == nil {
					found = append(found, ep)
				}
			}
			// check if frontend project also has backend
			if c.pType == "frontend" {
				if _, err := os.Stat("go.mod"); err == nil {
					return "fullstack", found
				}
				if _, err := os.Stat("server"); err == nil {
					return "fullstack", found
				}
			}
			return c.pType, found
		}
	}

	return "library", nil
}

func mustCwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
