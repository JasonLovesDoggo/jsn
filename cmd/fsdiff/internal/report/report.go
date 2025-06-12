package report

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/diff"
	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/snapshot"
)

//go:generate go tool templ generate

// GenerateHTML creates a detailed HTML report of the differences using templ
func GenerateHTML(result *diff.Result, filename string) error {
	// Build file trees
	addedTree := buildFileTree(result.Added, nil)
	modifiedTree := buildModifiedTree(result.Modified)
	deletedTree := buildFileTree(result.Deleted, nil)

	// Prepare data for template
	data := &HTMLReportData{
		Result:            result,
		GeneratedAt:       time.Now(),
		CriticalChanges:   result.GetCriticalChanges(),
		ChangesByType:     result.GetChangesByType(),
		TopLargestAdded:   getTopLargestAddedFiles(result.Added, 10),
		TopLargestDeleted: getTopLargestDeletedFiles(result.Deleted, 10),
		AddedTreeHTML:     renderTreeToHTML(addedTree, "added", "text-green-400"),
		ModifiedTreeHTML:  renderModifiedTreeToHTML(modifiedTree, "modified", "text-yellow-400"),
		DeletedTreeHTML:   renderTreeToHTML(deletedTree, "deleted", "text-red-400"),
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create report file: %v", err)
	}
	defer file.Close()

	// Render template
	ctx := context.Background()
	if err := reportTemplate(data).Render(ctx, file); err != nil {
		return fmt.Errorf("failed to render template: %v", err)
	}

	return nil
}

// HTMLReportData contains all data needed for the HTML report
type HTMLReportData struct {
	Result            *diff.Result
	GeneratedAt       time.Time
	CriticalChanges   []diff.CriticalChange
	ChangesByType     map[diff.ChangeType][]string
	TopLargestAdded   []FileSize
	TopLargestDeleted []FileSize
	AddedTreeHTML     string
	ModifiedTreeHTML  string
	DeletedTreeHTML   string
}

// TreeNode represents a node in the file tree
type TreeNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children map[string]*TreeNode
	File     interface{} // *snapshot.FileRecord or *diff.ChangeDetail
	Count    int         // Number of files in this directory
}

// FileSize represents a file and its size for sorting
type FileSize struct {
	Path string
	Size int64
}

// getTopLargestAddedFiles returns the largest added files
func getTopLargestAddedFiles(files map[string]*snapshot.FileRecord, limit int) []FileSize {
	var fileSizes []FileSize
	for path, record := range files {
		fileSizes = append(fileSizes, FileSize{Path: path, Size: record.Size})
	}
	sort.Slice(fileSizes, func(i, j int) bool {
		return fileSizes[i].Size > fileSizes[j].Size
	})
	if len(fileSizes) > limit {
		fileSizes = fileSizes[:limit]
	}
	return fileSizes
}

// getTopLargestDeletedFiles returns the largest deleted files
func getTopLargestDeletedFiles(files map[string]*snapshot.FileRecord, limit int) []FileSize {
	var fileSizes []FileSize
	for path, record := range files {
		fileSizes = append(fileSizes, FileSize{Path: path, Size: record.Size})
	}
	sort.Slice(fileSizes, func(i, j int) bool {
		return fileSizes[i].Size > fileSizes[j].Size
	})
	if len(fileSizes) > limit {
		fileSizes = fileSizes[:limit]
	}
	return fileSizes
}

// Helper functions for template
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func getChangeIcon(changeType diff.ChangeType) string {
	switch changeType {
	case diff.ChangeAdded:
		return "➕"
	case diff.ChangeModified:
		return "🔄"
	case diff.ChangeDeleted:
		return "❌"
	default:
		return "❓"
	}
}

func getSeverityColorClass(severity int) string {
	if severity >= 8 {
		return "text-red-600 font-bold"
	} else if severity >= 6 {
		return "text-orange-600 font-bold"
	} else if severity >= 4 {
		return "text-yellow-600 font-semibold"
	}
	return "text-green-600"
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// buildFileTree creates a tree structure from a map of file records
func buildFileTree(files map[string]*snapshot.FileRecord, changes map[string]*diff.ChangeDetail) map[string]*TreeNode {
	tree := make(map[string]*TreeNode)

	for path, record := range files {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
			continue
		}

		current := tree
		currentPath := ""

		// Build directory structure
		for i, part := range parts {
			if part == "" {
				continue
			}

			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "/" + part
			}

			if _, exists := current[part]; !exists {
				isDir := i < len(parts)-1
				var file interface{}
				if !isDir {
					file = record
				}

				current[part] = &TreeNode{
					Name:     part,
					Path:     currentPath,
					IsDir:    isDir,
					Children: make(map[string]*TreeNode),
					File:     file,
					Count:    0,
				}
			}

			// Update count for directories
			if current[part].IsDir {
				current[part].Count++
			}

			current = current[part].Children
		}
	}

	return tree
}

// buildModifiedTree creates a tree structure from modified files
func buildModifiedTree(changes map[string]*diff.ChangeDetail) map[string]*TreeNode {
	tree := make(map[string]*TreeNode)

	for path, change := range changes {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
			continue
		}

		current := tree
		currentPath := ""

		// Build directory structure
		for i, part := range parts {
			if part == "" {
				continue
			}

			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "/" + part
			}

			if _, exists := current[part]; !exists {
				isDir := i < len(parts)-1
				var file interface{}
				if !isDir {
					file = change
				}

				current[part] = &TreeNode{
					Name:     part,
					Path:     currentPath,
					IsDir:    isDir,
					Children: make(map[string]*TreeNode),
					File:     file,
					Count:    0,
				}
			}

			// Update count for directories
			if current[part].IsDir {
				current[part].Count++
			}

			current = current[part].Children
		}
	}

	return tree
}

// escapeID creates a safe HTML ID from a path
func escapeID(path string) string {
	// Replace special characters with safe alternatives
	safe := strings.ReplaceAll(path, "/", "-")
	safe = strings.ReplaceAll(safe, ".", "_")
	safe = strings.ReplaceAll(safe, " ", "-")
	return safe
}

// renderTreeToHTML generates HTML for the file tree
func renderTreeToHTML(tree map[string]*TreeNode, prefix, colorClass string) string {
	var html strings.Builder

	// Sort keys for consistent output
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		node := tree[key]
		nodeID := prefix + "-" + escapeID(node.Path)

		html.WriteString(`<div class="border-l border-gray-600 pl-4">`)

		if node.IsDir {
			html.WriteString(fmt.Sprintf(`
				<div class="flex items-center py-1 hover:bg-gray-700/30 rounded transition-colors">
					<button onclick="toggleTree('%s-tree')" class="flex items-center text-sm font-medium text-gray-300 hover:text-white transition-colors">
						<span id="%s-tree-icon" class="mr-2 text-base">📁</span>
						<span class="mr-2">%s</span>
						<span class="text-xs bg-gray-600 px-2 py-0.5 rounded-full">%d</span>
					</button>
				</div>
				<div id="%s-tree" class="hidden ml-4 mt-1">
					%s
				</div>`,
				nodeID, nodeID, node.Name, node.Count, nodeID, renderTreeToHTML(node.Children, prefix, colorClass)))
		} else {
			if record, ok := node.File.(*snapshot.FileRecord); ok {
				html.WriteString(fmt.Sprintf(`
					<div class="flex items-center justify-between py-1 px-2 hover:bg-gray-700/30 rounded transition-colors group">
						<div class="flex items-center">
							<span class="mr-2 text-sm">📄</span>
							<code class="bg-gray-900 px-2 py-1 rounded text-xs font-mono group-hover:bg-gray-800 transition-colors %s">%s</code>
						</div>
						<div class="flex items-center gap-2 text-xs text-gray-400">
							<span class="text-blue-400 font-mono">%s</span>
							<span class="font-mono">%s</span>
						</div>
					</div>`,
					colorClass, node.Name, formatBytes(record.Size), formatTime(record.ModTime)))
			}
		}

		html.WriteString(`</div>`)
	}

	return html.String()
}

// renderModifiedTreeToHTML generates HTML for the modified file tree
func renderModifiedTreeToHTML(tree map[string]*TreeNode, prefix, colorClass string) string {
	var html strings.Builder

	// Sort keys for consistent output
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		node := tree[key]
		nodeID := prefix + "-" + escapeID(node.Path)

		html.WriteString(`<div class="border-l border-gray-600 pl-4">`)

		if node.IsDir {
			html.WriteString(fmt.Sprintf(`
				<div class="flex items-center py-1 hover:bg-gray-700/30 rounded transition-colors">
					<button onclick="toggleTree('%s-tree')" class="flex items-center text-sm font-medium text-gray-300 hover:text-white transition-colors">
						<span id="%s-tree-icon" class="mr-2 text-base">📁</span>
						<span class="mr-2">%s</span>
						<span class="text-xs bg-gray-600 px-2 py-0.5 rounded-full">%d</span>
					</button>
				</div>
				<div id="%s-tree" class="hidden ml-4 mt-1">
					%s
				</div>`,
				nodeID, nodeID, node.Name, node.Count, nodeID, renderModifiedTreeToHTML(node.Children, prefix, colorClass)))
		} else {
			if change, ok := node.File.(*diff.ChangeDetail); ok {
				var changesHTML strings.Builder
				for i, ch := range change.Changes {
					if i > 0 {
						changesHTML.WriteString(" ")
					}
					changesHTML.WriteString(fmt.Sprintf(`<span class="bg-orange-500/20 text-orange-300 px-1 rounded text-xs">%s</span>`, ch))
				}

				html.WriteString(fmt.Sprintf(`
					<div class="flex items-center justify-between py-1 px-2 hover:bg-gray-700/30 rounded transition-colors group">
						<div class="flex items-center">
							<span class="mr-2 text-sm">📄</span>
							<code class="bg-gray-900 px-2 py-1 rounded text-xs font-mono group-hover:bg-gray-800 transition-colors %s">%s</code>
						</div>
						<div class="flex items-center gap-2 text-xs text-gray-400">
							<span class="text-blue-400 font-mono">%s</span>
							<div class="flex gap-1">%s</div>
						</div>
					</div>`,
					colorClass, node.Name, formatBytes(change.NewRecord.Size), changesHTML.String()))
			}
		}

		html.WriteString(`</div>`)
	}

	return html.String()
}
