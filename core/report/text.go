// Package report presents measured usage and the limits of the measurement.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/lautar0t/MapMyStorage/core/models"
)

func Bytes(b int64) string {
	if b < 0 {
		return "-" + Bytes(-b)
	}
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(1024), 0
	for n := b / 1024; n >= 1024 && exp < 5; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func Text(root *models.Entry) string {
	var b strings.Builder
	Write(&b, root)
	return b.String()
}

func Write(w io.Writer, root *models.Entry) {
	fmt.Fprintf(w, "MapMyStorage — %s\n", root.Path)
	fmt.Fprintf(w, "Allocated on disk: %s\nLogical size:      %s\n", Bytes(root.PhysicalSize), Bytes(root.LogicalSize))
	fmt.Fprintln(w, "Allocated = filesystem blocks, not guaranteed space freed by deletion.")
	fmt.Fprintln(w, "Logical = file lengths; sparse/compressed/cloud files may allocate much less.")
	r := root.Report
	if r != nil {
		if !r.FinishedAt.IsZero() {
			fmt.Fprintf(w, "Scan completed: %s (%.1f seconds)\n", r.FinishedAt.Format("2006-01-02 15:04:05 MST"), r.FinishedAt.Sub(r.StartedAt).Seconds())
		}
		fmt.Fprintf(w, "\nCoverage: %d files, %d directories; %d errors, %d skipped, %d duplicate references\n", r.Files, r.Directories, r.Errors, r.Skipped, r.Duplicates)
		if root.Incomplete {
			fmt.Fprintln(w, "PARTIAL: some paths could not be measured. Unknown usage is not zero.")
		}
		if r.Config.MaxDepth > 0 {
			fmt.Fprintln(w, "Depth limits retained tree detail, not the scan; totals and largest files include deeper entries.")
		}
		if v := r.Volume; v != nil {
			fmt.Fprintf(w, "\nFilesystem capacity: %s (%.2f GB decimal)\nFree: %s | Available to current user: %s\n", Bytes(v.Capacity), float64(v.Capacity)/1e9, Bytes(v.Free), Bytes(v.Available))
			occupied := v.Capacity - v.Free
			fmt.Fprintf(w, "Capacity minus free: %s\nDifference versus scanned allocation: %s\n", Bytes(occupied), Bytes(occupied-root.PhysicalSize))
			fmt.Fprintln(w, "This difference is NOT a measured 'System Data' or reclaimable amount.")
			fmt.Fprintln(w, "On APFS, volumes share container space. Snapshots, metadata, unscanned paths,")
			fmt.Fprintln(w, "open deleted files and shared clone blocks prevent an exact reconciliation.")
		} else {
			fmt.Fprintf(w, "\nVolume information unavailable: %s\n", r.VolumeError)
		}
		fmt.Fprintln(w, "\nCloud files (provider path hints; online-only requires OS evidence):")
		providers := make([]string, 0, len(r.Cloud))
		for p := range r.Cloud {
			providers = append(providers, string(p))
		}
		sort.Strings(providers)
		if len(providers) == 0 {
			fmt.Fprintln(w, "  No cloud files identified in accessible paths; this does not prove cloud usage is zero.")
		}
		for _, p := range providers {
			c := r.Cloud[models.CloudProvider(p)]
			fmt.Fprintf(w, "  %-12s allocated %12s | logical %12s | %d files, %d online-only, %d unknown, %d unenumerated directories\n", p, Bytes(c.Allocated), Bytes(c.Logical), c.Files, c.OnlineOnly, c.Unknown, c.SkippedDirectories)
		}
		fmt.Fprintln(w, "  Provider caches/databases outside sync folders appear separately in the folder totals.")
	}
	fmt.Fprintln(w, "\nRoot folders/files (non-overlapping; sorted by allocated size):")
	children := append([]*models.Entry(nil), root.Children...)
	models.SortDescendingEntries(children, models.SortByPhysicalSize)
	for i, e := range children {
		if i >= 25 {
			break
		}
		row(w, e)
	}
	fmt.Fprintln(w, "\nLargest directories (nested rows overlap; do not add them together):")
	var dirs []*models.Entry
	if r != nil && len(r.LargestDirectories) > 0 {
		dirs = append(dirs, r.LargestDirectories...)
	} else {
		_ = models.WalkEntry(root, func(e *models.Entry) error {
			if e != root && e.Type == models.EntryTypeDir && e.CountedElsewhere == "" {
				dirs = append(dirs, e)
			}
			return nil
		})
	}

	models.SortDescendingEntries(dirs, models.SortByPhysicalSize)
	for i, e := range dirs {
		if i >= 20 {
			break
		}
		row(w, e)
	}
	if r != nil {
		fmt.Fprintln(w, "\nLargest individual files (allocated | logical | path):")
		for _, e := range r.LargestFiles {
			row(w, e)
		}
		if len(r.Issues) > 0 {
			fmt.Fprintln(w, "\nUnmeasured/filtered paths (sample; JSON contains up to 100):")
			for i, issue := range r.Issues {
				if i >= 15 {
					break
				}
				fmt.Fprintf(w, "  %s: %s\n", issue.Path, issue.Reason)
			}
			fmt.Fprintln(w, "On macOS, grant Full Disk Access to the terminal/host running this command,")
			fmt.Fprintln(w, "then restart it and scan again. sudo alone does not bypass privacy restrictions.")
		}
		for _, check := range r.Diagnostics {
			fmt.Fprintf(w, "\n$ %s\n", check.Command)
			if check.Error != "" {
				fmt.Fprintf(w, "UNAVAILABLE: %s\n", check.Error)
			}
			fmt.Fprintln(w, check.Output)
		}
	}
	fmt.Fprintln(w, "\nInspect large Library/Application Support, Caches, Containers, Group Containers,")
	fmt.Fprintln(w, "developer data, virtual machines, backups and /private/var. 'System Data' is a")
	fmt.Fprintln(w, "macOS category, not a directory. Snapshot listings do not establish reclaimable size.")
}
func row(w io.Writer, e *models.Entry) {
	status := ""
	switch {
	case e.ScanError != "":
		status = " [ERROR]"
	case e.Skipped != "":
		status = " [SKIPPED]"
	case e.CountedElsewhere != "":
		status = " [counted at " + e.CountedElsewhere + "]"
	case e.Incomplete:
		status = " [PARTIAL]"
	}
	fmt.Fprintf(w, "  %12s | %12s | %s%s\n", Bytes(e.PhysicalSize), Bytes(e.LogicalSize), e.Path, status)
}
