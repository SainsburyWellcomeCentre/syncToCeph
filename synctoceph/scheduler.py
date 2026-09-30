"""One controller per state directory; no overlapping runs or catch-up bursts."""

from datetime import datetime, timedelta, timezone
import os
from pathlib import Path
import signal
import time
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

from .config import Settings, SyncError, daily_time
from .engine import Control, sync
from .state import Lock, Store, identity, timestamp


def local_zone():
    configured = os.environ.get("TZ")
    if configured:
        try:
            return ZoneInfo(configured.removeprefix(":"))
        except (ZoneInfoNotFoundError, ValueError) as exc:
            raise SyncError("TZ must be an IANA timezone name, e.g. Europe/London") from exc
    with Path("/etc/localtime").open("rb") as handle:
        return ZoneInfo.from_file(handle, key="system local timezone")


def next_daily(now: datetime, at: str, zone, last_date: str | None = None) -> datetime:
    """Return the next UTC minute matching the local time after last_date."""
    hour, minute = daily_time(at)
    cursor = now.astimezone(timezone.utc).replace(second=0, microsecond=0) + timedelta(minutes=1)
    for _ in range(3 * 24 * 60):
        local = cursor.astimezone(zone)
        if (local.hour, local.minute) == (hour, minute) and (
            last_date is None or local.date().isoformat() > last_date
        ):
            return cursor
        cursor += timedelta(minutes=1)
    raise SyncError("could not find the next daily time")


def run_controller(settings: Settings, *, interval: float | None = None,
                   at: str | None = None, immediate: bool = False,
                   ready_fd: int | None = None) -> int:
    settings.validate_layout()
    store = Store(settings.state_dir)
    store.prepare()
    scheduled = interval is not None or at is not None
    control = Control()
    old_handlers = {}
    with Lock(settings.state_dir / "job.lock") as job_lock:
        logger = store.logger(console=ready_fd is None)
        previous = store.read()
        if (previous.get("source"), previous.get("dest")) != (str(settings.source), str(settings.dest)):
            previous = {}
        process = identity(os.getpid())
        if process is None:
            raise SyncError("Linux /proc process identity is required")
        zone = local_zone() if at else None
        # Validate mounts per run so scheduled jobs can recover when a mount returns.
        state = {
            "schema_version": 1, "process": process, "mode": "schedule" if scheduled else "run",
            "state": "starting", "started_at": timestamp(),
            "source": str(settings.source), "dest": str(settings.dest),
            "dry_run": settings.dry_run,
            "schedule": {"interval_seconds": interval, "at": at, "timezone": str(zone) if zone else None},
            "last_run": previous.get("last_run"),
            "last_success_at": previous.get("last_success_at"),
            "recent_runs": previous.get("recent_runs", [])[-19:],
            "total_transferred_bytes": previous.get("total_transferred_bytes", 0),
            "last_daily_date": previous.get("last_daily_date") if previous.get("schedule", {}).get("at") == at else None,
        }
        for signum in (signal.SIGINT, signal.SIGTERM):
            old_handlers[signum] = signal.signal(signum, control.request_stop)

        def publish(phase):
            state["state"] = phase
            store.write(state)

        try:
            publish("waiting" if scheduled and not immediate else "starting")
            if ready_fd is not None:
                os.write(ready_fd, b"ready\n")
                os.close(ready_fd)
                ready_fd = None
            logger.info("Controller started (pid=%s, mode=%s)", os.getpid(), state["mode"])
            first = True
            while not control.stopping:
                if scheduled and not (first and immediate):
                    if interval is not None:
                        deadline = time.monotonic() + interval
                        state["next_run_at"] = datetime.fromtimestamp(time.time() + interval, timezone.utc).isoformat()
                    else:
                        due = next_daily(datetime.now(timezone.utc), at, zone, state["last_daily_date"])
                        state["next_run_at"] = due.isoformat()
                    publish("waiting")
                    while not control.stopping:
                        remaining = deadline - time.monotonic() if interval is not None else due.timestamp() - time.time()
                        if remaining <= 0:
                            break
                        time.sleep(min(0.2, remaining))
                if control.stopping:
                    break
                first = False
                state["next_run_at"] = None
                if at:
                    state["last_daily_date"] = datetime.now(zone).date().isoformat()
                publish("scanning")
                result = sync(settings, control, logger, job_lock.fd, publish)
                state["last_run"] = result
                state["recent_runs"] = (state["recent_runs"] + [result])[-20:]
                if not settings.dry_run:
                    state["total_transferred_bytes"] += result["transferred_bytes"]
                if result["verified"]:
                    state["last_success_at"] = result["finished_at"]
                store.write(state)
                if not scheduled:
                    return result["exit_code"]
            return 0
        finally:
            state["state"] = "stopped"
            state["stopped_at"] = timestamp()
            state["next_run_at"] = None
            try:
                store.write(state)
                logger.info("Controller stopped")
            finally:
                for signum, handler in old_handlers.items():
                    signal.signal(signum, handler)
                if ready_fd is not None:
                    os.close(ready_fd)
