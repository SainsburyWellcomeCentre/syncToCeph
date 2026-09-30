"""Atomic status, bounded logs, advisory locks and safe Linux process identity."""

from collections import deque
from contextlib import AbstractContextManager
from datetime import datetime, timezone
import fcntl
import json
import logging
from logging.handlers import RotatingFileHandler
import os
from pathlib import Path
import signal
import stat
import uuid

from .config import SyncError


def timestamp() -> str:
    return datetime.now(timezone.utc).isoformat()


def no_symlinks(path: Path) -> None:
    for part in [*reversed(path.parents), path]:
        if part.is_symlink():
            raise SyncError(f"symlink paths are not supported: {part}")


class Lock(AbstractContextManager):
    def __init__(self, path: Path):
        self.path = path
        self.fd: int | None = None

    def __enter__(self):
        no_symlinks(self.path)
        self.fd = os.open(self.path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        try:
            if not stat.S_ISREG(os.fstat(self.fd).st_mode):
                raise SyncError(f"lock is not a regular file: {self.path}")
            fcntl.flock(self.fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            os.close(self.fd)
            self.fd = None
            raise SyncError(f"another task holds the lock: {self.path}") from exc
        except BaseException:
            os.close(self.fd)
            self.fd = None
            raise
        return self

    def __exit__(self, *args):
        if self.fd is not None:
            os.close(self.fd)
            self.fd = None


def identity(pid: int) -> dict | None:
    try:
        fields = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()
        if fields[0] == "Z":
            return None
        return {
            "pid": pid,
            "start_ticks": fields[19],
            "boot_id": Path("/proc/sys/kernel/random/boot_id").read_text().strip(),
        }
    except (OSError, IndexError):
        return None


def alive(process: dict | None) -> bool:
    return bool(process and identity(process.get("pid", -1)) == process)


def lock_held(path: Path) -> bool:
    """Check whether a process holds the lock without creating a lock file."""
    no_symlinks(path)
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    except FileNotFoundError:
        return False
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            return False
        except BlockingIOError:
            return True
    finally:
        os.close(fd)


def signal_process(process: dict, signum: int = signal.SIGTERM) -> bool:
    """Open a pidfd before checking identity to avoid signalling a reused PID."""
    if not hasattr(os, "pidfd_open") or not hasattr(signal, "pidfd_send_signal"):
        raise SyncError("safe stop requires Linux with pidfd support (kernel 5.3+)")
    try:
        fd = os.pidfd_open(process["pid"])
    except ProcessLookupError:
        return False
    try:
        if not alive(process):
            return False
        signal.pidfd_send_signal(fd, signum)
        return True
    except ProcessLookupError:
        return False
    finally:
        os.close(fd)


class Store:
    def __init__(self, directory: Path):
        self.directory = directory
        self.path = directory / "status.json"

    def prepare(self):
        no_symlinks(self.directory)
        self.directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        info = self.directory.stat()
        if info.st_uid != os.getuid() or info.st_mode & 0o022:
            raise SyncError("state directory must be owned by you and not writable by group/others")

    def read(self) -> dict:
        try:
            no_symlinks(self.path)
            result = json.loads(self.path.read_text())
            if not isinstance(result, dict):
                raise ValueError("expected an object")
            return result
        except FileNotFoundError:
            return {}
        except (ValueError, OSError) as exc:
            raise SyncError(f"cannot read state {self.path}: {exc}") from exc

    def write(self, data: dict):
        data["updated_at"] = timestamp()
        temporary = self.directory / f".status-{uuid.uuid4().hex}.tmp"
        try:
            with temporary.open("x", encoding="utf-8") as handle:
                json.dump(data, handle, indent=2)
                handle.write("\n")
                handle.flush()
                os.fsync(handle.fileno())
            os.replace(temporary, self.path)
            fd = os.open(self.directory, os.O_RDONLY | os.O_DIRECTORY)
            try:
                os.fsync(fd)
            finally:
                os.close(fd)
        finally:
            temporary.unlink(missing_ok=True)

    def logger(self, console: bool = True) -> logging.Logger:
        path = self.directory / "sync.log"
        no_symlinks(path)
        logger = logging.getLogger(f"synctoceph.{self.directory}")
        logger.setLevel(logging.INFO)
        logger.propagate = False
        for handler in logger.handlers[:]:
            handler.close()
            logger.removeHandler(handler)
        file_handler = RotatingFileHandler(path, maxBytes=5 * 1024 * 1024, backupCount=5, encoding="utf-8")
        handlers = [file_handler]
        if console:
            handlers.append(logging.StreamHandler())
        for handler in handlers:
            handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s %(message)s"))
            logger.addHandler(handler)
        return logger

    def tail(self, count: int) -> str:
        lines: deque[str] = deque(maxlen=count)
        for name in [*(f"sync.log.{i}" for i in range(5, 0, -1)), "sync.log"]:
            path = self.directory / name
            no_symlinks(path)
            try:
                with path.open(encoding="utf-8", errors="replace") as handle:
                    lines.extend(handle)
            except FileNotFoundError:
                pass
        return "".join(lines)
