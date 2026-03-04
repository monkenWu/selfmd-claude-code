package generator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/monkenwu/selfmd/internal/catalog"
	"github.com/monkenwu/selfmd/internal/claude"
	"github.com/monkenwu/selfmd/internal/config"
	"github.com/monkenwu/selfmd/internal/git"
	"github.com/monkenwu/selfmd/internal/output"
	"github.com/monkenwu/selfmd/internal/prompt"
	"github.com/monkenwu/selfmd/internal/scanner"
)

// Generator orchestrates the full documentation generation pipeline.
type Generator struct {
	Config  *config.Config
	Runner  *claude.Runner
	Engine  *prompt.Engine
	Writer  *output.Writer
	Logger  *slog.Logger
	RootDir string // target project root directory

	// stats
	TotalCost   float64
	TotalPages  int
	FailedPages int
}

// NewGenerator creates a new Generator.
func NewGenerator(cfg *config.Config, rootDir string, logger *slog.Logger) (*Generator, error) {
	templateLang := cfg.Output.GetEffectiveTemplateLang()
	engine, err := prompt.NewEngine(templateLang)
	if err != nil {
		return nil, err
	}

	runner := claude.NewRunner(&cfg.Claude, logger)

	absOutDir := cfg.Output.Dir
	if absOutDir == "" {
		absOutDir = ".doc-build"
	}

	writer := output.NewWriter(absOutDir)

	return &Generator{
		Config:  cfg,
		Runner:  runner,
		Engine:  engine,
		Writer:  writer,
		Logger:  logger,
		RootDir: rootDir,
	}, nil
}

// GenerateOptions configures the generation run.
type GenerateOptions struct {
	Clean       bool
	DryRun      bool
	Concurrency int // override max_concurrent if > 0
}

// Generate runs the full 4-phase documentation generation pipeline.
func (g *Generator) Generate(ctx context.Context, opts GenerateOptions) error {
	start := time.Now()

	// Phase 0: Setup
	clean := opts.Clean || g.Config.Output.CleanBeforeGenerate
	if clean {
		fmt.Println("[0/4] Cleaning output directory...")
		if !opts.DryRun {
			if err := g.Writer.Clean(); err != nil {
				return err
			}
		}
	} else {
		if err := g.Writer.EnsureDir(); err != nil {
			return err
		}
	}

	// Phase 1: Scan
	fmt.Println("[1/4] Scanning project structure...")
	scan, err := scanner.Scan(g.Config, g.RootDir)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}
	fmt.Printf("      Found %d files in %d directories\n", scan.TotalFiles, scan.TotalDirs)

	if opts.DryRun {
		fmt.Println("\n[Dry Run] File tree:")
		fmt.Println(scanner.RenderTree(scan.Tree, 3))
		fmt.Println("[Dry Run] No Claude calls will be made.")
		return nil
	}

	// Phase 2: Generate Catalog
	var cat *catalog.Catalog
	if !clean {
		// Try to reuse existing catalog
		catJSON, readErr := g.Writer.ReadCatalogJSON()
		if readErr == nil {
			cat, err = catalog.Parse(catJSON)
		}
		if cat != nil {
			items := cat.Flatten()
			fmt.Printf("[2/4] Loaded existing catalog (%d sections, %d items)\n", len(cat.Items), len(items))
		}
	}
	if cat == nil {
		fmt.Println("[2/4] Generating catalog...")
		cat, err = g.GenerateCatalog(ctx, scan)
		if err != nil {
			return fmt.Errorf("failed to generate catalog: %w", err)
		}
		items := cat.Flatten()
		fmt.Printf("      Catalog: %d sections, %d items\n", len(cat.Items), len(items))

		// Save catalog JSON
		if err := g.Writer.WriteCatalogJSON(cat); err != nil {
			g.Logger.Warn("failed to save catalog JSON", "error", err)
		}
	}

	// Phase 3: Generate Content
	concurrency := g.Config.Claude.MaxConcurrent
	if opts.Concurrency > 0 {
		concurrency = opts.Concurrency
	}
	fmt.Printf("[3/4] Generating content pages (concurrency: %d)...\n", concurrency)
	if err := g.GenerateContent(ctx, scan, cat, concurrency, !clean); err != nil {
		g.Logger.Warn("some pages failed to generate", "error", err)
	}

	// Phase 4: Generate Index & Navigation
	fmt.Println("[4/4] Generating navigation and index...")
	if err := g.GenerateIndex(ctx, cat); err != nil {
		return fmt.Errorf("failed to generate index: %w", err)
	}

	// Build doc metadata for multi-language support
	docMeta := g.buildDocMeta()

	// Generate static viewer (HTML/JS/CSS + _data.js bundle)
	fmt.Println("Generating documentation viewer...")
	if err := g.Writer.WriteViewer(g.Config.Project.Name, docMeta); err != nil {
		g.Logger.Warn("failed to generate viewer", "error", err)
		fmt.Printf("      Viewer generation failed: %v\n", err)
	} else {
		fmt.Println("      Done, open .doc-build/index.html to browse")
	}

	// Write .nojekyll to prevent GitHub Pages from ignoring files starting with _
	if err := g.Writer.WriteFile(".nojekyll", ""); err != nil {
		g.Logger.Warn("failed to write .nojekyll", "error", err)
	}

	// Save current commit for incremental updates
	if git.IsGitRepo(g.RootDir) {
		if commit, err := git.GetCurrentCommit(g.RootDir); err == nil {
			if err := g.Writer.SaveLastCommit(commit); err != nil {
				g.Logger.Warn("failed to save commit record", "error", err)
			}
		}
	}

	elapsed := time.Since(start)
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Documentation generation complete!")
	fmt.Printf("  Output dir: %s\n", g.Config.Output.Dir)
	fmt.Printf("  Pages: %d succeeded", g.TotalPages)
	if g.FailedPages > 0 {
		fmt.Printf(", %d failed", g.FailedPages)
	}
	fmt.Println()
	fmt.Printf("  Total time: %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Total cost: $%.4f USD\n", g.TotalCost)
	fmt.Println("========================================")

	return nil
}

// buildDocMeta constructs language metadata for the documentation viewer.
func (g *Generator) buildDocMeta() *output.DocMeta {
	meta := &output.DocMeta{
		DefaultLanguage: g.Config.Output.Language,
		AvailableLanguages: []output.LangInfo{
			{
				Code:       g.Config.Output.Language,
				NativeName: config.GetLangNativeName(g.Config.Output.Language),
				IsDefault:  true,
			},
		},
	}
	for _, lang := range g.Config.Output.SecondaryLanguages {
		meta.AvailableLanguages = append(meta.AvailableLanguages, output.LangInfo{
			Code:       lang,
			NativeName: config.GetLangNativeName(lang),
			IsDefault:  false,
		})
	}
	return meta
}
