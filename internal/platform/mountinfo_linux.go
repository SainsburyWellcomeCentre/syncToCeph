// This file reads /proc/self/mountinfo, the Linux kernel's list of mounted
// filesystems. synctoceph uses it to check that the lab share is really
// mounted, to find out what kind of filesystem a folder is on (a Windows
// drive, a network share or a local disk), and to suggest mount commands.
// Reading this list never touches the mounted drives themselves.
package platform

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// mountinfoPath is where Linux publishes the mount table for this process.
const mountinfoPath = "/proc/self/mountinfo"

// ReadMounts returns every mounted filesystem, in the kernel's order (a
// later entry for the same mount point is mounted on top of an earlier one).
func ReadMounts() ([]Mount, error) {
	f, err := os.Open(mountinfoPath)
	if err != nil {
		return nil, fmt.Errorf("reading the mount table %s: %w", mountinfoPath, err)
	}
	defer f.Close()
	var mounts []Mount
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if m, ok := parseMountinfoLine(scanner.Text()); ok {
			mounts = append(mounts, m)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading the mount table %s: %w", mountinfoPath, err)
	}
	return mounts, nil
}

// parseMountinfoLine reads one line such as
//
//	36 35 98:0 / /mnt/z rw,noatime - 9p Z:\134 rw,aname=drvfs;path=Z:\
//
// Fields before " - " are fixed plus optional tags; after it come the
// filesystem type, the source and the filesystem options.
func parseMountinfoLine(line string) (Mount, bool) {
	before, after, found := strings.Cut(line, " - ")
	if !found {
		return Mount{}, false
	}
	left := strings.Fields(before)
	right := strings.Fields(after)
	if len(left) < 6 || len(right) < 2 {
		return Mount{}, false
	}
	m := Mount{Point: unescapeMount(left[4]), FSType: right[0], Source: unescapeMount(right[1])}
	if len(right) > 2 {
		m.Options = unescapeMount(right[2])
	}
	return m, true
}

// unescapeMount decodes the octal escapes (\040 for a space and so on) that
// the kernel uses in the mount table.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// IsWSL reports whether this Linux system is running under Windows (WSL).
func IsWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// HasSystemd reports whether systemd is running as the service manager.
func HasSystemd() bool {
	info, err := os.Stat("/run/systemd/system")
	return err == nil && info.IsDir()
}
