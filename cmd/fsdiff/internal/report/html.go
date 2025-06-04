package report

import (
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/diff"
	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/snapshot"
)

// GenerateHTML creates a detailed HTML report of the differences
func GenerateHTML(result *diff.Result, filename string) error {
	// Prepare data for template
	data := &HTMLReportData{
		Result:            result,
		GeneratedAt:       time.Now(),
		CriticalChanges:   result.GetCriticalChanges(),
		ChangesByType:     result.GetChangesByType(),
		TopLargestAdded:   getTopLargestAddedFiles(result.Added, 10),
		TopLargestDeleted: getTopLargestDeletedFiles(result.Deleted, 10),
	}

	// Parse template
	tmpl, err := template.New("report").Funcs(templateFuncs()).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create report file: %v", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
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

	// Sort by size descending
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

	// Sort by size descending
	sort.Slice(fileSizes, func(i, j int) bool {
		return fileSizes[i].Size > fileSizes[j].Size
	})

	if len(fileSizes) > limit {
		fileSizes = fileSizes[:limit]
	}

	return fileSizes
}

// templateFuncs returns template helper functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatBytes":      formatBytes,
		"formatTime":       formatTime,
		"getIcon":          getChangeIcon,
		"getSeverityColor": getSeverityColor,
		"truncate":         truncateString,
		"join":             strings.Join,
		"hasPrefix":        strings.HasPrefix,
	}
}

// formatBytes formats byte size for display
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

// formatTime formats time for display
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// getChangeIcon returns an icon for the change type
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

// getSeverityColor returns a color class for severity
func getSeverityColor(severity int) string {
	if severity >= 8 {
		return "severity-critical"
	} else if severity >= 6 {
		return "severity-high"
	} else if severity >= 4 {
		return "severity-medium"
	}
	return "severity-low"
}

// truncateString truncates a string to the specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// HTML template for the report (simplified version)
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Filesystem Diff Report</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif; line-height: 1.6; margin: 0; padding: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); overflow: hidden; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 2rem; text-align: center; }
        .header h1 { margin: 0; font-size: 2.5rem; font-weight: 300; }
        .content { padding: 2rem; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
        .stat-card { background: #f8f9fa; border-radius: 8px; padding: 1.5rem; text-align: center; border-left: 4px solid #007bff; }
        .stat-card.added { border-left-color: #28a745; }
        .stat-card.modified { border-left-color: #ffc107; }
        .stat-card.deleted { border-left-color: #dc3545; }
        .stat-number { font-size: 2rem; font-weight: bold; color: #333; }
        .stat-label { color: #666; margin-top: 0.5rem; }
        .changes-table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
        .changes-table th, .changes-table td { text-align: left; padding: 0.75rem; border-bottom: 1px solid #eee; }
        .changes-table th { background: #f8f9fa; font-weight: 600; color: #495057; }
        .path-cell { font-family: 'Monaco', 'Menlo', 'Consolas', monospace; font-size: 0.9rem; }
        .severity-critical { color: #c53030; font-weight: bold; }
        .severity-high { color: #dd6b20; font-weight: bold; }
        .severity-medium { color: #d69e2e; }
        .severity-low { color: #38a169; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔍 Filesystem Diff Report</h1>
            <p>Generated on {{formatTime .GeneratedAt}}</p>
        </div>

        <div class="content">
            <div class="summary">
                <div class="stat-card added">
                    <div class="stat-number">{{.Result.Summary.AddedCount}}</div>
                    <div class="stat-label">Files Added</div>
                </div>
                <div class="stat-card modified">
                    <div class="stat-number">{{.Result.Summary.ModifiedCount}}</div>
                    <div class="stat-label">Files Modified</div>
                </div>
                <div class="stat-card deleted">
                    <div class="stat-number">{{.Result.Summary.DeletedCount}}</div>
                    <div class="stat-label">Files Deleted</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">{{.Result.Summary.TotalChanges}}</div>
                    <div class="stat-label">Total Changes</div>
                </div>
            </div>

            <h2>System Information</h2>
            <p><strong>Baseline:</strong> {{.Result.Baseline.SystemInfo.Hostname}} ({{.Result.Baseline.SystemInfo.Distro}}) - {{formatTime .Result.Baseline.SystemInfo.Timestamp}}</p>
            <p><strong>Current:</strong> {{.Result.Current.SystemInfo.Hostname}} ({{.Result.Current.SystemInfo.Distro}}) - {{formatTime .Result.Current.SystemInfo.Timestamp}}</p>

            {{if .CriticalChanges}}
            <h2>🚨 Critical Changes</h2>
            <table class="changes-table">
                <thead>
                    <tr><th>Severity</th><th>Type</th><th>Path</th><th>Reason</th></tr>
                </thead>
                <tbody>
                    {{range .CriticalChanges}}
                    <tr>
                        <td><span class="{{getSeverityColor .Severity}}">{{.Severity}}/10</span></td>
                        <td>{{getIcon .Type}} {{.Type}}</td>
                        <td class="path-cell">{{.Path}}</td>
                        <td>{{.Reason}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{end}}

            <h2>📁 Added Files ({{.Result.Summary.AddedCount}})</h2>
            {{if .Result.Added}}
            <table class="changes-table">
                <thead>
                    <tr><th>Path</th><th>Size</th><th>Modified</th></tr>
                </thead>
                <tbody>
                    {{range $path, $record := .Result.Added}}
                    <tr>
                        <td class="path-cell">{{$path}}</td>
                        <td>{{formatBytes $record.Size}}</td>
                        <td>{{formatTime $record.ModTime}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <p>No files were added.</p>
            {{end}}

            <h2>🔄 Modified Files ({{.Result.Summary.ModifiedCount}})</h2>
            {{if .Result.Modified}}
            <table class="changes-table">
                <thead>
                    <tr><th>Path</th><th>Size</th><th>Changes</th></tr>
                </thead>
                <tbody>
                    {{range $path, $change := .Result.Modified}}
                    <tr>
                        <td class="path-cell">{{$path}}</td>
                        <td>{{formatBytes $change.NewRecord.Size}}</td>
                        <td>{{join $change.Changes ", "}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <p>No files were modified.</p>
            {{end}}

            <h2>❌ Deleted Files ({{.Result.Summary.DeletedCount}})</h2>
            {{if .Result.Deleted}}
            <table class="changes-table">
                <thead>
                    <tr><th>Path</th><th>Size</th><th>Modified</th></tr>
                </thead>
                <tbody>
                    {{range $path, $record := .Result.Deleted}}
                    <tr>
                        <td class="path-cell">{{$path}}</td>
                        <td>{{formatBytes $record.Size}}</td>
                        <td>{{formatTime $record.ModTime}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <p>No files were deleted.</p>
            {{end}}
        </div>
    </div>
</body>
</html>`
