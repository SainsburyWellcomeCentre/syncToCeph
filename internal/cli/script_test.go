// This file runs the scenario tests in testdata/script/*.txtar. Each scenario
// reads like a terminal session: commands to run and the output to expect.
// Every scenario runs in its own temporary folder, with HOME and the XDG
// folders pointing inside it, so no real data or configuration is touched.
//
// Extra commands available in scenarios (besides testscript's own):
//
//	config [key=value ...]   write the profile config (defaults below)
//	age DURATION PATH...     set the modification time to DURATION ago
//	waitfor PATH             wait until PATH exists
//	waitlog [N] REGEXP       wait until the log has N lines matching REGEXP
//	snapshot DIR FILE        record names, sizes, times and SHA-256 of DIR
//	killpid FILE             kill the process group whose ID is in FILE
//	mkfifo PATH              create a named pipe
package cli

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

// scriptWait is how long waitfor and waitlog wait before failing.
const scriptWait = 20 * time.Second

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"synctoceph": func() { os.Exit(Main()) },
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 filepath.Join("..", "..", "testdata", "script"),
		RequireExplicitExec: true,
		Setup:               setupScript,
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"config":   cmdConfig,
			"age":      cmdAge,
			"waitfor":  cmdWaitfor,
			"waitlog":  cmdWaitlog,
			"snapshot": cmdSnapshot,
			"killpid":  cmdKillpid,
			"mkfifo":   cmdMkfifo,
		},
	})
}

// setupScript points HOME and the XDG folders into the scenario's work folder
// and limits PATH to the test program and the system folders (so Windows
// programs such as schtasks.exe are never run by a test).
func setupScript(env *testscript.Env) error {
	home := filepath.Join(env.WorkDir, "home")
	env.Setenv("HOME", home)
	env.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	env.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	env.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	env.Setenv("NO_COLOR", "1")
	env.Setenv("TZ", "UTC")
	env.Setenv("WSL_DISTRO_NAME", "")
	bin := strings.Split(env.Getenv("PATH"), string(os.PathListSeparator))[0]
	env.Setenv("PATH", bin+":/usr/bin:/bin")
	// Stand-in rsync programs record their process ID in $WORK/rsync-pid.
	// Kill them when the scenario ends, even if it failed half-way.
	env.Defer(func() {
		if data, err := os.ReadFile(filepath.Join(env.WorkDir, "rsync-pid")); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 1 {
				syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
	})
	return os.MkdirAll(home, 0o700)
}

// cmdConfig writes the config of a profile. Defaults: machine_name
// "scope-01", source $WORK/src, archive $WORK/archive, settle_time "0s".
// The pseudo-key profile=NAME chooses the profile.
func cmdConfig(ts *testscript.TestScript, neg bool, args []string) {
	work := ts.Getenv("WORK")
	values := map[string]string{"machine_name": `"scope-01"`, "source": fmt.Sprintf("%q", work+"/src"),
		"archive": fmt.Sprintf("%q", work+"/archive"), "settle_time": `"0s"`}
	order := []string{"machine_name", "source", "archive", "settle_time"}
	profile := "default"
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			ts.Fatalf("config: want key=value, got %q", arg)
		}
		if key == "profile" {
			profile = value
			continue
		}
		if _, known := values[key]; !known {
			order = append(order, key)
		}
		if value != "true" && value != "false" && !strings.HasPrefix(value, "[") {
			value = strconv.Quote(value)
		}
		values[key] = value
	}
	var b strings.Builder
	for _, key := range order {
		fmt.Fprintf(&b, "%s = %s\n", key, values[key])
	}
	dir := filepath.Join(ts.Getenv("XDG_CONFIG_HOME"), "synctoceph")
	ts.Check(os.MkdirAll(dir, 0o700))
	ts.Check(os.WriteFile(filepath.Join(dir, profile+".toml"), []byte(b.String()), 0o600))
}

// cmdAge sets modification times to DURATION ago.
func cmdAge(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: age DURATION PATH...")
	}
	d, err := time.ParseDuration(args[0])
	ts.Check(err)
	when := time.Now().Add(-d)
	for _, path := range args[1:] {
		ts.Check(os.Chtimes(ts.MkAbs(path), when, when))
	}
}

// waitUntil polls check until it returns true or scriptWait passes.
func waitUntil(ts *testscript.TestScript, what string, check func() bool) {
	deadline := time.Now().Add(scriptWait)
	for !check() {
		if time.Now().After(deadline) {
			ts.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func cmdWaitfor(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: waitfor PATH")
	}
	waitUntil(ts, args[0], func() bool { _, err := os.Stat(ts.MkAbs(args[0])); return err == nil })
}

func cmdWaitlog(ts *testscript.TestScript, neg bool, args []string) {
	count := 1
	if len(args) == 2 {
		var err error
		count, err = strconv.Atoi(args[0])
		ts.Check(err)
		args = args[1:]
	}
	if len(args) != 1 {
		ts.Fatalf("usage: waitlog [N] REGEXP")
	}
	re := regexp.MustCompile(args[0])
	log := filepath.Join(ts.Getenv("XDG_STATE_HOME"), "synctoceph", "default", "sync.log")
	waitUntil(ts, fmt.Sprintf("%d log lines matching %q", count, args[0]), func() bool {
		data, _ := os.ReadFile(log)
		return len(re.FindAllIndex(data, -1)) >= count
	})
}

// cmdSnapshot records every entry of DIR so two snapshots can be compared
// with cmp (to prove a folder did not change).
func cmdSnapshot(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: snapshot DIR FILE")
	}
	root := ts.MkAbs(args[0])
	var lines []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(path)
			lines = append(lines, rel+" -> "+target)
		case d.IsDir():
			lines = append(lines, rel+"/")
		case d.Type().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lines = append(lines, fmt.Sprintf("%s %d %d %x", rel, info.Size(), info.ModTime().UnixNano(), sha256.Sum256(data)))
		default:
			lines = append(lines, rel+" (special)")
		}
		return nil
	})
	ts.Check(err)
	sort.Strings(lines)
	ts.Check(os.WriteFile(ts.MkAbs(args[1]), []byte(strings.Join(lines, "\n")+"\n"), 0o644))
}

// cmdKillpid kills the process group whose leader's ID is in FILE, if it is
// still running. Scenarios use it to clean up processes they started.
func cmdKillpid(ts *testscript.TestScript, neg bool, args []string) {
	data, err := os.ReadFile(ts.MkAbs(args[0]))
	ts.Check(err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	ts.Check(err)
	syscall.Kill(-pid, syscall.SIGKILL)
	syscall.Kill(pid, syscall.SIGKILL)
	waitUntil(ts, "process "+strconv.Itoa(pid)+" to exit", func() bool {
		return syscall.Kill(pid, 0) != nil
	})
}

func cmdMkfifo(ts *testscript.TestScript, neg bool, args []string) {
	ts.Check(syscall.Mkfifo(ts.MkAbs(args[0]), 0o644))
}
