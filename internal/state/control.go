// This file implements the control socket: a Unix socket in the state folder
// through which `synctoceph stop` asks a running run or scheduler to stop,
// and `synctoceph status` asks what it is doing right now. A socket is used
// rather than process IDs because a process ID can be reused by an unrelated
// program after a crash; the socket only exists while synctoceph is running.
// The state folder is private to the user, so only they can connect.
package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// maxSocketPath is the longest path Linux accepts for a Unix socket; longer
// paths are reached through /proc/self/fd (see withShortPath).
const maxSocketPath = 107

// controlTimeout bounds one request/response exchange on the socket.
const controlTimeout = 5 * time.Second

// Request is sent to a running synctoceph.
type Request struct {
	Command string `json:"command"` // "stop" or "status"
	// Grace is how long rsync gets to stop after SIGINT before it is killed
	// (stop only; zero means the default).
	GraceSeconds int `json:"grace_seconds,omitempty"`
}

// Response is the answer from a running synctoceph.
type Response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Live    *Live  `json:"live,omitempty"`
}

// Live describes what a running process is doing right now.
type Live struct {
	PID       int       `json:"pid"`
	Mode      string    `json:"mode"`
	Phase     string    `json:"phase"`
	RunID     string    `json:"run_id,omitempty"`
	Progress  string    `json:"progress,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Stopping  bool      `json:"stopping"`
}

// ControlServer answers requests until Close is called.
type ControlServer struct {
	listener *net.UnixListener
	path     string
}

// ServeControl starts listening on the control socket in dir. It must only
// be called while holding the state lock, because it removes any socket left
// behind by a process that crashed.
func ServeControl(dir string, handle func(Request) Response) (*ControlServer, error) {
	path := filepath.Join(dir, ControlName)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("%s exists and is not a socket", path)
		}
		os.Remove(path)
	}
	var listener *net.UnixListener
	err := withShortPath(dir, func(short string) error {
		l, err := net.Listen("unix", short)
		if err == nil {
			listener = l.(*net.UnixListener)
			// The short path is only valid now; remove the real one on Close.
			listener.SetUnlinkOnClose(false)
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", path, err)
	}
	os.Chmod(path, 0o600)
	server := &ControlServer{listener: listener, path: path}
	go server.accept(handle)
	return server, nil
}

// withShortPath calls use with a path to the control socket in dir that fits
// Linux's limit on socket path length. Long paths are reached through
// /proc/self/fd/N, where N is an open handle on dir.
func withShortPath(dir string, use func(path string) error) error {
	path := filepath.Join(dir, ControlName)
	if len(path) <= maxSocketPath {
		return use(path)
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return use(fmt.Sprintf("/proc/self/fd/%d/%s", d.Fd(), ControlName))
}

func (s *ControlServer) accept(handle func(Request) Response) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // closed
		}
		go func() {
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(controlTimeout))
			var req Request
			line, err := bufio.NewReader(conn).ReadBytes('\n')
			if err != nil || json.Unmarshal(line, &req) != nil {
				return
			}
			data, _ := json.Marshal(handle(req))
			conn.Write(append(data, '\n'))
		}()
	}
}

// Close stops answering and removes the socket file.
func (s *ControlServer) Close() {
	if s == nil {
		return
	}
	s.listener.Close()
	os.Remove(s.path)
}

// ErrNotRunning means nothing is listening on the control socket.
var ErrNotRunning = errors.New("no synctoceph process is answering on the control socket")

// SendControl sends one request to the process running for the state folder
// dir and returns its answer.
func SendControl(dir string, req Request) (Response, error) {
	path := filepath.Join(dir, ControlName)
	var conn net.Conn
	err := withShortPath(dir, func(short string) (err error) {
		conn, err = net.DialTimeout("unix", short, controlTimeout)
		return err
	})
	if err != nil {
		return Response{}, ErrNotRunning
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(controlTimeout))
	data, _ := json.Marshal(req)
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return Response{}, fmt.Errorf("sending to %s: %w", path, err)
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return Response{}, fmt.Errorf("reading answer from %s: %w", path, err)
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, fmt.Errorf("reading answer from %s: %w", path, err)
	}
	return resp, nil
}
