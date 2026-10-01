// mdlinks checks that every relative link in the repository's Markdown files
// points to a file or folder that exists. Web links (http, https, mailto) are
// not checked, so the check works offline. Run by scripts/check.sh.
//
// Usage: go run ./internal/tools/mdlinks [ROOT]
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// linkPattern finds Markdown links and images: [text](target).
var linkPattern = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

// skipDirs are folders whose Markdown is not ours.
var skipDirs = map[string]bool{".git": true, "node_modules": true, ".test-tmp": true}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	broken := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range linkPattern.FindAllStringSubmatch(stripCode(string(data)), -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), target)); err != nil {
				fmt.Printf("%s: broken link to %s\n", path, m[1])
				broken++
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdlinks:", err)
		os.Exit(1)
	}
	if broken > 0 {
		os.Exit(1)
	}
}

// codeBlock matches fenced code blocks and inline code, which may contain
// text that looks like a link but is not one.
var codeBlock = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")

func stripCode(s string) string { return codeBlock.ReplaceAllString(s, "") }
