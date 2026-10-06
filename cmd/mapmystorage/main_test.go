package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lautar0t/MapMyStorage/core/models"
)

// Exercise real argument parsing and exit codes in a subprocess, including
// os.Exit paths, without recompiling a binary for each test case.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("MMS_CLI_TEST") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"mapmystorage"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("mapmystorage", flag.ExitOnError)
	main()
	os.Exit(0)
}
func invoke(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "MMS_CLI_TEST=1", "GORACE=atexit_sleep_ms=0")
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	code := 0
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return out.String(), errout.String(), code
}
func TestCLIHelpAndVersion(t *testing.T) {
	for _, option := range []string{"--help", "-help", "-h"} {
		t.Run(option, func(t *testing.T) {
			out, errout, code := invoke(t, option)
			if code != 0 || errout != "" {
				t.Fatalf("help code=%d stderr=%q", code, errout)
			}
			for _, topic := range []string{"--help", "--version", "--root", "--whole-disk", "--diagnose", "--json", "--csv", "--no-interactive", "--hidden=false", "--follow-symlinks", "--max-depth", "--exclude", "--cross-filesystems", "Examples:", "Exit codes:", "Full Disk Access"} {
				if !strings.Contains(out, topic) {
					t.Errorf("help missing %s", topic)
				}
			}
		})
	}
	out, errout, code := invoke(t, "--version")
	if code != 0 || errout != "" || !strings.Contains(out, "MapMyStorage version") {
		t.Fatalf("version: %d %s %s", code, out, errout)
	}
}
func TestCLIInvalidOptions(t *testing.T) {
	cases := [][]string{
		{"--json", "--csv"}, {"--max-depth", "-1"}, {"--max-depth", "oops"},
		{"--whole-disk", "--root", "."}, {"--whole-disk", "."}, {"--root", ".", "."},
		{".", "--json"}, {".", "."}, {"--not-an-option"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, errout, code := invoke(t, args...)
			if code != 2 || out != "" || errout == "" {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, out, errout)
			}
		})
	}
	_, _, code := invoke(t, "--root", filepath.Join(t.TempDir(), "missing"))
	if code != 1 {
		t.Fatalf("missing root exit=%d", code)
	}
}
func TestCLIExportAndRedirectedText(t *testing.T) {
	root := filepath.Join(t.TempDir(), "folder with spaces á")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".hidden", "nested/file"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("local data"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	out, errout, code := invoke(t, "--json", "--max-depth", "1", "--root", root)
	if code != 0 || errout != "" {
		t.Fatalf("JSON: %d %s", code, errout)
	}
	var entry models.Entry
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Report == nil || entry.Report.Files != 2 || len(entry.Report.LargestFiles) != 2 {
		t.Fatal("missing deep/hidden accounting")
	}
	out, errout, code = invoke(t, "--csv", "--root", root)
	if code != 0 || errout != "" {
		t.Fatalf("CSV: %d %s", code, errout)
	}
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || !strings.Contains(strings.Join(rows[0], ","), "scan_error") {
		t.Fatalf("invalid CSV rows: %v", rows)
	}
	// A pipe must select text automatically, without opening a TUI or reading stdin.
	out, errout, code = invoke(t, root)
	if code != 0 || errout != "" || !strings.Contains(out, "Allocated on disk:") {
		t.Fatalf("text: %d %s %s", code, out, errout)
	}
	out, _, code = invoke(t, "--json", "--hidden=false", root)
	if code != 0 {
		t.Fatal("hidden filter failed")
	}
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatal(err)
	}
	if !entry.Incomplete || entry.Report.Files != 1 || entry.Report.Skipped != 1 {
		t.Fatal("filter coverage missing")
	}
}
