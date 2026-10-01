// This file reads rsync's --itemize-changes output, one line per file, such
// as ">f+++++++++ session_1/stack_0001.tif". The first 11 characters say what
// happened (">f" = a file was received); the rest is the file name. rsync
// writes unusual characters in names as \#ooo (octal), so a newline in a file
// name cannot break the line structure.
package engine

import (
	"regexp"
	"strconv"
	"strings"
)

// itemizePattern matches an itemized line: update type, file type, nine
// attribute flags, a space, then the name.
var itemizePattern = regexp.MustCompile(`^([<>ch.*])([fdLDS])([.+ ?a-zA-Z]{9}) (.+)$`)

// Item is one parsed itemized line.
type Item struct {
	Update   byte   // '>' received, 'c' created, '.' unchanged, ...
	FileType byte   // 'f' file, 'd' directory, 'L' symlink, ...
	Name     string // path relative to the destination, decoded
}

// Received reports whether a regular file's content was transferred.
func (i Item) Received() bool { return i.Update == '>' && i.FileType == 'f' }

// ParseItemized parses one line of rsync output. ok is false for lines that
// are not itemized changes (for example warnings).
func ParseItemized(line string) (Item, bool) {
	m := itemizePattern.FindStringSubmatch(line)
	if m == nil {
		return Item{}, false
	}
	return Item{Update: m[1][0], FileType: m[2][0], Name: unescapeRsync(m[4])}, true
}

// unescapeRsync decodes rsync's \#ooo escapes (octal byte values).
func unescapeRsync(s string) string {
	if !strings.Contains(s, `\#`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+5 <= len(s) && s[i+1] == '#' {
			if v, err := strconv.ParseUint(s[i+2:i+5], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 4
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
