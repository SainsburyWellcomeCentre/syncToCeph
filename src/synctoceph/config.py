"""Strict TOML configuration and path/schedule parsing."""

from dataclasses import dataclass
from pathlib import Path
import re
import tomllib

DEFAULT_SOURCE = "/mnt/d/luminoseData"
DEFAULT_DEST = "/mnt/ceph/LuminoseFM/LuminoseDataCeph"


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
    import os
    return Path(os.path.abspath(Path(value).expanduser()))


def load_config(path: str | None) -> dict:
    if not path:
        return {}
    try:
        with open(path, "rb") as handle:
            data = tomllib.load(handle)
    except (OSError, tomllib.TOMLDecodeError) as exc:
        raise SyncError(f"cannot read config {path}: {exc}") from exc
    allowed = {"source", "dest", "state_dir", "require_mount", "interval", "at", "dry_run"}
    if unknown := data.keys() - allowed:
        raise SyncError(f"unknown config keys: {', '.join(sorted(unknown))}")
    for key, value in data.items():
        expected = bool if key == "dry_run" else str
        if not isinstance(value, expected):
            raise SyncError(f"config {key} must be {expected.__name__}")
    if "interval" in data and "at" in data:
        raise SyncError("config must specify either interval or at, not both")
    return data


@dataclass(frozen=True)
class Settings:
    source: Path
    dest: Path
    state_dir: Path
    require_mount: Path | None = None
    dry_run: bool = False

    def validate_layout(self) -> None:
        paths = [self.source.resolve(), self.dest.resolve(), self.state_dir.resolve()]
        for i, first in enumerate(paths):
            for second in paths[i + 1:]:
                if first == second or first in second.parents or second in first.parents:
                    raise SyncError("source, destination, and state directory must be separate, non-nested paths")
