package models

import "time"

// ScanReport survives depth-limited tree exports. Issues are sampled, counts are not.
type ScanReport struct {
	StartedAt          time.Time                     `json:"started_at"`
	FinishedAt         time.Time                     `json:"finished_at"`
	Diagnostics        []Diagnostic                  `json:"diagnostics,omitempty"`
	Files              int64                         `json:"files"`
	Directories        int64                         `json:"directories"`
	Errors             int64                         `json:"errors"`
	Skipped            int64                         `json:"skipped"`
	Duplicates         int64                         `json:"duplicates"`
	Issues             []ScanIssue                   `json:"issues,omitempty"`
	Cloud              map[CloudProvider]*CloudUsage `json:"cloud"`
	LargestDirectories []*Entry                      `json:"largest_directories"`
	LargestFiles       []*Entry                      `json:"largest_files"`
	Volume             *VolumeUsage                  `json:"volume,omitempty"`
	VolumeError        string                        `json:"volume_error,omitempty"`
	Config             ScanConfig                    `json:"config"`
}

type ScanIssue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type CloudUsage struct {
	SkippedDirectories int64 `json:"skipped_directories"`
	Files              int64 `json:"files"`
	Logical            int64 `json:"logical_size"`
	Allocated          int64 `json:"allocated_size"`
	OnlineOnly         int64 `json:"online_only"`
	Unknown            int64 `json:"unknown"`
}

// Capacity minus free can include other volumes sharing an APFS container.
// It must not be presented as the scanned directory's exclusive usage.
type VolumeUsage struct {
	Path      string `json:"path"`
	Capacity  int64  `json:"capacity"`
	Free      int64  `json:"free"`
	Available int64  `json:"available"`
}

// Diagnostic keeps the exact OS output and any failure separately.
type Diagnostic struct {
	Command string `json:"command"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}
