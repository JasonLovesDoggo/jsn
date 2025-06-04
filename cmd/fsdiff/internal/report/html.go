package report

import (
	"fmt"
	"html/template"
	"os"
	"pkg.jsn.cam/jsn/cmd/fsdiff/pkg/data"
	"sort"
	"strings"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/diff"
)

// GenerateHTML creates a detailed HTML report of the differences
func GenerateHTML(result *diff.Result, filename string) error {
	// Prepare data for template
	data := &HTMLReportData{
		Result:            result,
		GeneratedAt:       time.Now(),
		CriticalChanges:   result.GetCriticalChanges(),
		ChangesByType:     result.GetChangesByType(),
		TopLargestAdded:   getTopLargestFiles(result.Added, 10),
		TopLargestDeleted: getTopLargestFiles(result.Deleted, 10),
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

// getTopLargestFiles returns the largest files from a map
func getTopLargestFiles(files map[string]*data.FileRecord, limit int) []FileSize {
	var fileSizes []FileSize

	for path, f := range files {
		fileSizes = append(fileSizes, FileSize{Path: path, Size: f.Size})
	}

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

// HTML template for the report
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Filesystem Diff Report</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
            line-height: 1.6;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 2rem;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 2.5rem;
            font-weight: 300;
        }
        .header p {
            margin: 0.5rem 0 0 0;
            opacity: 0.9;
        }
        .content {
            padding: 2rem;
        }
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: #f8f9fa;
            border-radius: 8px;
            padding: 1.5rem;
            text-align: center;
            border-left: 4px solid #007bff;
        }
        .stat-card.added { border-left-color: #28a745; }
        .stat-card.modified { border-left-color: #ffc107; }
        .stat-card.deleted { border-left-color: #dc3545; }
        .stat-number {
            font-size: 2rem;
            font-weight: bold;
            color: #333;
        }
        .stat-label {
            color: #666;
            margin-top: 0.5rem;
        }
        .section {
            margin-bottom: 2rem;
        }
        .section h2 {
            color: #333;
            border-bottom: 2px solid #eee;
            padding-bottom: 0.5rem;
            margin-bottom: 1rem;
        }
        .critical-changes {
            background: #fff5f5;
            border: 1px solid #fed7d7;
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 2rem;
        }
        .critical-changes h2 {
            color: #c53030;
            margin-top: 0;
        }
        .severity-critical { color: #c53030; font-weight: bold; }
        .severity-high { color: #dd6b20; font-weight: bold; }
        .severity-medium { color: #d69e2e; }
        .severity-low { color: #38a169; }
        .changes-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        .changes-table th,
        .changes-table td {
            text-align: left;
            padding: 0.75rem;
            border-bottom: 1px solid #eee;
        }
        .changes-table th {
            background: #f8f9fa;
            font-weight: 600;
            color: #495057;
        }
        .changes-table tr:hover {
            background: #f8f9fa;
        }
        .path-cell {
            font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
            font-size: 0.9rem;
            max-width: 400px;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        .change-type {
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.8rem;
            font-weight: bold;
            text-transform: uppercase;
        }
        .change-type.added {
            background: #d4edda;
            color: #155724;
        }
        .change-type.modified {
            background: #fff3cd;
            color: #856404;
        }
        .change-type.deleted {
            background: #f8d7da;
            color: #721c24;
        }
        .system-info {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 2rem;
            background: #f8f9fa;
            padding: 1.5rem;
            border-radius: 8px;
            margin-bottom: 2rem;
        }
        .system-info h3 {
            margin-top: 0;
            color: #495057;
        }
        .info-item {
            display: flex;
            justify-content: space-between;
            padding: 0.25rem 0;
            border-bottom: 1px solid #dee2e6;
        }
        .info-label {
            font-weight: 600;
            color: #495057;
        }
        .info-value {
            color: #6c757d;
            font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
        }
        .tabs {
            display: flex;
            border-bottom: 2px solid #eee;
            margin-bottom: 1rem;
        }
        .tab {
            padding: 0.75rem 1.5rem;
            background: none;
            border: none;
            cursor: pointer;
            font-weight: 600;
            color: #666;
            transition: all 0.3s ease;
        }
        .tab.active {
            color: #007bff;
            border-bottom: 2px solid #007bff;
        }
        .tab-content {
            display: none;
        }
        .tab-content.active {
            display: block;
        }
        .footer {
            background: #f8f9fa;
            padding: 1rem 2rem;
            text-align: center;
            color: #666;
            font-size: 0.9rem;
        }
        @media (max-width: 768px) {
            .container {
                margin: 0;
                border-radius: 0;
            }
            .summary {
                grid-template-columns: 1fr;
            }
            .system-info {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔍 Filesystem Diff Report</h1>
            <p>Generated on {{formatTime .GeneratedAt}}</p>
        </div>

        <div class="content">
            <!-- Summary Section -->
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

            <!-- System Information -->
            <div class="system-info">
                <div>
                    <h3>📊 Baseline System</h3>
                    <div class="info-item">
                        <span class="info-label">Hostname:</span>
                        <span class="info-value">{{.Result.Baseline.SystemInfo.Hostname}}</span>
                    </div>
                    <div class="info-item">
                        <span class="info-label">OS:</span>
                        <span class="info-value">{{.Result.Baseline.SystemInfo.Distro}}</span>
                    </div>
                    <div class="info-item">
                        <span class="info-label">Snapshot:</span>
                        <span class="info-value">{{formatTime .Result.Baseline.SystemInfo.Timestamp}}</span>
                    </div>
                    <div class="info-item">
                        <span class="info-label">Files:</span>
                        <span class="info-value">{{.Result.Current.Stats.FileCount}}</span>
                    </div>
                </div>
            </div>

            <!-- Critical Changes -->
            {{if .CriticalChanges}}
            <div class="critical-changes">
                <h2>🚨 Critical Changes</h2>
                <p>These changes affect security-sensitive files and should be investigated immediately.</p>
                <table class="changes-table">
                    <thead>
                        <tr>
                            <th>Severity</th>
                            <th>Type</th>
                            <th>Path</th>
                            <th>Reason</th>
                            <th>Size</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .CriticalChanges}}
                        <tr>
                            <td><span class="{{getSeverityColor .Severity}}">{{.Severity}}/10</span></td>
                            <td><span class="change-type {{.Type}}">{{getIcon .Type}} {{.Type}}</span></td>
                            <td class="path-cell">{{.Path}}</td>
                            <td>{{.Reason}}</td>
                            <td>{{formatBytes .Record.Size}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}

            <!-- Detailed Changes -->
            <div class="section">
                <h2>📋 Detailed Changes</h2>
                
                <div class="tabs">
                    <button class="tab active" onclick="showTab('added')">➕ Added ({{.Result.Summary.AddedCount}})</button>
                    <button class="tab" onclick="showTab('modified')">🔄 Modified ({{.Result.Summary.ModifiedCount}})</button>
                    <button class="tab" onclick="showTab('deleted')">❌ Deleted ({{.Result.Summary.DeletedCount}})</button>
                </div>

                <!-- Added Files -->
                <div id="added" class="tab-content active">
                    {{if .Result.Added}}
                    <table class="changes-table">
                        <thead>
                            <tr>
                                <th>Path</th>
                                <th>Size</th>
                                <th>Mode</th>
                                <th>Modified</th>
                                <th>Hash</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range $path, $record := .Result.Added}}
                            <tr>
                                <td class="path-cell">{{$path}}</td>
                                <td>{{formatBytes $record.Size}}</td>
                                <td>{{$record.Mode}}</td>
                                <td>{{formatTime $record.ModTime}}</td>
                                <td>{{truncate $record.Hash 16}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                    {{else}}
                    <p>No files were added.</p>
                    {{end}}
                </div>

                <!-- Modified Files -->
                <div id="modified" class="tab-content">
                    {{if .Result.Modified}}
                    <table class="changes-table">
                        <thead>
                            <tr>
                                <th>Path</th>
                                <th>Size</th>
                                <th>Mode</th>
                                <th>Modified</th>
                                <th>Changes</th>
                                <th>Hash</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range $path, $change := .Result.Modified}}
                            <tr>
                                <td class="path-cell">{{$path}}</td>
                                <td>{{formatBytes $change.NewRecord.Size}}</td>
                                <td>{{$change.NewRecord.Mode}}</td>
                                <td>{{formatTime $change.NewRecord.ModTime}}</td>
                                <td>{{join $change.Changes ", "}}</td>
                                <td>{{truncate $change.NewRecord.Hash 16}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                    {{else}}
                    <p>No files were modified.</p>
                    {{end}}
                </div>

                <!-- Deleted Files -->
                <div id="deleted" class="tab-content">
                    {{if .Result.Deleted}}
                    <table class="changes-table">
                        <thead>
                            <tr>
                                <th>Path</th>
                                <th>Size</th>
                                <th>Mode</th>
                                <th>Modified</th>
                                <th>Hash</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range $path, $record := .Result.Deleted}}
                            <tr>
                                <td class="path-cell">{{$path}}</td>
                                <td>{{formatBytes $record.Size}}</td>
                                <td>{{$record.Mode}}</td>
                                <td>{{formatTime $record.ModTime}}</td>
                                <td>{{truncate $record.Hash 16}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                    {{else}}
                    <p>No files were deleted.</p>
                    {{end}}
                </div>
            </div>

            <!-- Statistics -->
            <div class="section">
                <h2>📈 Statistics</h2>
                <div class="system-info">
                    <div>
                        <h3>Size Changes</h3>
                        <div class="info-item">
                            <span class="info-label">Added Size:</span>
                            <span class="info-value">{{formatBytes .Result.Summary.AddedSize}}</span>
                        </div>
                        <div class="info-item">
                            <span class="info-label">Deleted Size:</span>
                            <span class="info-value">{{formatBytes .Result.Summary.DeletedSize}}</span>
                        </div>
                        <div class="info-item">
                            <span class="info-label">Net Change:</span>
                            <span class="info-value">{{formatBytes .Result.Summary.SizeDiff}}</span>
                        </div>
                    </div>
                    <div>
                        <h3>Performance</h3>
                        <div class="info-item">
                            <span class="info-label">Comparison Time:</span>
                            <span class="info-value">{{.Result.Summary.ComparisonTime}}</span>
                        </div>
                        <div class="info-item">
                            <span class="info-label">Baseline Scan:</span>
                            <span class="info-value">{{.Result.Baseline.Stats.ScanDuration}}</span>
                        </div>
                        <div class="info-item">
                            <span class="info-label">Current Scan:</span>
                            <span class="info-value">{{.Result.Current.Stats.ScanDuration}}</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <div class="footer">
            <p>Generated by fsdiff v2.0 • Report generated in {{.Result.Summary.ComparisonTime}}</p>
        </div>
    </div>

    <script>
        function showTab(tabName) {
            // Hide all tab contents
            const contents = document.querySelectorAll('.tab-content');
            contents.forEach(content => content.classList.remove('active'));
            
            // Remove active class from all tabs
            const tabs = document.querySelectorAll('.tab');
            tabs.forEach(tab => tab.classList.remove('active'));
            
            // Show selected tab content
            document.getElementById(tabName).classList.add('active');
            
            // Add active class to clicked tab
            event.target.classList.add('active');
        }

        // Add search functionality
        function addSearchBox() {
            const tables = document.querySelectorAll('.changes-table tbody');
            tables.forEach(table => {
                const rows = Array.from(table.querySelectorAll('tr'));
                
                // Create search input
                const searchInput = document.createElement('input');
                searchInput.type = 'text';
                searchInput.placeholder = 'Search paths...';
                searchInput.style.cssText = 'width: 100%; padding: 0.5rem; margin-bottom: 1rem; border: 1px solid #ddd; border-radius: 4px;';
                
                // Insert before table
                table.parentElement.insertBefore(searchInput, table.parentElement);
                
                // Add search functionality
                searchInput.addEventListener('input', function() {
                    const query = this.value.toLowerCase();
                    rows.forEach(row => {
                        const pathCell = row.querySelector('.path-cell');
                        if (pathCell) {
                            const path = pathCell.textContent.toLowerCase();
                            row.style.display = path.includes(query) ? '' : 'none';
                        }
                    });
                });
            });
        }

        // Initialize search boxes when page loads
        document.addEventListener('DOMContentLoaded', addSearchBox);
        
        // Add sorting functionality
        function addTableSorting() {
            const tables = document.querySelectorAll('.changes-table');
            tables.forEach(table => {
                const headers = table.querySelectorAll('th');
                headers.forEach((header, index) => {
                    header.style.cursor = 'pointer';
                    header.addEventListener('click', () => sortTable(table, index));
                });
            });
        }

        function sortTable(table, columnIndex) {
            const tbody = table.querySelector('tbody');
            const rows = Array.from(tbody.querySelectorAll('tr'));
            
            rows.sort((a, b) => {
                const aText = a.cells[columnIndex].textContent.trim();
                const bText = b.cells[columnIndex].textContent.trim();
                return aText.localeCompare(bText);
            });
            
            // Clear tbody and re-append sorted rows
            tbody.innerHTML = '';
            rows.forEach(row => tbody.appendChild(row));
        }

        // Initialize sorting when page loads
        document.addEventListener('DOMContentLoaded', addTableSorting);
    </script>
</body>
</html>`
