// This file prints the lines people see in the terminal. Every line starts
// with a plain-text marker (OK, NOTE, DEFERRED, WARNING, ERROR, RESULT) padded
// to a fixed width, so output stays readable in logs and without colour.
// Colour is added only when writing to a terminal and NO_COLOR is not set.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Markers that start each output line. They are plain words, never emoji.
const (
	MarkOK       = "OK"
	MarkNote     = "NOTE"
	MarkDeferred = "DEFERRED"
	MarkWarning  = "WARNING"
	MarkError    = "ERROR"
	MarkResult   = "RESULT"
)

// markerWidth is the column where message text starts, so continuation lines
// line up under the first line.
const markerWidth = 10

// Indent is the prefix for continuation lines under a marker.
var Indent = strings.Repeat(" ", markerWidth)

// ANSI colour codes, used only when colour is enabled.
var markerColours = map[string]string{
	MarkOK:       "\x1b[32m",
	MarkNote:     "\x1b[36m",
	MarkDeferred: "\x1b[33m",
	MarkWarning:  "\x1b[33m",
	MarkError:    "\x1b[31m",
	MarkResult:   "\x1b[1m",
}

const colourReset = "\x1b[0m"

// Printer writes marker lines to one output stream.
type Printer struct {
	Out    io.Writer
	Colour bool
	// Quiet hides everything except RESULT and ERROR lines.
	Quiet bool
}

// NewPrinter returns a printer for w. Colour is used only if w is a terminal,
// noColour is false and the NO_COLOR environment variable is unset.
func NewPrinter(w io.Writer, noColour, quiet bool) *Printer {
	return &Printer{Out: w, Colour: !noColour && os.Getenv("NO_COLOR") == "" && isTerminal(w), Quiet: quiet}
}

// isTerminal reports whether w is an interactive terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

// Line prints text after a marker. Text may contain several lines; the extra
// lines are indented to line up under the first.
func (p *Printer) Line(marker, text string) {
	if p.Quiet && marker != MarkResult && marker != MarkError {
		return
	}
	label := fmt.Sprintf("%-*s", markerWidth, marker)
	if p.Colour && marker != "" {
		label = markerColours[marker] + strings.TrimRight(label, " ") + colourReset +
			strings.Repeat(" ", markerWidth-len(marker))
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	fmt.Fprintln(p.Out, label+lines[0])
	for _, line := range lines[1:] {
		if line == "" {
			fmt.Fprintln(p.Out)
			continue
		}
		fmt.Fprintln(p.Out, Indent+line)
	}
}

// Plain prints text without a marker (for lists and tables), unless quiet.
func (p *Printer) Plain(text string) {
	if p.Quiet {
		return
	}
	fmt.Fprintln(p.Out, strings.TrimRight(text, "\n"))
}

// Problem prints an error with its explanation and fix.
func (p *Printer) Problem(err error) {
	p.Line(MarkError, Explain(err))
}
