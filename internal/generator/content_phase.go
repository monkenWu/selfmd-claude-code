package generator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/monkenwu/selfmd/internal/catalog"
	"github.com/monkenwu/selfmd/internal/claude"
	"github.com/monkenwu/selfmd/internal/config"
	"github.com/monkenwu/selfmd/internal/output"
	"github.com/monkenwu/selfmd/internal/prompt"
	"github.com/monkenwu/selfmd/internal/scanner"
	"golang.org/x/sync/errgroup"
)

// GenerateContent generates documentation pages for all catalog items concurrently.
// When skipExisting is true, pages that already exist on disk are skipped.
func (g *Generator) GenerateContent(ctx context.Context, scan *scanner.ScanResult, cat *catalog.Catalog, concurrency int, skipExisting bool) error {
	items := cat.Flatten()
	total := len(items)

	// Build the catalog link table once for all pages
	catalogTable := cat.BuildLinkTable()

	// Build the link fixer once for all pages
	linkFixer := output.NewLinkFixer(cat)

	var done atomic.Int32
	var failed atomic.Int32
	var skipped atomic.Int32
	var costMu sync.Mutex

	eg, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, concurrency)

	for _, item := range items {
		item := item
		eg.Go(func() error {
			// Skip existing pages when not cleaning
			if skipExisting && g.Writer.PageExists(item) {
				skipped.Add(1)
				fmt.Printf("      [Skip] %s (exists)\n", item.Title)
				return nil
			}

			sem <- struct{}{}
			defer func() { <-sem }()

			idx := done.Load() + failed.Load() + 1
			fmt.Printf("      [%d/%d] %s（%s）...", idx, total-int(skipped.Load()), item.Title, item.Path)

			err := g.generateSinglePage(ctx, scan, item, catalogTable, linkFixer, "")
			if err != nil {
				failed.Add(1)
				fmt.Printf(" Failed: %v\n", err)
				g.Logger.Warn("page generation failed",
					"title", item.Title,
					"path", item.Path,
					"error", err,
				)
				g.writePlaceholder(item, err)
				return nil // don't abort other pages
			}

			done.Add(1)
			costMu.Lock()
			costMu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	g.TotalPages = int(done.Load())
	g.FailedPages = int(failed.Load())

	if s := skipped.Load(); s > 0 {
		fmt.Printf("      Skipped %d existing pages\n", s)
	}

	return nil
}

func (g *Generator) generateSinglePage(ctx context.Context, scan *scanner.ScanResult, item catalog.FlatItem, catalogTable string, linkFixer *output.LinkFixer, existingContent string) error {
	langName := config.GetLangNativeName(g.Config.Output.Language)
	data := prompt.ContentPromptData{
		RepositoryName:       g.Config.Project.Name,
		Language:             g.Config.Output.Language,
		LanguageName:         langName,
		LanguageOverride:     g.Config.Output.NeedsLanguageOverride(),
		LanguageOverrideName: langName,
		CatalogPath:          item.Path,
		CatalogTitle:         item.Title,
		CatalogDirPath:       item.DirPath,
		ProjectDir:           g.RootDir,
		FileTree:             scanner.RenderTree(scan.Tree, 3),
		CatalogTable:         catalogTable,
		ExistingContent:      existingContent,
	}

	rendered, err := g.Engine.RenderContent(data)
	if err != nil {
		return err
	}

	maxAttempts := 2
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := g.Runner.RunWithRetry(ctx, claude.RunOptions{
			Prompt:  rendered,
			WorkDir: g.RootDir,
		})
		if err != nil {
			return err
		}

		g.TotalCost += result.CostUSD

		// Extract content from <document> tag
		content, extractErr := claude.ExtractDocumentTag(result.Content)
		if extractErr != nil {
			lastErr = fmt.Errorf("failed to extract document content: %w", extractErr)
			if attempt < maxAttempts {
				fmt.Printf(" Format error, retrying...\n      ")
				continue
			}
			fmt.Printf(" Failed (format error)\n")
			return lastErr
		}

		content = strings.TrimSpace(content)
		if content == "" || !strings.HasPrefix(content, "#") {
			lastErr = fmt.Errorf("Claude did not output valid Markdown document (missing heading)")
			if attempt < maxAttempts {
				fmt.Printf(" Invalid content, retrying...\n      ")
				continue
			}
			fmt.Printf(" Failed (invalid content)\n")
			return lastErr
		}

		fmt.Printf(" Done (%.1fs, $%.4f)\n", float64(result.DurationMs)/1000, result.CostUSD)

		// Post-process: fix broken links
		content = linkFixer.FixLinks(content, item.DirPath)

		return g.Writer.WritePage(item, content)
	}

	return lastErr
}

func (g *Generator) writePlaceholder(item catalog.FlatItem, genErr error) {
	content := fmt.Sprintf("# %s\n\n> This page failed to generate. Please re-run `selfmd generate`.\n>\n> Error: %v\n", item.Title, genErr)
	if err := g.Writer.WritePage(item, content); err != nil {
		g.Logger.Warn("failed to write placeholder page", "path", item.Path, "error", err)
	}
}
