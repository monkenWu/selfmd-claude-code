package scanner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// FileNode represents a node in the file tree (file or directory).
type FileNode struct {
	Name     string
	Path     string // relative path from project root
	IsDir    bool
	Children []*FileNode
}

// ScanResult holds the result of a project scan.
type ScanResult struct {
	RootDir            string
	Tree               *FileNode
	FileList           []string
	TotalFiles         int
	TotalDirs          int
	ReadmeContent      string
	EntryPointContents map[string]string
}

// BuildTree constructs a FileNode tree from a flat list of relative paths.
func BuildTree(rootName string, paths []string) *FileNode {
	root := &FileNode{
		Name:  rootName,
		Path:  "",
		IsDir: true,
	}

	for _, p := range paths {
		parts := strings.Split(filepath.ToSlash(p), "/")
		current := root
		for i, part := range parts {
			isLast := i == len(parts)-1
			child := findChild(current, part)
			if child == nil {
				child = &FileNode{
					Name:  part,
					Path:  strings.Join(parts[:i+1], "/"),
					IsDir: !isLast,
				}
				current.Children = append(current.Children, child)
			}
			if !isLast {
				child.IsDir = true
			}
			current = child
		}
	}

	sortTree(root)
	return root
}

func findChild(node *FileNode, name string) *FileNode {
	for _, c := range node.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func sortTree(node *FileNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		// directories first, then alphabetical
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, c := range node.Children {
		if c.IsDir {
			sortTree(c)
		}
	}
}

// RenderTree renders the file tree in a TOON-like format for prompts.
func RenderTree(node *FileNode, maxDepth int) string {
	var sb strings.Builder
	sb.WriteString(node.Name + "/\n")
	renderChildren(&sb, node, "", maxDepth, 0)
	return sb.String()
}

func renderChildren(sb *strings.Builder, node *FileNode, prefix string, maxDepth, depth int) {
	if maxDepth > 0 && depth >= maxDepth {
		if len(node.Children) > 0 {
			sb.WriteString(prefix + "└── ...\n")
		}
		return
	}

	children := node.Children
	// truncate if too many children
	truncated := false
	if len(children) > 30 {
		children = children[:30]
		truncated = true
	}

	for i, child := range children {
		isLast := i == len(children)-1 && !truncated
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		if child.IsDir {
			sb.WriteString(fmt.Sprintf("%s%s%s/\n", prefix, connector, child.Name))
			renderChildren(sb, child, childPrefix, maxDepth, depth+1)
		} else {
			sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, connector, child.Name))
		}
	}

	if truncated {
		sb.WriteString(fmt.Sprintf("%s└── ... (%d more items)\n", prefix, len(node.Children)-30))
	}
}
