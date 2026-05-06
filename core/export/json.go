// Package export provides functions for exporting scan results.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/lautaro/real-disk-map/core/models"
)

// ExportOptions holds options for export.
type ExportOptions struct {
	IncludeChildren bool `json:"include_children"`
	MaxDepth        int  `json:"max_depth"` // 0 = unlimited
}

// ExportJSON exports the scan result to JSON.
func ExportJSON(entry *models.Entry, w io.Writer, opts ExportOptions) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entry)
}

// ExportJSONCompact exports to compact JSON (one line per entry).
func ExportJSONCompact(entry *models.Entry, w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(entry)
}

// CSVRecord represents a single row in the CSV export.
type CSVRecord struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	LogicalSize   int64     `json:"logical_size"`
	PhysicalSize  int64     `json:"physical_size"`
	SizeDiff      int64     `json:"size_diff"`
	SizeRatio     float64   `json:"size_ratio"`
	IsCloud       bool      `json:"is_cloud"`
	CloudProvider string    `json:"cloud_provider"`
	CloudStatus   string    `json:"cloud_status"`
	ModTime       time.Time `json:"mod_time"`
}

// ExportCSV exports the scan result to CSV format.
func ExportCSV(entry *models.Entry, w io.Writer, opts ExportOptions) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	headers := []string{
		"path",
		"name",
		"type",
		"logical_size",
		"physical_size",
		"size_diff",
		"size_ratio",
		"is_cloud",
		"cloud_provider",
		"cloud_status",
		"mod_time",
	}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write entries
	err := models.WalkEntry(entry, func(e *models.Entry) error {
		// Skip if depth exceeded
		if opts.MaxDepth > 0 {
			depth := calculateDepth(entry, e)
			if depth > opts.MaxDepth {
				if e.Type == models.EntryTypeDir {
					return fs.SkipDir
				}
				return nil
			}
		}

		record := CSVRecord{
			Path:          e.Path,
			Name:          e.Name,
			Type:          string(e.Type),
			LogicalSize:   e.LogicalSize,
			PhysicalSize:  e.PhysicalSize,
			SizeDiff:      e.LogicalSize - e.PhysicalSize,
			SizeRatio:     e.SizeRatio(),
			IsCloud:       e.IsCloud,
			CloudProvider: string(e.CloudProvider),
			CloudStatus:   string(e.CloudStatus),
			ModTime:       e.ModTime,
		}

		row := []string{
			record.Path,
			record.Name,
			record.Type,
			formatInt(record.LogicalSize),
			formatInt(record.PhysicalSize),
			formatInt(record.SizeDiff),
			formatFloat(record.SizeRatio),
			formatBool(record.IsCloud),
			record.CloudProvider,
			record.CloudStatus,
			record.ModTime.Format(time.RFC3339),
		}

		return writer.Write(row)
	})

	if err != nil {
		return fmt.Errorf("failed to write CSV data: %w", err)
	}

	return nil
}

// ExportSummary exports a summary of the scan.
func ExportSummary(entry *models.Entry, w io.Writer) error {
	type Summary struct {
		RootPath          string    `json:"root_path"`
		TotalFiles        int       `json:"total_files"`
		TotalDirs         int       `json:"total_dirs"`
		TotalLogicalSize  int64     `json:"total_logical_size"`
		TotalPhysicalSize int64     `json:"total_physical_size"`
		SpaceSaved        int64     `json:"space_saved"`
		CloudFiles        int       `json:"cloud_files"`
		CloudOnlineOnly   int       `json:"cloud_online_only"`
		Symlinks          int       `json:"symlinks"`
		ExportedAt        time.Time `json:"exported_at"`
	}

	var totalFiles, totalDirs, cloudFiles, cloudOnlineOnly, symlinks int

	models.WalkEntry(entry, func(e *models.Entry) error {
		switch e.Type {
		case models.EntryTypeFile:
			totalFiles++
		case models.EntryTypeDir:
			totalDirs++
		case models.EntryTypeSymlink:
			symlinks++
		}

		if e.IsCloud {
			cloudFiles++
			if e.CloudStatus == models.CloudStatusOnlineOnly {
				cloudOnlineOnly++
			}
		}

		return nil
	})

	summary := Summary{
		RootPath:          entry.Path,
		TotalFiles:        totalFiles,
		TotalDirs:         totalDirs,
		TotalLogicalSize:  entry.TotalLogicalSize(),
		TotalPhysicalSize: entry.TotalPhysicalSize(),
		SpaceSaved:        entry.TotalLogicalSize() - entry.TotalPhysicalSize(),
		CloudFiles:        cloudFiles,
		CloudOnlineOnly:   cloudOnlineOnly,
		Symlinks:          symlinks,
		ExportedAt:        time.Now(),
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

// calculateDepth calculates the depth of a child entry relative to root.
func calculateDepth(root, child *models.Entry) int {
	depth := 0
	for p := child; p != nil && p != root; p = p.Parent {
		depth++
	}
	return depth
}

// formatInt formats an int64 as a string.
func formatInt(v int64) string {
	return fmt.Sprintf("%d", v)
}

// formatFloat formats a float64 as a string.
func formatFloat(v float64) string {
	return fmt.Sprintf("%.4f", v)
}

// formatBool formats a bool as a string.
func formatBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// ToJSON returns the entry as a JSON string.
func ToJSON(entry *models.Entry) (string, error) {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSONCompact returns the entry as a compact JSON string.
func ToJSONCompact(entry *models.Entry) (string, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
