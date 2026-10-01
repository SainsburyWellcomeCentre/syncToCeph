// gendocs writes the command reference (docs/cli/*.md) and the shell
// completion scripts (completions/) from the command definitions, so they
// can never drift from the program. Run it through scripts/gen-docs.sh.
//
// Usage: go run ./internal/tools/gendocs DOCS_DIR COMPLETIONS_DIR
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/cli"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gendocs DOCS_DIR COMPLETIONS_DIR")
		os.Exit(2)
	}
	if err := generate(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

// navigation is the line at the top of every command page, linking back to
// the README, the documentation index and the command overview.
func navigation(filename string) string {
	nav := "[Home](../../README.md) · [All documentation](../README.md)"
	if filepath.Base(filename) != "synctoceph.md" {
		nav += " · [All commands](synctoceph.md)"
	}
	return nav + "\n\n"
}

func generate(docsDir, completionsDir string) error {
	root := cli.NewRoot()
	root.DisableAutoGenTag = true // no dates, so the output only changes when the commands do
	for _, dir := range []string{docsDir, completionsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := doc.GenMarkdownTreeCustom(root, docsDir, navigation, func(link string) string { return link }); err != nil {
		return fmt.Errorf("writing %s: %w", docsDir, err)
	}
	bash, err := os.Create(filepath.Join(completionsDir, "synctoceph.bash"))
	if err != nil {
		return err
	}
	defer bash.Close()
	if err := root.GenBashCompletionV2(bash, true); err != nil {
		return err
	}
	zsh, err := os.Create(filepath.Join(completionsDir, "_synctoceph"))
	if err != nil {
		return err
	}
	defer zsh.Close()
	return root.GenZshCompletion(zsh)
}
