// Tests for number and time formatting, markers, and the progress lines.
package ui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

func TestSize(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 999: "999 B", 1000: "1.0 KB", 38_200_000_000: "38.2 GB",
		999_960: "1.0 MB", 1_500_000_000_000: "1.5 TB"} {
		if got := Size(in); got != want {
			t.Errorf("Size(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDuration(t *testing.T) {
	for in, want := range map[time.Duration]string{3 * time.Second: "3s", 842 * time.Second: "14m 02s",
		10 * time.Minute: "10m", 125 * time.Minute: "2h 05m", 76 * time.Hour: "3d 04h"} {
		if got := Duration(in); got != want {
			t.Errorf("Duration(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestCount(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1204: "1,204", 1234567: "1,234,567", -1000: "-1,000"} {
		if got := Count(in); got != want {
			t.Errorf("Count(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPrinterMarkersAndQuiet(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	p.Line(MarkDeferred, "first\nsecond")
	if buf.String() != "DEFERRED  first\n          second\n" {
		t.Fatalf("got %q", buf.String())
	}
	buf.Reset()
	p.Quiet = true
	p.Line(MarkNote, "hidden")
	p.Line(MarkResult, "shown")
	if buf.String() != "RESULT    shown\n" {
		t.Fatalf("quiet output: %q", buf.String())
	}
}

func TestEveryProblemSaysWhatToDo(t *testing.T) {
	problems := []error{NoConfig("f"), NotMounted("/mnt/z", "/mnt/z/a"), DestinationMissing("/a", nil),
		RsyncTooOld("/usr/bin/rsync", "3.1.3"), PathsOverlap("source", "/a", "destination", "/a/b"),
		AlreadyRunning("default", "run", 42), StateNotLocal("/mnt/c/x", "a Windows drive")}
	for _, err := range problems {
		var p *Problem
		if !errors.As(err, &p) || p.What == "" || p.Fix == "" {
			t.Errorf("%v: missing what or fix", err)
		}
		if !strings.Contains(Explain(err), "What to do:") {
			t.Errorf("%v: explanation lacks a fix", err)
		}
	}
}

func TestStepsAndColourByResult(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	p.Step("Scanning the source")
	p.Detail("Found 3 files")
	FileProgress(p, "copied", "a\nb.txt", "6 B", 3, 12)
	want := "==> Scanning the source\n    Found 3 files\n    [ 3/12] copied    \"a\\nb.txt\"  6 B\n"
	if buf.String() != want {
		t.Fatalf("got %q\nwant %q", buf.String(), want)
	}
	// With colour, the verdict of a RESULT line is green, yellow or red.
	for text, style := range map[string]string{"OK: done": styleGood, "PARTIAL: later": styleWait,
		"FAILED: 1 problem": styleBad, "NOT SAFE: no": styleBad, "SAFE: yes": styleGood} {
		buf.Reset()
		p := &Printer{Out: &buf, Colour: true}
		p.Line(MarkResult, text)
		if !strings.Contains(buf.String(), style) {
			t.Errorf("%q: colour %q missing in %q", text, style, buf.String())
		}
	}
}

func TestScanAndPlanDetails(t *testing.T) {
	s := destination.RunSummary{Scanned: 5, ScannedBytes: 2000, Excluded: 1,
		Deferred: []destination.DeferredFile{{Path: "x"}}, Planned: 2, PlannedBytes: 1500, UpToDate: 2,
		Differing: []string{"d"}}
	if got, want := ScanDetail(s), "Found 5 files; 4 ready (2.0 KB); 1 changed recently (left for a later run); 1 excluded"; got != want {
		t.Errorf("ScanDetail = %q, want %q", got, want)
	}
	if got, want := PlanDetail(s), "2 files to copy (1.5 KB); 2 already copied and verified; 1 differs from the copy on ceph (left as it is)"; got != want {
		t.Errorf("PlanDetail = %q, want %q", got, want)
	}
}

func TestProfilesSummaryTakesTheWorstResult(t *testing.T) {
	tests := []struct {
		results []string
		want    string
	}{
		{[]string{destination.ResultOK, destination.ResultOK}, destination.ResultOK},
		{[]string{destination.ResultOK, destination.ResultPartial}, destination.ResultPartial},
		{[]string{destination.ResultPartial, destination.ResultFailed}, destination.ResultFailed},
		{[]string{destination.ResultFailed, destination.ResultInterrupted}, destination.ResultInterrupted},
	}
	for _, tt := range tests {
		var outcomes []ProfileOutcome
		for i, r := range tt.results {
			outcomes = append(outcomes, ProfileOutcome{Profile: fmt.Sprint("p", i), Result: r})
		}
		var buf bytes.Buffer
		if got := ProfilesSummary(&Printer{Out: &buf}, outcomes, len(outcomes), false); got != tt.want {
			t.Errorf("%v: got %s, want %s", tt.results, got, tt.want)
		}
		if !strings.Contains(buf.String(), "RESULT    "+tt.want) {
			t.Errorf("%v: RESULT line missing in %q", tt.results, buf.String())
		}
	}
}
