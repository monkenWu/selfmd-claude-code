package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/monkenwu/selfmd/internal/claude"
	"github.com/monkenwu/selfmd/internal/config"
	"github.com/monkenwu/selfmd/internal/generator"
	"github.com/spf13/cobra"
)

var (
	cleanFlag      bool
	noCleanFlag    bool
	dryRun         bool
	concurrencyNum int
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate the complete project documentation",
	Long: `Run the four-phase documentation generation flow:
  1. Scan project structure
  2. Generate documentation catalog
  3. Generate content pages (concurrent)
  4. Generate navigation and index`,
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().BoolVar(&cleanFlag, "clean", false, "Force clean the output directory")
	generateCmd.Flags().BoolVar(&noCleanFlag, "no-clean", false, "Do not clean the output directory")
	generateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show plan only, do not execute")
	generateCmd.Flags().IntVar(&concurrencyNum, "concurrency", 0, "Concurrency (overrides config file)")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Check claude CLI availability
	if err := claude.CheckAvailable(); err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	// Setup logger
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	if quiet {
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	// Setup context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Get working directory
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Create generator
	gen, err := generator.NewGenerator(cfg, rootDir, logger)
	if err != nil {
		return err
	}

	// Determine clean option
	clean := cfg.Output.CleanBeforeGenerate
	if cleanFlag {
		clean = true
	}
	if noCleanFlag {
		clean = false
	}

	opts := generator.GenerateOptions{
		Clean:       clean,
		DryRun:      dryRun,
		Concurrency: concurrencyNum,
	}

	return gen.Generate(ctx, opts)
}
