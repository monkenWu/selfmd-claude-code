package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monkenwu/selfmd/internal/catalog"
	"github.com/monkenwu/selfmd/internal/claude"
	"github.com/monkenwu/selfmd/internal/git"
	"github.com/monkenwu/selfmd/internal/output"
	"github.com/monkenwu/selfmd/internal/prompt"
	"github.com/monkenwu/selfmd/internal/scanner"
)

// UpdateMatchedResult represents a page that Claude determined needs regeneration.
type UpdateMatchedResult struct {
	CatalogPath string `json:"catalogPath"`
	Title       string `json:"title"`
	Reason      string `json:"reason"`
}

// UpdateUnmatchedResult represents a new page that Claude determined should be created.
type UpdateUnmatchedResult struct {
	CatalogPath string `json:"catalogPath"`
	Title       string `json:"title"`
	Reason      string `json:"reason"`
}

// Update performs an incremental documentation update based on git changes.
func (g *Generator) Update(ctx context.Context, scan *scanner.ScanResult, previousCommit, currentCommit, changedFiles string) error {
	// Read existing catalog
	existingCatalogJSON, err := g.Writer.ReadCatalogJSON()
	if err != nil {
		return fmt.Errorf("failed to read existing catalog (please run selfmd generate first): %w", err)
	}

	cat, err := catalog.Parse(existingCatalogJSON)
	if err != nil {
		return fmt.Errorf("failed to parse existing catalog: %w", err)
	}

	// Step 1: Parse changed files
	files := git.ParseChangedFiles(changedFiles)
	if len(files) == 0 {
		fmt.Println("No file changes detected.")
		return nil
	}

	// Step 2: Match changed files to existing doc pages
	fmt.Println("[1/4] Searching for affected documentation pages...")
	matched, unmatched := g.matchChangedFilesToDocs(files, cat)

	fmt.Printf("      %d changed files matched to existing docs, %d unmatched\n", len(matched), len(unmatched))

	// Build shared resources for content generation
	catalogTable := cat.BuildLinkTable()
	linkFixer := output.NewLinkFixer(cat)

	var pagesToRegenerate []catalog.FlatItem

	// Step 3: For matched files, ask Claude which pages need regeneration
	if len(matched) > 0 {
		fmt.Print("[2/4] Calling Claude to determine pages needing update...")
		regenPages, err := g.determineMatchedUpdates(ctx, matched, cat)
		if err != nil {
			fmt.Println(" Failed")
			return fmt.Errorf("failed to determine pages to update: %w", err)
		}
		fmt.Printf(" Done (%d pages need update)\n", len(regenPages))
		pagesToRegenerate = append(pagesToRegenerate, regenPages...)
	} else {
		fmt.Println("[2/4] No matched doc pages, skipping")
	}

	// Step 4: For unmatched files, ask Claude if new pages are needed
	var newPages []catalog.FlatItem
	if len(unmatched) > 0 {
		fmt.Print("[3/4] Calling Claude to determine if new pages are needed...")
		newPageResults, err := g.determineUnmatchedPages(ctx, unmatched, cat)
		if err != nil {
			fmt.Println(" Failed")
			return fmt.Errorf("failed to determine new pages: %w", err)
		}
		fmt.Printf(" Done (%d new pages)\n", len(newPageResults))

		for _, np := range newPageResults {
			item := catalog.FlatItem{
				Title:   np.Title,
				Path:    np.CatalogPath,
				DirPath: catalogPathToDir(np.CatalogPath),
			}
			newPages = append(newPages, item)
			// Add to catalog; handle leaf-to-parent promotion
			promoted := addItemToCatalog(cat, np.CatalogPath, np.Title)
			if promoted != nil {
				// A leaf node was promoted to a parent.
				// Move the original content to the new "overview" child.
				origItem := catalog.FlatItem{
					Path:    promoted.OriginalPath,
					DirPath: catalogPathToDir(promoted.OriginalPath),
				}
				overviewItem := catalog.FlatItem{
					Title:   promoted.OriginalTitle,
					Path:    promoted.OverviewPath,
					DirPath: catalogPathToDir(promoted.OverviewPath),
				}
				if content, err := g.Writer.ReadPage(origItem); err == nil && content != "" {
					if err := g.Writer.WritePage(overviewItem, content); err != nil {
						g.Logger.Warn("failed to move page to overview", "from", promoted.OriginalPath, "error", err)
					} else {
						fmt.Printf("      → Page promoted: %s original content moved to %s\n", promoted.OriginalPath, promoted.OverviewPath)
					}
				}
			}
		}

		if len(newPages) > 0 {
			// Re-build catalog table and link fixer with new pages
			catalogTable = cat.BuildLinkTable()
			linkFixer = output.NewLinkFixer(cat)
			// Save updated catalog
			if err := g.Writer.WriteCatalogJSON(cat); err != nil {
				g.Logger.Warn("failed to save updated catalog", "error", err)
			}
		}
	} else {
		fmt.Println("[3/4] All changed files have matching docs, skipping")
	}

	// Step 5: Regenerate pages using the content generation flow
	allPages := append(pagesToRegenerate, newPages...)
	if len(allPages) == 0 {
		fmt.Println("[4/4] No documentation pages need updating or creating.")
	} else {
		fmt.Printf("[4/4] Regenerating %d pages...\n", len(allPages))
		for i, item := range allPages {
			fmt.Printf("      [%d/%d] %s（%s）...", i+1, len(allPages), item.Title, item.Path)
			// Read existing content to pass as context for regeneration
			existing, _ := g.Writer.ReadPage(item)
			err := g.generateSinglePage(ctx, scan, item, catalogTable, linkFixer, existing)
			if err != nil {
				fmt.Printf(" Failed: %v\n", err)
				g.Logger.Warn("page regeneration failed", "title", item.Title, "path", item.Path, "error", err)
				g.writePlaceholder(item, err)
			}
		}
	}

	// Regenerate index and sidebar if new pages were added
	if len(newPages) > 0 {
		fmt.Println("Updating navigation and index...")
		if err := g.GenerateIndex(ctx, cat); err != nil {
			g.Logger.Warn("failed to update index", "error", err)
		}
	}

	// Save current commit for next incremental update
	if err := g.Writer.SaveLastCommit(currentCommit); err != nil {
		g.Logger.Warn("failed to save commit record", "error", err)
	}

	fmt.Printf("\nUpdate complete! Total cost: $%.4f USD\n", g.TotalCost)
	return nil
}

// matchResult holds the mapping between changed files and the doc pages that reference them.
type matchResult struct {
	// changedFile is the source file path that changed
	changedFile string
	// pages are the doc pages that reference this file
	pages []catalog.FlatItem
}

// matchChangedFilesToDocs searches existing doc pages for references to changed file paths.
func (g *Generator) matchChangedFilesToDocs(files []git.ChangedFile, cat *catalog.Catalog) (matched []matchResult, unmatched []string) {
	items := cat.Flatten()

	// Pre-read all page contents
	pageContents := make(map[string]string) // key: item.Path, value: page content
	for _, item := range items {
		content, err := g.Writer.ReadPage(item)
		if err != nil {
			continue
		}
		pageContents[item.Path] = content
	}

	// For each changed file, find which pages reference it
	for _, f := range files {
		var matchedPages []catalog.FlatItem
		for _, item := range items {
			content, ok := pageContents[item.Path]
			if !ok {
				continue
			}
			if strings.Contains(content, f.Path) {
				matchedPages = append(matchedPages, item)
			}
		}

		if len(matchedPages) > 0 {
			matched = append(matched, matchResult{
				changedFile: f.Path,
				pages:       matchedPages,
			})
		} else {
			unmatched = append(unmatched, f.Path)
		}
	}

	return matched, unmatched
}

// determineMatchedUpdates asks Claude which existing pages need regeneration.
func (g *Generator) determineMatchedUpdates(ctx context.Context, matched []matchResult, cat *catalog.Catalog) ([]catalog.FlatItem, error) {
	// Build changed files list
	var changedFilesList strings.Builder
	for _, m := range matched {
		changedFilesList.WriteString(fmt.Sprintf("- `%s`\n", m.changedFile))
	}

	// Build affected pages info (deduplicated)
	seenPages := make(map[string]bool)
	var affectedPagesInfo strings.Builder
	for _, m := range matched {
		for _, page := range m.pages {
			if seenPages[page.Path] {
				continue
			}
			seenPages[page.Path] = true

			summary := ""
			content, err := g.Writer.ReadPage(page)
			if err == nil && len(content) > 500 {
				summary = content[:500] + "..."
			} else if err == nil {
				summary = content
			}

			affectedPagesInfo.WriteString(fmt.Sprintf("### %s (catalogPath: %s)\n", page.Title, page.Path))
			affectedPagesInfo.WriteString("Referenced changed files: ")
			// List which changed files this page references
			for _, m2 := range matched {
				for _, p := range m2.pages {
					if p.Path == page.Path {
						affectedPagesInfo.WriteString(fmt.Sprintf("`%s` ", m2.changedFile))
					}
				}
			}
			affectedPagesInfo.WriteString("\n")
			affectedPagesInfo.WriteString(fmt.Sprintf("Summary:\n%s\n\n", summary))
		}
	}

	data := prompt.UpdateMatchedPromptData{
		RepositoryName: g.Config.Project.Name,
		Language:       g.Config.Output.Language,
		ChangedFiles:   changedFilesList.String(),
		AffectedPages:  affectedPagesInfo.String(),
	}

	rendered, err := g.Engine.RenderUpdateMatched(data)
	if err != nil {
		return nil, err
	}

	result, err := g.Runner.RunWithRetry(ctx, claude.RunOptions{
		Prompt:  rendered,
		WorkDir: g.RootDir,
	})
	if err != nil {
		return nil, err
	}
	g.TotalCost += result.CostUSD

	jsonStr, err := claude.ExtractJSONBlock(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract analysis result: %w", err)
	}

	var results []UpdateMatchedResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, fmt.Errorf("failed to parse analysis result: %w", err)
	}

	// Convert results to FlatItems, looking up from catalog
	items := cat.Flatten()
	itemMap := make(map[string]catalog.FlatItem)
	for _, item := range items {
		itemMap[item.Path] = item
	}

	var pagesToRegen []catalog.FlatItem
	for _, r := range results {
		if item, ok := itemMap[r.CatalogPath]; ok {
			fmt.Printf("      → %s：%s\n", item.Title, r.Reason)
			pagesToRegen = append(pagesToRegen, item)
		} else {
			g.Logger.Warn("catalogPath returned by Claude does not exist", "path", r.CatalogPath)
		}
	}

	return pagesToRegen, nil
}

// determineUnmatchedPages asks Claude whether new pages should be created for unmatched files.
func (g *Generator) determineUnmatchedPages(ctx context.Context, unmatchedFiles []string, cat *catalog.Catalog) ([]UpdateUnmatchedResult, error) {
	var fileList strings.Builder
	for _, f := range unmatchedFiles {
		fileList.WriteString(fmt.Sprintf("- `%s`\n", f))
	}

	existingCatalog, err := cat.ToJSON()
	if err != nil {
		return nil, err
	}

	data := prompt.UpdateUnmatchedPromptData{
		RepositoryName:  g.Config.Project.Name,
		Language:        g.Config.Output.Language,
		UnmatchedFiles:  fileList.String(),
		ExistingCatalog: existingCatalog,
		CatalogTable:    cat.BuildLinkTable(),
	}

	rendered, err := g.Engine.RenderUpdateUnmatched(data)
	if err != nil {
		return nil, err
	}

	result, err := g.Runner.RunWithRetry(ctx, claude.RunOptions{
		Prompt:  rendered,
		WorkDir: g.RootDir,
	})
	if err != nil {
		return nil, err
	}
	g.TotalCost += result.CostUSD

	jsonStr, err := claude.ExtractJSONBlock(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to extract analysis result: %w", err)
	}

	var results []UpdateUnmatchedResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, fmt.Errorf("failed to parse analysis result: %w", err)
	}

	for _, r := range results {
		fmt.Printf("      → New: %s (%s)\n", r.Title, r.Reason)
	}

	return results, nil
}

// promotedLeaf records when a leaf node was promoted to a parent by adding an "overview" child.
type promotedLeaf struct {
	// OriginalPath is the dot-notation path of the original leaf (e.g. "core-modules.mcp-integration")
	OriginalPath string
	// OverviewPath is the dot-notation path of the new overview child (e.g. "core-modules.mcp-integration.overview")
	OverviewPath string
	// OriginalTitle is the title of the original leaf
	OriginalTitle string
}

// addItemToCatalog adds a new item to the catalog based on its dot-notation path.
// Supports arbitrary nesting depth, e.g. "core-modules.mcp-integration.mcp-risk-card-widget".
// Returns a promotedLeaf if an existing leaf node was promoted to a parent.
func addItemToCatalog(cat *catalog.Catalog, catalogPath, title string) *promotedLeaf {
	parts := strings.Split(catalogPath, ".")
	var promoted *promotedLeaf
	addItemToChildren(&cat.Items, parts, title, "", &promoted)
	return promoted
}

// addItemToChildren recursively walks the catalog tree and inserts the item at the correct depth.
// parentDotPath tracks the full dot-notation path of the current level's parent.
func addItemToChildren(children *[]catalog.CatalogItem, pathParts []string, title string, parentDotPath string, promoted **promotedLeaf) {
	if len(pathParts) == 1 {
		// Leaf node — add here
		*children = append(*children, catalog.CatalogItem{
			Title: title,
			Path:  pathParts[0],
			Order: len(*children) + 1,
		})
		return
	}

	// Find existing parent matching the first path segment
	parentSlug := pathParts[0]
	currentDotPath := parentSlug
	if parentDotPath != "" {
		currentDotPath = parentDotPath + "." + parentSlug
	}

	for i, item := range *children {
		if item.Path == parentSlug {
			// Found existing item — check if it's a leaf being promoted to parent
			if len(item.Children) == 0 {
				// This is a leaf node that needs to become a parent.
				// Add an "overview" child to preserve the original content.
				(*children)[i].Children = append((*children)[i].Children, catalog.CatalogItem{
					Title: item.Title,
					Path:  "overview",
					Order: 0,
				})
				*promoted = &promotedLeaf{
					OriginalPath:  currentDotPath,
					OverviewPath:  currentDotPath + ".overview",
					OriginalTitle: item.Title,
				}
			}
			// Recurse into children
			addItemToChildren(&(*children)[i].Children, pathParts[1:], title, currentDotPath, promoted)
			return
		}
	}

	// Parent not found — create it, then recurse
	newParent := catalog.CatalogItem{
		Title: parentSlug,
		Path:  parentSlug,
		Order: len(*children) + 1,
	}
	*children = append(*children, newParent)
	addItemToChildren(&(*children)[len(*children)-1].Children, pathParts[1:], title, currentDotPath, promoted)
}

func catalogPathToDir(path string) string {
	result := ""
	for _, c := range path {
		if c == '.' {
			result += "/"
		} else {
			result += string(c)
		}
	}
	return result
}
