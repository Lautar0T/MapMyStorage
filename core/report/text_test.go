package report

import (
	"github.com/lautar0t/MapMyStorage/core/models"
	"strings"
	"testing"
)

func TestPartialAndSharedSpaceNotReclaimable(t *testing.T) {
	e := &models.Entry{Path: "/", PhysicalSize: 4096, LogicalSize: 1024, Incomplete: true, Report: &models.ScanReport{Errors: 1, Volume: &models.VolumeUsage{Capacity: 8192, Free: 6144}, Issues: []models.ScanIssue{{Path: "/denied", Reason: "permission denied"}}}}
	text := Text(e)
	for _, want := range []string{"PARTIAL", "-2.00 KiB", "NOT a measured", "/denied", "Full Disk Access"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
}
