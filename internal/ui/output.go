// This file prints the lines people see in the terminal. Every line starts
// with a plain-text marker (OK, NOTE, DEFERRED, WARNING, ERROR, RESULT) padded
// to a fixed width, so output stays readable in logs and without colour.
// While a run is working, each step starts with "==>" (the same style as
// install.sh) and its details are indented under it. Colour is added only
// when writing to a terminal and NO_COLOR is not set.
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

// Other ANSI styles: for step lines, headings and secondary details.
const (
	colourReset = "\x1b[0m"
	styleBold   = "\x1b[1m"
	styleDim    = "\x1b[2m"
	styleStep   = "\x1b[1;34m" // bold blue, for the "==>" of a step
	styleGood   = "\x1b[1;32m" // bold green
	styleWait   = "\x1b[1;33m" // bold yellow
	styleBad    = "\x1b[1;31m" // bold red
)

// stepArrow starts a step line; stepIndent lines details up under its text.
const (
	stepArrow  = "==>"
	stepIndent = "    "
)

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
	if marker == MarkResult {
		lines[0] = p.resultWord(lines[0])
	}
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

// Step prints the start of a step of the work, e.g. "==> Scanning the source".
func (p *Printer) Step(text string) {
	p.Plain(p.paint(styleStep, stepArrow) + " " + p.paint(styleBold, text))
}

// Detail prints a line under a step, lined up with the step's text.
func (p *Printer) Detail(text string) {
	p.Plain(stepIndent + text)
}

// Blank prints an empty line, unless quiet.
func (p *Printer) Blank() { p.Plain("") }

// paint wraps text in an ANSI style when colour is on.
func (p *Printer) paint(style, text string) string {
	if !p.Colour || text == "" {
		return text
	}
	return style + text + colourReset
}

// resultWord colours the verdict at the start of a RESULT line ("OK",
// "PARTIAL", "FAILED", ...): green for good, yellow for "not finished yet",
// red for a problem.
func (p *Printer) resultWord(line string) string {
	word, rest, found := strings.Cut(line, ":")
	if !found {
		return line
	}
	return p.paint(resultStyle(word), word) + ":" + rest
}

// resultStyle picks the colour of a verdict such as "OK" or "FAILED".
func resultStyle(word string) string {
	switch {
	case strings.HasPrefix(word, "OK"), word == "SAFE":
		return styleGood
	case strings.HasPrefix(word, "PARTIAL"), strings.HasPrefix(word, "INTERRUPTED"):
		return styleWait
	}
	return styleBad
}
