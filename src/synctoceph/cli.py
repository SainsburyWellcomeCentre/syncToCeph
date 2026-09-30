"""Command line interface; no shell interpolation or arbitrary rsync options."""

import argparse
import json
import math
import os
from pathlib import Path
import selectors
import subprocess
import sys
import time

from . import __version__
from .config import (CONFIG_KEYS, DEFAULT_CONFIG, DEFAULT_DEST, DEFAULT_SOURCE, Settings,
                     SyncError, absolute, config_path, daily_time, interval_seconds,
                     load_config, resolve_settings, state_directory, write_config)
from .scheduler import run_controller
from .state import Lock, Store, alive, lock_held, no_symlinks, signal_process


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(prog="syncToCeph", description="Additive, SHA-256 verified archives powered by rsync.")
    root.add_argument("--version", action="version", version=f"%(prog)s {__version__}")
    # Shared arguments can appear before or after the subcommand.
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--config", default=argparse.SUPPRESS, help=f"TOML file (default: ./{DEFAULT_CONFIG} if present)")
    common.add_argument("--state-dir", default=argparse.SUPPRESS, help="state/log directory (default: ./.synctoceph-state)")
    root.add_argument("--config", default=argparse.SUPPRESS, help=f"TOML file (default: ./{DEFAULT_CONFIG} if present)")
    root.add_argument("--state-dir", default=argparse.SUPPRESS, help="state/log directory")
    commands = root.add_subparsers(dest="command", required=True)
    for name in ("run", "schedule"):
        command = commands.add_parser(name, parents=[common], help="sync once" if name == "run" else "run on a schedule")
        command.add_argument("--source", help=f"override configured source (built-in default: {DEFAULT_SOURCE})")
        command.add_argument("--dest", help=f"override configured destination (built-in default: {DEFAULT_DEST})")
        command.add_argument("--require-mount", help="refuse transfers unless this destination ancestor is a mount point")
        command.add_argument("--dry-run", action="store_true", default=None, help="preview only; do not write to the destination")
        if name == "schedule":
            timing = command.add_mutually_exclusive_group()
            timing.add_argument("--interval", "--every", type=interval_seconds, help="delay between completed runs, e.g. 4h or 30m")
            timing.add_argument("--at", type=lambda value: (daily_time(value), value)[1], help="daily local time HH:MM")
            command.add_argument("--immediate", action="store_true", help="also run immediately on startup")
            command.add_argument("--background", action="store_true", help="detach and acknowledge successful startup")
            command.add_argument("--_ready-fd", type=int, help=argparse.SUPPRESS)
    status = commands.add_parser("status", parents=[common], help="show process and transfer status")
    status.add_argument("--json", action="store_true", help="machine-readable status")
    stop = commands.add_parser("stop", parents=[common], help="request graceful shutdown and wait")
    stop.add_argument("--timeout", type=float, default=45, help="seconds to wait (default: 45)")
    logs = commands.add_parser("logs", parents=[common], help="show recent rotated logs")
    logs.add_argument("--lines", "-n", type=int, default=100, help="number of recent lines (default: 100)")
    config = commands.add_parser("config", parents=[common], help="view or change saved defaults")
    actions = config.add_subparsers(dest="config_action", required=True)
    for name in ("show", "init"):
        action = actions.add_parser(name, parents=[common], help="show effective settings and derived paths" if name == "show"
                                   else "create a defaults file without overwriting an existing file")
        action.add_argument("--source", help="source directory")
        action.add_argument("--dest", help="destination directory")
        action.add_argument("--require-mount", help="required destination mount point (empty string: automatic)")
        if name == "show":
            action.add_argument("--json", action="store_true", help="machine-readable settings")
    update = actions.add_parser("set", parents=[common], help="save one setting in an existing config file")
    update.add_argument("key", choices=CONFIG_KEYS)
    update.add_argument("value", help="new value; quote paths containing spaces; dry_run accepts true or false")
    return root


def configure(args: argparse.Namespace, path: Path | None, config: dict) -> int:
    target = path or absolute(DEFAULT_CONFIG)
    if args.config_action == "set":
        if path is None:
            raise SyncError("no config file found; create one with config init first")
        value = args.value
        if args.key == "dry_run":
            if value not in ("true", "false"):
                raise SyncError("dry_run must be true or false")
            value = value == "true"
        elif args.key in ("source", "dest", "state_dir", "require_mount") and value:
            value = str(absolute(value))
        updated = dict(config, **{args.key: value})
        if args.key in ("interval", "at"):
            updated.pop("at" if args.key == "interval" else "interval", None)
        # Saving defaults must work even while data mounts are unavailable.
        resolve_settings(updated, {}).validate_layout(resolve_paths=False)
        no_symlinks(target)
        write_config(target, updated)
        print(f"Saved {args.key} in {target}")
        print("Changes apply to new commands; restart a running scheduler to use them.")
        return 0

    settings = resolve_settings(config, vars(args))
    if args.config_action == "init":
        settings.validate_layout(resolve_paths=False)
        data = dict(config, source=str(settings.source), dest=str(settings.dest),
                    state_dir=str(settings.state_dir), dry_run=settings.dry_run,
                    require_mount=args.require_mount or "")
        if data["require_mount"]:
            data["require_mount"] = str(absolute(data["require_mount"]))
        no_symlinks(target)
        write_config(target, data, create=True)
        print(f"Created {target}")
        print("View with config show; change defaults with config set KEY VALUE or edit this file.")
        return 0

    result = {
        "config_file": str(path) if path else None,
        "source": str(settings.source), "dest": str(settings.dest),
        "state_dir": str(settings.state_dir),
        "require_mount": str(settings.require_mount) if settings.require_mount else None,
        "dry_run": settings.dry_run,
        "interval": config.get("interval"), "at": config.get("at"),
        "log_file": str(settings.state_dir / "sync.log"),
        "status_file": str(settings.state_dir / "status.json"),
        "startup_log": str(settings.state_dir / "startup.log"),
        "history_dir": str(settings.dest / ".syncToCeph" / "history"),
        "partial_dirs": str(settings.dest / "<relative-directory>" / ".syncToCeph-partial"),
    }
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(f"Config file: {path if path else f'{target} (not found; using defaults)'}")
        for key in ("source", "dest", "state_dir", "require_mount", "dry_run"):
            origin = "CLI" if getattr(args, key, None) is not None else "config" if key in config else (
                "SYNCTOCEPH_STATE_DIR" if key == "state_dir" and "SYNCTOCEPH_STATE_DIR" in os.environ else "default")
            print(f"{key}: {result[key] if result[key] is not None else 'none'} ({origin})")
        print(f"Schedule: {config.get('interval') or config.get('at') or 'not configured'}")
        print("Derived paths (follow state_dir or dest):")
        for key in ("log_file", "status_file", "startup_log", "history_dir", "partial_dirs"):
            print(f"  {key}: {result[key]}")
        print("These settings apply to new commands; running schedulers keep their startup settings.")
    return 0


def background(settings: Settings, interval: float | None, at: str | None, immediate: bool) -> int:
    store = Store(settings.state_dir)
    store.prepare()
    # Reject an occupied state directory before spawning. The child acquires
    # the lifetime lock independently to handle concurrent launch attempts.
    with Lock(settings.state_dir / "job.lock"):
        pass
    read_fd, write_fd = os.pipe()
    bootstrap = "import sys; sys.path.insert(0, sys.argv.pop(1)); from synctoceph.cli import main; sys.exit(main())"
    args = [sys.executable, "-c", bootstrap, str(Path(__file__).resolve().parent.parent),
            # The parent already resolved config; do not load a different local file in the child.
            "--config", os.devnull, "schedule", "--source", str(settings.source), "--dest", str(settings.dest),
            "--state-dir", str(settings.state_dir), "--_ready-fd", str(write_fd)]
    if interval is not None:
        args.extend(["--interval", f"{int(interval)}s"])
    else:
        args.extend(["--at", at])
    if settings.require_mount:
        args.extend(["--require-mount", str(settings.require_mount)])
    if settings.dry_run:
        args.append("--dry-run")
    if immediate:
        args.append("--immediate")
    # Capture startup errors before the child initializes its rotating log.
    diagnostic = settings.state_dir / "startup.log"
    no_symlinks(diagnostic)
    try:
        with diagnostic.open("w") as output:
            process = subprocess.Popen(args, stdin=subprocess.DEVNULL, stdout=output,
                                       stderr=subprocess.STDOUT, start_new_session=True,
                                       pass_fds=(write_fd,))
        os.close(write_fd)
        write_fd = -1
        with selectors.DefaultSelector() as selector:
            selector.register(read_fd, selectors.EVENT_READ)
            ready = selector.select(timeout=15)
            message = os.read(read_fd, 32) if ready else b""
        if message != b"ready\n":
            process.terminate()
            try:
                process.wait(timeout=35)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            raise SyncError(f"background startup failed; see {diagnostic}")
        print(f"Scheduler started (PID {process.pid}); state: {settings.state_dir}")
        return 0
    finally:
        os.close(read_fd)
        if write_fd >= 0:
            os.close(write_fd)


def show_status(store: Store, as_json: bool) -> int:
    state = store.read()
    running = alive(state.get("process"))
    busy = lock_held(store.directory / "job.lock")
    result = dict(state, running=running, job_lock_held=busy)
    if not running:
        result["state"] = "orphaned" if busy else "stopped" if state.get("state") == "stopped" else "stale" if state else "idle"
    if as_json:
        print(json.dumps(result, indent=2))
        return 0
    last = state.get("last_run") or {}
    print(f"State: {result.get('state', 'idle')}")
    print(f"Running: {'yes' if running else 'no'}")
    print(f"Job lock held: {'yes' if busy else 'no'}")
    print(f"PID: {state.get('process', {}).get('pid', '-')}")
    print(f"Last sync: {last.get('finished_at', '-')}")
    print(f"Last verified success: {state.get('last_success_at') or '-'}")
    print(f"Last transferred bytes: {last.get('transferred_bytes', 0)}" + (" (dry-run estimate)" if last.get("dry_run") else ""))
    print(f"Last verified bytes: {last.get('verified_bytes', 0)}")
    print(f"Total transferred bytes: {state.get('total_transferred_bytes', 0)}")
    print(f"Recent exit codes: {[entry['exit_code'] for entry in state.get('recent_runs', [])]}")
    print(f"Next run: {state.get('next_run_at') or '-'}")
    if last.get("error"):
        print(f"Last error: {last['error']}")
    return 0


def stop(store: Store, timeout: float) -> int:
    if timeout < 0 or not math.isfinite(timeout):
        raise SyncError("timeout must be a finite nonnegative number")
    state = store.read()
    process = state.get("process")
    if not process or not alive(process) or not signal_process(process):
        if lock_held(store.directory / "job.lock"):
            raise SyncError("controller is gone but the job lock is held; an rsync child may still be finishing")
        print("No running controller for this state directory.")
        return 0
    deadline = time.monotonic() + timeout
    while alive(process):
        if time.monotonic() >= deadline:
            raise SyncError("shutdown requested but still in progress; check status/logs")
        time.sleep(0.1)
    print("Stopped.")
    return 0


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        path = config_path(getattr(args, "config", None))
        creating = args.command == "config" and args.config_action == "init"
        config = {} if creating else load_config(path)
        if args.command == "config":
            return configure(args, path, config)
        directory = state_directory(config, vars(args))
        store = Store(directory)
        if args.command == "status":
            return show_status(store, args.json)
        if args.command == "logs":
            if args.lines < 1:
                raise SyncError("--lines must be positive")
            print(store.tail(args.lines), end="")
            return 0
        if args.command == "stop":
            return stop(store, args.timeout)
        settings = resolve_settings(config, vars(args))
        settings.validate_layout()
        if args.command == "run":
            return run_controller(settings)
        interval, at = args.interval, args.at
        if interval is None and at is None:
            interval = interval_seconds(config["interval"]) if "interval" in config else None
            at = config.get("at")
        if at:
            daily_time(at)
        if interval is None and at is None:
            raise SyncError("schedule requires --interval/--every or --at (or a TOML schedule)")
        if args.background:
            return background(settings, interval, at, args.immediate)
        return run_controller(settings, interval=interval, at=at, immediate=args.immediate,
                              ready_fd=args._ready_fd)
    except (SyncError, OSError, ValueError) as exc:
        print(f"syncToCeph: {exc}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
