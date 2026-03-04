package generator

import (
	"context"

	"github.com/monkenwu/selfmd/internal/catalog"
	"github.com/monkenwu/selfmd/internal/output"
)

// GenerateIndex generates the index.md, _sidebar.md, and category index pages.
func (g *Generator) GenerateIndex(_ context.Context, cat *catalog.Catalog) error {
	lang := g.Config.Output.Language

	// Generate main index.md
	indexContent := output.GenerateIndex(
		g.Config.Project.Name,
		g.Config.Project.Description,
		cat,
		lang,
	)
	if err := g.Writer.WriteFile("index.md", indexContent); err != nil {
		return err
	}

	// Generate _sidebar.md
	sidebarContent := output.GenerateSidebar(g.Config.Project.Name, cat, lang)
	if err := g.Writer.WriteFile("_sidebar.md", sidebarContent); err != nil {
		return err
	}

	// Generate category index pages for items with children
	items := cat.Flatten()
	for _, item := range items {
		if !item.HasChildren {
			continue
		}

		// find direct children
		var children []catalog.FlatItem
		for _, child := range items {
			if child.ParentPath == item.Path && child.Path != item.Path {
				children = append(children, child)
			}
		}

		if len(children) > 0 {
			categoryContent := output.GenerateCategoryIndex(item, children, lang)
			if err := g.Writer.WritePage(item, categoryContent); err != nil {
				g.Logger.Warn("failed to write category index", "path", item.Path, "error", err)
			}
		}
	}

	return nil
}
