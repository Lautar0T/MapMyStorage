package export

import (
	"bytes"
	"errors"
	"github.com/lautar0t/MapMyStorage/core/models"
	"strings"
	"testing"
)

type brokenWriter struct{}

func (brokenWriter) Write(p []byte) (int, error) { return 0, errors.New("disk full") }
func TestCSVFlushError(t *testing.T) {
	if err := ExportCSV(&models.Entry{Path: "/"}, brokenWriter{}, ExportOptions{}); err == nil {
		t.Fatal("lost writer error")
	}
}
func TestExportsRetainCoverage(t *testing.T) {
	e := &models.Entry{Path: "/denied", Incomplete: true, ScanError: "permission denied", Report: &models.ScanReport{Errors: 1}}
	for _, f := range []func(*models.Entry, *bytes.Buffer) error{
		func(e *models.Entry, b *bytes.Buffer) error { return ExportJSON(e, b, ExportOptions{}) },
		func(e *models.Entry, b *bytes.Buffer) error { return ExportCSV(e, b, ExportOptions{}) },
	} {
		var b bytes.Buffer
		if err := f(e, &b); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "permission denied") {
			t.Fatal("export hid incomplete coverage")
		}
	}
}
