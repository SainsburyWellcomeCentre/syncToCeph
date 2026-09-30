"""Strict TOML configuration and path/schedule parsing."""

from dataclasses import dataclass
import json
import os
from pathlib import Path
import re
import stat
import tempfile
import tomllib

DEFAULT_SOURCE = "/mnt/d/luminoseData"
DEFAULT_DEST = "/mnt/ceph/LuminoseFM/LuminoseDataCeph"
DEFAULT_CONFIG = "syncToCeph.toml"
CONFIG_KEYS = ("source", "dest", "state_dir", "require_mount", "interval", "at", "dry_run")


class SyncError(Exception):
    """An actionable operational error."""


def interval_seconds(value: str) -> float:
    match = re.fullmatch(r"([1-9][0-9]*)(s|m|h|d)", value)
    if not match:
        raise ValueError("interval must be a positive integer followed by s, m, h, or d")
    return int(match[1]) * {"s": 1, "m": 60, "h": 3600, "d": 86400}[match[2]]


def daily_time(value: str) -> tuple[int, int]:
    if not re.fullmatch(r"([01][0-9]|2[0-3]):[0-5][0-9]", value):
        raise ValueError("daily time must use 24-hour HH:MM format")
    hour, minute = value.split(":")
    return int(hour), int(minute)


def absolute(value: str | Path) -> Path:
    # Do not resolve symlinks: validation must be able to detect them.
    return Path(os.path.abspath(Path(value).expanduser()))


def config_path(explicit: str | None) -> Path | None:
    if explicit is not None:
        return absolute(explicit)
    local = absolute(DEFAULT_CONFIG)
    return local if local.exists() or local.is_symlink() else None


def load_config(path: str | Path | None) -> dict:
    if not path:
        return {}
    try:
        with open(path, "rb") as handle:
            data = tomllib.load(handle)
    except (OSError, tomllib.TOMLDecodeError) as exc:
        raise SyncError(f"cannot read config {path}: {exc}") from exc
    validate_config(data)
    return data


def validate_config(data: dict) -> None:
    if unknown := data.keys() - set(CONFIG_KEYS):
        raise SyncError(f"unknown config keys: {', '.join(sorted(unknown))}")
    for key, value in data.items():
        expected = bool if key == "dry_run" else str
        if not isinstance(value, expected):
            raise SyncError(f"config {key} must be {expected.__name__}")
        if key in ("source", "dest", "state_dir") and not value:
            raise SyncError(f"config {key} must not be empty")
    if "interval" in data and "at" in data:
        raise SyncError("config must specify either interval or at, not both")
    if "interval" in data:
        interval_seconds(data["interval"])
    if "at" in data:
        daily_time(data["at"])


def state_directory(config: dict, overrides: dict) -> Path:
    return absolute(overrides.get("state_dir") or config.get("state_dir")
                    or os.environ.get("SYNCTOCEPH_STATE_DIR", ".synctoceph-state"))


def resolve_settings(config: dict, overrides: dict) -> "Settings":
    source = absolute(overrides.get("source") or config.get("source", DEFAULT_SOURCE))
    dest = absolute(overrides.get("dest") or config.get("dest", DEFAULT_DEST))
    mount_value = overrides.get("require_mount")
    if mount_value is None:
        mount_value = config.get("require_mount")
    mount = absolute(mount_value) if mount_value else Path("/mnt/ceph") if dest == Path(DEFAULT_DEST) else None
    dry_run = overrides.get("dry_run")
    return Settings(source, dest, state_directory(config, overrides), mount,
                    config.get("dry_run", False) if dry_run is None else dry_run)


def write_config(path: Path, data: dict, *, create: bool = False) -> None:
    """Create exclusively or atomically replace a validated TOML config."""
    validate_config(data)
    lines = ["# Directory defaults for syncToCeph.",
             "# Relative paths resolve from the working directory; absolute paths are recommended.",
             '# require_mount = "" selects the automatic guard: /mnt/ceph for the built-in destination, none otherwise.',
             "# Set at most one schedule: interval or at."]
    for key in CONFIG_KEYS:
        if key in data:
            # JSON basic strings are also TOML strings when Unicode is unescaped.
            encoded = json.dumps(data[key], ensure_ascii=False).replace("\x7f", "\\u007f")
            lines.append(f"{key} = {encoded}")
    content = "\n".join(lines) + "\n"
    if create:
        try:
            with path.open("x", encoding="utf-8") as handle:
                handle.write(content)
        except FileExistsError as exc:
            raise SyncError(f"config already exists: {path}; use config set or edit the file") from exc
        return
    temporary = None
    mode = stat.S_IMODE(path.stat().st_mode)
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=path.parent,
                                         prefix=f".{path.name}.", delete=False) as handle:
            temporary = Path(handle.name)
            os.fchmod(handle.fileno(), mode)
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


@dataclass(frozen=True)
class Settings:
    source: Path
    dest: Path
    state_dir: Path
    require_mount: Path | None = None
    dry_run: bool = False

    def validate_layout(self, *, resolve_paths: bool = True) -> None:
        paths = [self.source, self.dest, self.state_dir]
        if resolve_paths:
            paths = [path.resolve() for path in paths]
        for i, first in enumerate(paths):
            for second in paths[i + 1:]:
                if first == second or first in second.parents or second in first.parents:
                    raise SyncError("source, destination, and state directory must be separate, non-nested paths")
