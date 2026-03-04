package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/monkenwu/selfmd/internal/claude"
	"github.com/monkenwu/selfmd/internal/config"
	"github.com/monkenwu/selfmd/internal/generator"
	"github.com/spf13/cobra"
)

var (
	translateLangs []string
	translateForce bool
	translateConc  int
)

var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Translate primary language docs to secondary languages",
	Long: `Translates generated documentation in the primary language to the secondary languages defined in config.
Translation results are placed in .doc-build/{language-code}/ subdirectories.`,
	RunE: runTranslate,
}

func init() {
	translateCmd.Flags().StringSliceVar(&translateLangs, "lang", nil, "only translate specified languages (default: all secondary languages)")
	translateCmd.Flags().BoolVar(&translateForce, "force", false, "force re-translate existing files")
	translateCmd.Flags().IntVar(&translateConc, "concurrency", 0, "concurrency (override config)")
	rootCmd.AddCommand(translateCmd)
}

func runTranslate(cmd *cobra.Command, args []string) error {
	if err := claude.CheckAvailable(); err != nil {
		return err
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	if len(cfg.Output.SecondaryLanguages) == 0 {
		return fmt.Errorf("%s", "secondary_languages not defined in config, cannot translate")
	}

	// Determine target languages
	targetLangs := cfg.Output.SecondaryLanguages
	if len(translateLangs) > 0 {
		// Validate specified languages are in the config
		validLangs := make(map[string]bool)
		for _, l := range cfg.Output.SecondaryLanguages {
			validLangs[l] = true
		}
		for _, l := range translateLangs {
			if !validLangs[l] {
				return fmt.Errorf("language %s is not in secondary_languages list (available: %s)", l, strings.Join(cfg.Output.SecondaryLanguages, ", "))
			}
		}
		targetLangs = translateLangs
	}

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	if quiet {
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	gen, err := generator.NewGenerator(cfg, rootDir, logger)
	if err != nil {
		return err
	}

	concurrency := cfg.Claude.MaxConcurrent
	if translateConc > 0 {
		concurrency = translateConc
	}

	return gen.Translate(ctx, generator.TranslateOptions{
		TargetLanguages: targetLangs,
		Force:           translateForce,
		Concurrency:     concurrency,
	})
}
