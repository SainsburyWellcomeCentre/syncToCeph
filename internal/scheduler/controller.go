// This file is the controller: the part that owns a profile while synctoceph
// is working on it. It takes the state lock (so only one process works on a
// profile at a time), opens the log, answers `stop` and `status` requests on
// the control socket, and keeps status.json up to date.
package scheduler

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// Modes a controller runs in.
const (
	ModeRun      = "run"      // one run, then exit
	ModeSchedule = "schedule" // runs on the schedule until stopped
	ModeVerify   = "verify"   // re-verify copied files
)

// Controller owns one profile's state folder while synctoceph works on it.
type Controller struct {
	Settings config.Settings
	Version  string
	Mode     string
	Log      *state.Logger
	// Report, if set, is told about each phase of a run (for the terminal).
	Report func(phase, detail string)
	// Event, if set, is told about each file copied, verified or deferred,
	// and about the scan and plan results (for the terminal).
	Event func(engine.Event)
	// Verbose, if set, receives rsync's messages (warnings and errors).
	Verbose func(line string)
	// AfterRun, if set, is called with each finished run (schedule mode).
	AfterRun func(destination.RunSummary)
	// Waiting, if set, is told when the scheduler starts waiting.
	Waiting func(line string)

	lock   *state.Lock
	server *state.ControlServer
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	status state.Status
	live   state.Live
	grace  time.Duration
}

// Open takes the profile's state lock and prepares the controller. It fails
// with an explanation if another synctoceph process holds the lock.
func Open(s config.Settings, mode, version string) (*Controller, error) {
	// SAFETY: invariant 7: check separation before the state folder is created, so it
	// is never created inside the source or the destination.
	if err := engine.CheckSeparate(s); err != nil {
		return nil, err
	}
	if err := state.Prepare(s.StateDir); err != nil {
		return nil, ui.StateProblem(s.StateDir, err)
	}
	lockPath := s.StateDir + "/" + state.LockName
	// SAFETY: invariant 10 (one run per profile at a time).
	lock, err := state.Acquire(lockPath)
	if errors.Is(err, state.ErrLocked) {
		st, _ := state.ReadStatus(s.StateDir, s.Profile)
		return nil, ui.AlreadyRunning(s.Profile, st.Mode, st.PID)
	}
	if err != nil {
		return nil, ui.StateUnusable(s.StateDir, err)
	}
	log, err := state.OpenLog(s.StateDir)
	if err != nil {
		lock.Release()
		return nil, ui.StateUnusable(s.StateDir, err)
	}
	st, err := state.ReadStatus(s.StateDir, s.Profile)
	if err != nil {
		log.Warn("Starting with a fresh status: %v", err)
		st = state.Status{Profile: s.Profile}
	}
	c := &Controller{Settings: s, Version: version, Mode: mode, Log: log, lock: lock,
		status: st, grace: engine.KillGrace}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.live = state.Live{PID: os.Getpid(), Mode: mode, Phase: "starting", StartedAt: time.Now().UTC()}
	c.status.Mode, c.status.PID, c.status.NextRunAt = mode, os.Getpid(), nil
	c.status.Schedule = describeSchedule(s)
	if c.server, err = state.ServeControl(s.StateDir, c.handle); err != nil {
		log.Warn("%s", ui.NoControlSocket(err))
	}
	log.Info("Controller started (mode=%s, pid=%d, version=%s)", mode, os.Getpid(), version)
	return c, nil
}

// Close records the final state and releases the lock.
func (c *Controller) Close() {
	c.mu.Lock()
	if c.Mode == ModeSchedule {
		c.status.State = state.StateStopped
	} else {
		c.status.State = state.StateIdle
	}
	c.status.Phase, c.status.NextRunAt, c.status.PID = "", nil, 0
	c.writeStatusLocked()
	c.mu.Unlock()
	c.server.Close()
	c.Log.SetRun("")
	c.Log.Info("Controller stopped")
	c.Log.Close()
	c.cancel()
	c.lock.Release()
}

// Context is cancelled when a stop is requested.
func (c *Controller) Context() context.Context { return c.ctx }

// LockFile is the open state lock file, which child processes inherit.
func (c *Controller) LockFile() *os.File { return c.lock.File }

// RequestStop asks the current work to stop. rsync gets grace to exit after
// SIGINT before it is killed.
func (c *Controller) RequestStop(grace time.Duration) {
	c.mu.Lock()
	if grace > 0 {
		c.grace = grace
	}
	c.live.Stopping = true
	c.mu.Unlock()
	c.Log.Info("Stop requested (rsync grace period %s)", grace)
	c.cancel()
}

// handle answers a request from the control socket.
func (c *Controller) handle(req state.Request) state.Response {
	switch req.Command {
	case "stop":
		c.RequestStop(time.Duration(req.GraceSeconds) * time.Second)
		return state.Response{OK: true, Message: "stopping"}
	case "status":
		c.mu.Lock()
		live := c.live
		c.mu.Unlock()
		return state.Response{OK: true, Live: &live}
	}
	return state.Response{OK: false, Message: "unknown command " + req.Command}
}

// graceNow returns the current rsync grace period.
func (c *Controller) graceNow() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.grace
}

// setPhase updates the live phase, status.json and the terminal.
func (c *Controller) setPhase(phase, detail string) {
	c.mu.Lock()
	changed := c.live.Phase != phase
	c.live.Phase, c.live.Progress = phase, detail
	c.status.Phase = phase
	if changed {
		c.writeStatusLocked()
	}
	c.mu.Unlock()
	if c.Report != nil {
		c.Report(phase, detail)
	}
}

// writeStatusLocked saves status.json; c.mu must be held.
func (c *Controller) writeStatusLocked() {
	if err := state.WriteStatus(c.Settings.StateDir, &c.status); err != nil {
		c.Log.Error("Could not save status: %v", err)
	}
}

// ScheduleText returns the schedule in words, e.g. "every 4h".
func (c *Controller) ScheduleText() string { return c.status.Schedule }

// describeSchedule returns the schedule in words, e.g. "every 4h".
func describeSchedule(s config.Settings) string {
	switch {
	case s.Interval > 0:
		return "every " + config.FormatDuration(s.Interval)
	case s.At != "":
		return "daily at " + s.At + " (" + ZoneName() + ")"
	}
	return ""
}
