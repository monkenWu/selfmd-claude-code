package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/monkenwu/selfmd/internal/catalog"
	"github.com/monkenwu/selfmd/internal/claude"
	"github.com/monkenwu/selfmd/internal/config"
	"github.com/monkenwu/selfmd/internal/output"
	"github.com/monkenwu/selfmd/internal/prompt"
	"golang.org/x/sync/errgroup"
)

// TranslateOptions configures the translation run.
type TranslateOptions struct {
	TargetLanguages []string
	Force           bool
	Concurrency     int
}

// Translate runs the translation pipeline for all target languages.
func (g *Generator) Translate(ctx context.Context, opts TranslateOptions) error {
	start := time.Now()

	// Read master catalog
	catJSON, err := g.Writer.ReadCatalogJSON()
	if err != nil {
		return fmt.Errorf("failed to read catalog (please run selfmd generate first): %w", err)
	}

	cat, err := catalog.Parse(catJSON)
	if err != nil {
		return fmt.Errorf("failed to parse catalog: %w", err)
	}

	items := cat.Flatten()
	sourceLang := g.Config.Output.Language
	sourceLangName := config.GetLangNativeName(sourceLang)

	fmt.Printf("Source language: %s (%s)\n", sourceLangName, sourceLang)
	fmt.Printf("Target languages: %s\n", strings.Join(opts.TargetLanguages, ", "))
	fmt.Printf("Page count: %d\n\n", len(items))

	for _, targetLang := range opts.TargetLanguages {
		targetLangName := config.GetLangNativeName(targetLang)
		fmt.Printf("========== Translating to %s (%s) ==========\n", targetLangName, targetLang)

		langWriter := g.Writer.ForLanguage(targetLang)
		if err := langWriter.EnsureDir(); err != nil {
			return fmt.Errorf("failed to create language directory: %w", err)
		}

		translatedTitles := g.translatePages(ctx, items, langWriter, sourceLang, sourceLangName, targetLang, targetLangName, opts)

		// Translate category titles (items with children that weren't translated above)
		categoryTitles, err := g.translateCategoryTitles(ctx, items, translatedTitles, sourceLang, sourceLangName, targetLang, targetLangName)
		if err != nil {
			g.Logger.Warn("failed to translate category titles", "lang", targetLang, "error", err)
		} else {
			for k, v := range categoryTitles {
				translatedTitles[k] = v
			}
		}

		// Build translated catalog
		translatedCat := buildTranslatedCatalog(cat, translatedTitles)
		if err := langWriter.WriteCatalogJSON(translatedCat); err != nil {
			g.Logger.Warn("failed to save translated catalog", "lang", targetLang, "error", err)
		}

		// Generate translated index and sidebar
		indexContent := output.GenerateIndex(
			g.Config.Project.Name,
			g.Config.Project.Description,
			translatedCat,
			targetLang,
		)
		if err := langWriter.WriteFile("index.md", indexContent); err != nil {
			g.Logger.Warn("failed to write translated index", "lang", targetLang, "error", err)
		}

		sidebarContent := output.GenerateSidebar(g.Config.Project.Name, translatedCat, targetLang)
		if err := langWriter.WriteFile("_sidebar.md", sidebarContent); err != nil {
			g.Logger.Warn("failed to write translated sidebar", "lang", targetLang, "error", err)
		}

		// Generate category index pages
		translatedItems := translatedCat.Flatten()
		for _, item := range translatedItems {
			if !item.HasChildren {
				continue
			}
			var children []catalog.FlatItem
			for _, child := range translatedItems {
				if child.ParentPath == item.Path && child.Path != item.Path {
					children = append(children, child)
				}
			}
			if len(children) > 0 {
				categoryContent := output.GenerateCategoryIndex(item, children, targetLang)
				if err := langWriter.WritePage(item, categoryContent); err != nil {
					g.Logger.Warn("failed to write translated category index", "path", item.Path, "error", err)
				}
			}
		}

		fmt.Println()
	}

	// Regenerate viewer bundle with all language data
	docMeta := g.buildDocMeta()
	fmt.Println("Regenerating documentation viewer...")
	if err := g.Writer.WriteViewer(g.Config.Project.Name, docMeta); err != nil {
		g.Logger.Warn("failed to generate viewer", "error", err)
	} else {
		fmt.Println("      Done")
	}

	elapsed := time.Since(start)
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Translation complete!")
	fmt.Printf("  Total time: %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Total cost: $%.4f USD\n", g.TotalCost)
	fmt.Println("========================================")

	return nil
}

// translatePages translates all pages for a single target language.
// Returns a map of catalogPath -> translated title.
func (g *Generator) translatePages(
	ctx context.Context,
	items []catalog.FlatItem,
	langWriter *output.Writer,
	sourceLang, sourceLangName, targetLang, targetLangName string,
	opts TranslateOptions,
) map[string]string {
	translatedTitles := make(map[string]string)
	var titlesMu sync.Mutex

	var done atomic.Int32
	var skipped atomic.Int32
	var failed atomic.Int32

	// Only translate leaf items (non-category pages)
	var leafItems []catalog.FlatItem
	for _, item := range items {
		if !item.HasChildren {
			leafItems = append(leafItems, item)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, opts.Concurrency)

	for _, item := range leafItems {
		item := item
		eg.Go(func() error {
			// Skip if already translated and not forcing
			if !opts.Force && langWriter.PageExists(item) {
				skipped.Add(1)
				// Try to extract title from existing translation
				if content, err := langWriter.ReadPage(item); err == nil {
					if title := extractTitle(content); title != "" {
						titlesMu.Lock()
						translatedTitles[item.Path] = title
						titlesMu.Unlock()
					}
				}
				fmt.Printf("      [Skip] %s (exists)\n", item.Title)
				return nil
			}

			sem <- struct{}{}
			defer func() { <-sem }()

			idx := done.Load() + failed.Load() + 1
			fmt.Printf("      [%d/%d] %s...", idx, len(leafItems)-int(skipped.Load()), item.Title)

			// Read source content
			sourceContent, err := g.Writer.ReadPage(item)
			if err != nil {
				failed.Add(1)
				fmt.Printf(" Failed (read source): %v\n", err)
				return nil
			}

			// Render translate prompt
			data := prompt.TranslatePromptData{
				SourceLanguage:     sourceLang,
				SourceLanguageName: sourceLangName,
				TargetLanguage:     targetLang,
				TargetLanguageName: targetLangName,
				SourceContent:      sourceContent,
			}

			rendered, err := g.Engine.RenderTranslate(data)
			if err != nil {
				failed.Add(1)
				fmt.Printf(" Failed (render template): %v\n", err)
				return nil
			}

			// Call Claude
			result, err := g.Runner.RunWithRetry(ctx, claude.RunOptions{
				Prompt:  rendered,
				WorkDir: g.RootDir,
			})
			if err != nil {
				failed.Add(1)
				fmt.Printf(" Failed: %v\n", err)
				return nil
			}

			g.TotalCost += result.CostUSD

			// Extract translated content
			content, err := claude.ExtractDocumentTag(result.Content)
			if err != nil {
				failed.Add(1)
				fmt.Printf(" Failed (format error): %v\n", err)
				return nil
			}

			content = strings.TrimSpace(content)
			if content == "" {
				failed.Add(1)
				fmt.Printf(" Failed (empty content)\n")
				return nil
			}

			// Extract translated title
			if title := extractTitle(content); title != "" {
				titlesMu.Lock()
				translatedTitles[item.Path] = title
				titlesMu.Unlock()
			}

			// Write translated page
			if err := langWriter.WritePage(item, content); err != nil {
				failed.Add(1)
				fmt.Printf(" Failed (write): %v\n", err)
				return nil
			}

			done.Add(1)
			fmt.Printf(" Done (%.1fs, $%.4f)\n", float64(result.DurationMs)/1000, result.CostUSD)
			return nil
		})
	}

	eg.Wait()

	total := done.Load()
	failCount := failed.Load()
	skipCount := skipped.Load()
	fmt.Printf("      Translation result: %d succeeded", total)
	if failCount > 0 {
		fmt.Printf(", %d failed", failCount)
	}
	if skipCount > 0 {
		fmt.Printf(", %d skipped", skipCount)
	}
	fmt.Println()

	return translatedTitles
}

// extractTitle extracts the first # heading from markdown content.
func extractTitle(content string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	match := re.FindStringSubmatch(content)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// buildTranslatedCatalog creates a copy of the catalog with translated titles.
func buildTranslatedCatalog(original *catalog.Catalog, translatedTitles map[string]string) *catalog.Catalog {
	translated := &catalog.Catalog{
		Items: translateCatalogItems(original.Items, translatedTitles, ""),
	}
	return translated
}

// translateCategoryTitles batch-translates all category titles that aren't already translated.
func (g *Generator) translateCategoryTitles(
	ctx context.Context,
	items []catalog.FlatItem,
	alreadyTranslated map[string]string,
	sourceLang, sourceLangName, targetLang, targetLangName string,
) (map[string]string, error) {
	// Collect category items whose titles haven't been translated yet
	type categoryEntry struct {
		path  string
		title string
	}
	var toTranslate []categoryEntry
	for _, item := range items {
		if !item.HasChildren {
			continue
		}
		if _, ok := alreadyTranslated[item.Path]; ok {
			continue
		}
		toTranslate = append(toTranslate, categoryEntry{path: item.Path, title: item.Title})
	}

	if len(toTranslate) == 0 {
		return nil, nil
	}

	titles := make([]string, len(toTranslate))
	for i, entry := range toTranslate {
		titles[i] = entry.title
	}

	fmt.Printf("      Translating %d category titles...", len(titles))

	rendered, err := g.Engine.RenderTranslateTitles(prompt.TranslateTitlesPromptData{
		SourceLanguage:     sourceLang,
		SourceLanguageName: sourceLangName,
		TargetLanguage:     targetLang,
		TargetLanguageName: targetLangName,
		Titles:             titles,
	})
	if err != nil {
		fmt.Printf(" Failed (render template)\n")
		return nil, fmt.Errorf("render template: %w", err)
	}

	result, err := g.Runner.RunWithRetry(ctx, claude.RunOptions{
		Prompt:  rendered,
		WorkDir: g.RootDir,
	})
	if err != nil {
		fmt.Printf(" Failed\n")
		return nil, fmt.Errorf("claude call: %w", err)
	}

	g.TotalCost += result.CostUSD

	// Parse JSON array from response
	content := strings.TrimSpace(result.Content)

	// Try to extract JSON array from the response (may be wrapped in markdown code block)
	if idx := strings.Index(content, "["); idx != -1 {
		if end := strings.LastIndex(content, "]"); end != -1 && end > idx {
			content = content[idx : end+1]
		}
	}

	var translated []string
	if err := json.Unmarshal([]byte(content), &translated); err != nil {
		fmt.Printf(" Failed (parse response)\n")
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(translated) != len(toTranslate) {
		fmt.Printf(" Failed (count mismatch: expected %d, got %d)\n", len(toTranslate), len(translated))
		return nil, fmt.Errorf("count mismatch: expected %d, got %d", len(toTranslate), len(translated))
	}

	result2 := make(map[string]string, len(toTranslate))
	for i, entry := range toTranslate {
		result2[entry.path] = translated[i]
	}

	fmt.Printf(" Done ($%.4f)\n", result.CostUSD)
	return result2, nil
}

func translateCatalogItems(items []catalog.CatalogItem, titles map[string]string, parentPath string) []catalog.CatalogItem {
	result := make([]catalog.CatalogItem, len(items))
	for i, item := range items {
		dotPath := item.Path
		if parentPath != "" {
			dotPath = parentPath + "." + item.Path
		}

		result[i] = catalog.CatalogItem{
			Title:    item.Title,
			Path:     item.Path,
			Order:    item.Order,
			Children: translateCatalogItems(item.Children, titles, dotPath),
		}

		// Use translated title if available
		if translatedTitle, ok := titles[dotPath]; ok {
			result[i].Title = translatedTitle
		}
	}
	return result
}
