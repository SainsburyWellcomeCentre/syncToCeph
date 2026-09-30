"""Additive transfers, stable-source checks and independent SHA-256 verification."""

from contextlib import nullcontext
import hashlib
import logging
import os
from pathlib import Path
import re
import selectors
import shutil
import signal
import stat
import subprocess
import time
import uuid

from .config import Settings, SyncError
from .state import Lock, no_symlinks, timestamp

META = ".syncToCeph"
PARTIAL = ".syncToCeph-partial"


class Cancelled(SyncError):
    pass


class Control:
    def __init__(self):
        self.stopping = False

    def request_stop(self, *_):
        self.stopping = True

    def check(self):
        if self.stopping:
            raise Cancelled("stop requested; this run is not verified")


def signature(info: os.stat_result) -> tuple:
    return (info.st_mode, info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def snapshot(root: Path, control: Control) -> dict[str, tuple]:
    result = {}
    pending = [root]
    while pending:
        control.check()
        directory = pending.pop()
        with os.scandir(directory) as entries:
            for entry in entries:
                control.check()
                if entry.name in {META, PARTIAL}:
                    raise SyncError(f"source uses a reserved name: {entry.path}")
                info = entry.stat(follow_symlinks=False)
                if not (stat.S_ISREG(info.st_mode) or stat.S_ISDIR(info.st_mode)):
                    raise SyncError(f"only regular files and directories are supported: {entry.path}")
                result[str(Path(entry.path).relative_to(root))] = signature(info)
                if stat.S_ISDIR(info.st_mode):
                    pending.append(Path(entry.path))
    return result


def preflight(settings: Settings) -> str:
    settings.validate_layout()
    for path in (settings.source, settings.dest):
        no_symlinks(path)
        if not path.is_dir():
            raise SyncError(f"directory does not exist (it will not be created): {path}")
    if settings.require_mount:
        mount = settings.require_mount
        no_symlinks(mount)
        if mount != settings.dest and mount not in settings.dest.parents:
            raise SyncError("required mount must contain the destination")
        if not os.path.ismount(mount):
            raise SyncError(f"required destination mount is not mounted: {mount}")
    executable = shutil.which("rsync")
    if not executable:
        raise SyncError("rsync is missing; install rsync 3.2 or newer")
    return executable


def validate_destination(settings: Settings, manifest: dict[str, tuple]):
    no_symlinks(settings.dest / META)
    no_symlinks(settings.dest / PARTIAL)
    for name, source_signature in manifest.items():
        target = settings.dest / name
        no_symlinks(target)
        if stat.S_ISDIR(source_signature[0]):
            no_symlinks(target / PARTIAL)
        try:
            info = target.lstat()
        except FileNotFoundError:
            continue
        if stat.S_IFMT(info.st_mode) != stat.S_IFMT(source_signature[0]):
            raise SyncError(f"file/directory type conflict at destination: {target}")


def rsync_command(executable: str, settings: Settings, run_id: str) -> list[str]:
    command = [
        executable, "--recursive", "--times", "--omit-dir-times", "--checksum",
        "--no-whole-file", "--partial-dir=" + PARTIAL,
        "--backup", "--backup-dir=" + str(settings.dest / META / "history" / run_id),
        "--fsync", "--stats", "--itemize-changes", "--out-format=%i %n%L",
        "--exclude=" + META + "/", "--exclude=" + PARTIAL + "/",
    ]
    if settings.dry_run:
        command.append("--dry-run")
    command.extend(["--", str(settings.source) + "/", str(settings.dest) + "/"])
    return command


def execute(command: list[str], control: Control, logger: logging.Logger,
            lock_fds: tuple[int, ...]) -> tuple[int, int]:
    env = os.environ.copy()
    env["LC_ALL"] = "C"
    # Do not allow a caller's checksum negotiation override to disable rsync checks.
    env.pop("RSYNC_CHECKSUM_LIST", None)
    transferred = 0
    pending = b""

    def emit(raw: bytes):
        nonlocal transferred
        line = raw.decode("utf-8", errors="replace").rstrip()
        if line:
            logger.info("rsync: %s", line)
        match = re.fullmatch(r"Total transferred file size: ([0-9,]+) bytes", line)
        if match:
            transferred = int(match[1].replace(",", ""))

    control.check()
    with subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                          stdin=subprocess.DEVNULL, start_new_session=True,
                          env=env, pass_fds=lock_fds) as process:
        assert process.stdout is not None
        sent_at = None
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            try:
                while selector.get_map() or process.poll() is None:
                    if control.stopping and sent_at is None:
                        logger.info("Stopping rsync cleanly; keeping resumable partial data")
                        try:
                            os.killpg(process.pid, signal.SIGINT)
                        except ProcessLookupError:
                            pass
                        sent_at = time.monotonic()
                    if sent_at is not None and time.monotonic() - sent_at > 30:
                        try:
                            os.killpg(process.pid, signal.SIGKILL)
                        except ProcessLookupError:
                            pass
                    for key, _ in selector.select(timeout=0.2):
                        chunk = os.read(key.fd, 65536)
                        if not chunk:
                            selector.unregister(key.fileobj)
                            continue
                        pending += chunk
                        while b"\n" in pending:
                            line, pending = pending.split(b"\n", 1)
                            emit(line)
                        if len(pending) > 65536:
                            emit(pending)
                            pending = b""
                if pending:
                    emit(pending)
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
        return process.wait(), transferred


def digest(path: Path, control: Control, expected: tuple | None = None) -> tuple[str, int]:
    no_symlinks(path)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as handle:
        before = os.fstat(handle.fileno())
        if not stat.S_ISREG(before.st_mode):
            raise SyncError(f"not a regular file during verification: {path}")
        if expected and signature(before) != expected:
            raise SyncError(f"source changed during sync: {path}")
        checksum = hashlib.sha256()
        while True:
            control.check()
            chunk = handle.read(1024 * 1024)
            if not chunk:
                break
            checksum.update(chunk)
        if signature(os.fstat(handle.fileno())) != signature(before):
            raise SyncError(f"file changed while being verified: {path}")
        if signature(path.lstat()) != signature(before):
            raise SyncError(f"file was replaced while being verified: {path}")
        return checksum.hexdigest(), before.st_size


def verify(settings: Settings, manifest: dict[str, tuple], control: Control) -> tuple[int, int]:
    files = volume = 0
    for name, expected in manifest.items():
        control.check()
        if stat.S_ISDIR(expected[0]):
            target = settings.dest / name
            no_symlinks(target)
            if not target.is_dir():
                raise SyncError(f"missing destination directory: {target}")
            continue
        source_hash, size = digest(settings.source / name, control, expected)
        target_hash, target_size = digest(settings.dest / name, control)
        if (source_hash, size) != (target_hash, target_size):
            raise SyncError(f"SHA-256 verification failed: {name}")
        files += 1
        volume += size
    if snapshot(settings.source, control) != manifest:
        raise SyncError("source tree changed during sync; retry with a stable source")
    return files, volume


def sync(settings: Settings, control: Control, logger: logging.Logger,
         job_lock_fd: int, phase_callback=None) -> dict:
    run_id = uuid.uuid4().hex
    result = {"run_id": run_id, "started_at": timestamp(), "dry_run": settings.dry_run,
              "exit_code": 1, "rsync_exit_code": None, "verified": False,
              "transferred_bytes": 0, "verified_bytes": 0, "verified_files": 0}
    try:
        executable = preflight(settings)
        metadata = settings.dest / META
        no_symlinks(metadata)
        if not settings.dry_run:
            metadata.mkdir(exist_ok=True, mode=0o700)
        archive_lock = nullcontext() if settings.dry_run else Lock(metadata / "archive.lock")
        with archive_lock as held:
            control.check()
            manifest = snapshot(settings.source, control)
            validate_destination(settings, manifest)
            if not settings.dry_run:
                history = metadata / "history" / run_id
                no_symlinks(history)
                history.mkdir(parents=True, mode=0o700)
            logger.info("Run %s: %s -> %s (dry_run=%s)", run_id, settings.source,
                        settings.dest, settings.dry_run)
            if phase_callback:
                phase_callback("transferring")
            fds = (job_lock_fd,) if held is None else (job_lock_fd, held.fd)
            code, volume = execute(rsync_command(executable, settings, run_id), control, logger, fds)
            result.update(rsync_exit_code=code, transferred_bytes=volume)
            control.check()
            if code != 0:
                result["exit_code"] = code if code > 0 else 1
                raise SyncError(f"rsync failed with exit code {code}; run is not verified")
            if not settings.dry_run:
                if phase_callback:
                    phase_callback("verifying")
                logger.info("Verifying source and destination contents with SHA-256")
                files, size = verify(settings, manifest, control)
                result.update(verified=True, verified_files=files, verified_bytes=size,
                              history_dir=str(history))
            result["exit_code"] = 0
    except Cancelled as exc:
        result.update(exit_code=130, error=str(exc))
        logger.warning("%s", exc)
    except (SyncError, OSError) as exc:
        result["error"] = str(exc)
        logger.error("%s", exc)
    result["finished_at"] = timestamp()
    logger.info("Run %s finished: exit=%s verified=%s transferred_bytes=%s", run_id,
                result["exit_code"], result["verified"], result["transferred_bytes"])
    return result
