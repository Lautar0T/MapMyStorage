package disk

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/lautar0t/MapMyStorage/core/models"
)

// CollectDiagnostics only invokes read-only OS commands. Failures stay visible;
// an unavailable snapshot query must never be reported as zero snapshots.
func CollectDiagnostics(ctx context.Context, path string) []models.Diagnostic {
	if runtime.GOOS != "darwin" {
		return nil
	}
	commands := [][]string{
		{"/usr/sbin/diskutil", "apfs", "list"},
		{"/usr/bin/tmutil", "listlocalsnapshots", "/"},
		{"/usr/sbin/diskutil", "apfs", "listSnapshots", "/"},
		{"/usr/sbin/diskutil", "apfs", "listSnapshots", "/System/Volumes/Data"},
	}
	var checks []models.Diagnostic
	volume, err := diagnosticVolume(path)
	if err != nil {
		checks = append(checks, models.Diagnostic{Command: "Resolve volume for " + path, Error: err.Error()})
	} else {
		commands = append([][]string{{"/usr/sbin/diskutil", "info", volume}}, commands...)
	}
	for _, args := range commands {
		if ctx.Err() != nil {
			break
		}
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := exec.CommandContext(checkCtx, args[0], args[1:]...).CombinedOutput()
		check := models.Diagnostic{Command: strings.Join(args, " "), Output: strings.TrimSpace(string(out))}
		if err != nil {
			check.Error = err.Error()
		}
		if checkCtx.Err() != nil {
			check.Error = checkCtx.Err().Error()
		}
		cancel()
		checks = append(checks, check)
	}
	return checks
}
