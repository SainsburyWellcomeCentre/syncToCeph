// Tests for number and time formatting, and markers.
package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
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
	problems := []error{NoConfig("f"), NotMounted("/mnt/z", "/mnt/z/a"), ArchiveMissing("/a", nil),
		RsyncTooOld("/usr/bin/rsync", "3.1.3"), PathsOverlap("source", "/a", "archive", "/a/b"),
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
