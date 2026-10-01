// This file defines Problem, the error type for anything a person has to act
// on. A Problem always says what happened, why it matters and what to do
// next, so no error leaves the user without a way forward.
package ui

import (
	"errors"
	"strings"
)

// Problem is an error with a plain-language explanation and a fix.
type Problem struct {
	// What happened, in one sentence.
	What string
	// Why it matters (may be empty when it is obvious from What).
	Why string
	// What to do next.
	Fix string
	// The underlying error, if any. It is shown as a detail line.
	Err error
}

// Error returns the one-line form, used in logs and JSON output.
func (p *Problem) Error() string {
	if p.Err != nil {
		return p.What + ": " + p.Err.Error()
	}
	return p.What
}

// Unwrap lets errors.Is and errors.As see the underlying error.
func (p *Problem) Unwrap() error { return p.Err }

// Explain returns the multi-line explanation of err for the terminal. For a
// Problem it includes the reason and the fix; other errors get a generic hint.
func Explain(err error) string {
	var p *Problem
	if !errors.As(err, &p) {
		return err.Error() + "\n" + "What to do: " + GenericFix
	}
	var b strings.Builder
	b.WriteString(p.What)
	if p.Err != nil {
		b.WriteString("\nDetail: " + p.Err.Error())
	}
	if p.Why != "" {
		b.WriteString("\nWhy it matters: " + p.Why)
	}
	if p.Fix != "" {
		b.WriteString("\nWhat to do: " + p.Fix)
	}
	return b.String()
}
