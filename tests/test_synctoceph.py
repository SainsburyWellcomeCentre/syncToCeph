"""Real-rsync integration tests; all temporary writes stay in the workspace."""

from datetime import datetime, timezone
import json
import logging
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch
from zoneinfo import ZoneInfo

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from synctoceph.config import Settings, SyncError, daily_time, interval_seconds, load_config
from synctoceph.engine import Control, execute, rsync_command, snapshot, sync, verify
from synctoceph.scheduler import next_daily
from synctoceph.state import Lock, Store, identity, signal_process

TEMP = ROOT / ".test-tmp"
TEMP.mkdir(exist_ok=True)


class WorkspaceTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(dir=TEMP)
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.dest = self.root / "archive"
        self.state = self.root / "state"
        self.source.mkdir()
        self.dest.mkdir()
        self.settings = Settings(self.source, self.dest, self.state)

    def cli(self, *args, expected=0):
        result = subprocess.run([sys.executable, str(ROOT / "syncToCeph"), "--state-dir", str(self.state), *args],
                                cwd=ROOT, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        return result

    def run_sync(self, *args, expected=0):
        return self.cli("run", "--source", str(self.source), "--dest", str(self.dest), *args, expected=expected)

    def status(self):
        return json.loads(self.cli("status", "--json").stdout)

    def wait_for(self, predicate, seconds=8):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            if predicate():
                return
            time.sleep(0.05)
        self.fail("condition was not met before timeout")


class LauncherTests(WorkspaceTest):
    def test_help_from_another_directory_without_pythonpath(self):
        env = dict(os.environ)
        env.pop("PYTHONPATH", None)
        result = subprocess.run([sys.executable, str(ROOT / "syncToCeph"), "--help"],
                                cwd=self.root, env=env, capture_output=True,
                                text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("{run,schedule,status,stop,logs}", result.stdout)
        self.assertFalse((self.root / ".synctoceph-state").exists())


class TransferTests(WorkspaceTest):
    def test_archive_survives_source_deletion(self):
        (self.source / "nested").mkdir()
        (self.source / "nested" / "data.bin").write_bytes(b"data\x00\xff")
        (self.dest / "other-machine.txt").write_text("retain")
        self.run_sync()
        self.assertTrue(self.status()["last_run"]["verified"])
        (self.source / "nested" / "data.bin").unlink()
        self.run_sync()
        self.assertEqual((self.dest / "nested" / "data.bin").read_bytes(), b"data\x00\xff")
        self.assertEqual((self.dest / "other-machine.txt").read_text(), "retain")

    def test_changed_file_backs_up_previous_bytes_even_with_same_mtime_and_size(self):
        source = self.source / "data"
        source.write_bytes(b"old")
        self.run_sync()
        old = source.stat()
        source.write_bytes(b"new")
        os.utime(source, ns=(old.st_atime_ns, old.st_mtime_ns))
        self.run_sync()
        last = self.status()["last_run"]
        self.assertEqual((self.dest / "data").read_bytes(), b"new")
        self.assertEqual((Path(last["history_dir"]) / "data").read_bytes(), b"old")
        self.assertEqual(last["transferred_bytes"], 3)
        self.assertEqual(last["verified_bytes"], 3)
        self.run_sync()
        self.assertEqual(self.status()["last_run"]["transferred_bytes"], 0)

    def test_dry_run_does_not_write_destination(self):
        (self.source / "data").write_text("preview")
        self.run_sync("--dry-run")
        self.assertEqual(list(self.dest.iterdir()), [])
        state = self.status()
        self.assertFalse(state["last_run"]["verified"])
        self.assertIsNone(state["last_success_at"])
        self.assertEqual(state["total_transferred_bytes"], 0)

    def test_content_corruption_is_detected(self):
        (self.source / "data").write_text("good")
        (self.dest / "data").write_text("evil")
        manifest = snapshot(self.source, Control())
        with self.assertRaisesRegex(SyncError, "SHA-256"):
            verify(self.settings, manifest, Control())

    def test_source_mutation_and_removal_are_detected(self):
        file = self.source / "data"
        file.write_text("before")
        manifest = snapshot(self.source, Control())
        file.write_text("after")
        with self.assertRaisesRegex(SyncError, "source changed"):
            verify(self.settings, manifest, Control())
        file.unlink()
        with self.assertRaises(OSError):
            verify(self.settings, manifest, Control())

    def test_new_source_file_is_detected(self):
        manifest = snapshot(self.source, Control())
        (self.source / "new").touch()
        with self.assertRaisesRegex(SyncError, "source tree changed"):
            verify(self.settings, manifest, Control())

    def test_reject_source_symlinks_and_special_files(self):
        link = self.source / "link"
        link.symlink_to(self.dest, target_is_directory=True)
        self.run_sync(expected=1)
        link.unlink()
        os.mkfifo(self.source / "pipe")
        self.run_sync(expected=1)
        self.assertEqual(list((self.dest / ".syncToCeph").glob("history/*")), [])

    def test_reject_destination_symlink(self):
        (self.source / "data").write_text("source")
        outside = self.root / "outside"
        outside.write_text("keep")
        (self.dest / "data").symlink_to(outside)
        self.run_sync(expected=1)
        self.assertEqual(outside.read_text(), "keep")

    def test_type_conflict_never_removes_destination_files(self):
        (self.source / "conflict").write_text("file")
        (self.dest / "conflict").mkdir()
        (self.dest / "conflict" / "retain").write_text("keep")
        self.run_sync(expected=1)
        self.assertEqual((self.dest / "conflict" / "retain").read_text(), "keep")

    def test_missing_destination_and_unmounted_path_fail(self):
        self.dest.rmdir()
        self.run_sync(expected=1)
        self.assertFalse(self.dest.exists())
        self.dest.mkdir()
        self.run_sync("--require-mount", str(self.dest), expected=1)

    def test_overlapping_paths_rejected_before_writes(self):
        self.cli("run", "--source", str(self.source), "--dest", str(self.source / "child"), expected=1)
        self.assertFalse(self.state.exists())

    def test_reserved_names_are_rejected(self):
        (self.source / ".syncToCeph-partial").mkdir()
        self.run_sync(expected=1)

    def test_archive_lock_serializes_different_state_directories(self):
        metadata = self.dest / ".syncToCeph"
        metadata.mkdir()
        with Lock(metadata / "archive.lock"):
            self.run_sync(expected=1)
        self.assertIn("another task holds", self.status()["last_run"]["error"])

    def test_filename_spaces_newlines_and_shell_characters(self):
        name = "-data : $(touch UNEXPECTED)\nname"
        (self.source / name).write_text("literal")
        self.run_sync()
        self.assertEqual((self.dest / name).read_text(), "literal")
        self.assertFalse((ROOT / "UNEXPECTED").exists())

    def test_rsync_failure_is_persisted_and_not_verified(self):
        self.state.mkdir()
        logger = logging.getLogger("test.failure")
        with Lock(self.state / "job.lock") as lock:
            with patch("synctoceph.engine.execute", return_value=(23, 42)):
                result = sync(self.settings, Control(), logger, lock.fd)
        self.assertEqual(result["exit_code"], 23)
        self.assertEqual(result["rsync_exit_code"], 23)
        self.assertFalse(result["verified"])

    def test_no_destructive_flags(self):
        command = rsync_command("rsync", self.settings, "test")
        self.assertFalse(any(arg.startswith(("--delete", "--remove-source", "--inplace", "--append")) for arg in command))
        self.assertIn("--checksum", command)

    def test_real_rsync_interruption_keeps_old_file_and_can_resume(self):
        (self.source / "large").write_bytes(os.urandom(2 * 1024 * 1024))
        (self.dest / "large").write_bytes(b"previous version")
        self.state.mkdir()
        control = Control()
        command = rsync_command("rsync", self.settings, "interrupted")
        command.insert(1, "--bwlimit=128")

        def interrupt_after_data_arrives():
            deadline = time.monotonic() + 8
            while time.monotonic() < deadline:
                if any(path.stat().st_size >= 65536 for path in self.dest.glob(".large.*")):
                    break
                time.sleep(0.05)
            control.request_stop()

        interrupter = threading.Thread(target=interrupt_after_data_arrives)
        with Lock(self.state / "job.lock") as lock:
            interrupter.start()
            try:
                code, _ = execute(command, control, logging.getLogger("test.interrupt"), (lock.fd,))
            finally:
                interrupter.join()
        self.assertNotEqual(code, 0)
        self.assertEqual((self.dest / "large").read_bytes(), b"previous version")
        self.assertTrue((self.dest / ".syncToCeph-partial" / "large").is_file())
        self.run_sync()
        self.assertEqual((self.source / "large").read_bytes(), (self.dest / "large").read_bytes())
        self.assertTrue(self.status()["last_run"]["verified"])


class ProcessTests(WorkspaceTest):
    def scheduler_args(self, *extra):
        return ["schedule", "--source", str(self.source), "--dest", str(self.dest), *extra]

    def test_background_scheduler_status_logs_stop_and_duplicate_lock(self):
        (self.source / "data").write_text("scheduled")
        try:
            self.cli(*self.scheduler_args("--every", "1s", "--immediate", "--background"))
            self.wait_for(lambda: self.status().get("last_run") is not None)
            self.assertTrue(self.status()["running"])
            self.run_sync(expected=1)
            self.wait_for(lambda: len(self.status()["recent_runs"]) >= 2)
            self.assertIn("verified=True", self.cli("logs", "-n", "100").stdout)
        finally:
            self.cli("stop")
        state = self.status()
        self.assertFalse(state["running"])
        self.assertEqual(state["state"], "stopped")
        self.cli("stop")

    def test_sigterm_while_waiting(self):
        process = subprocess.Popen([sys.executable, str(ROOT / "syncToCeph"), "--state-dir", str(self.state),
                                    *self.scheduler_args("--at", "02:00")],
                                   cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            self.wait_for(lambda: (self.state / "status.json").exists())
            process.send_signal(signal.SIGTERM)
            stdout, stderr = process.communicate(timeout=5)
            self.assertEqual(process.returncode, 0, stderr)
            self.assertEqual(self.status()["state"], "stopped")
        finally:
            if process.poll() is None:
                process.kill()
                process.communicate()

    def test_stop_during_active_rsync_preserves_prior_destination(self):
        # A fixture on PATH waits for SIGINT to test signal forwarding independently
        # of transfer duration.
        scripts = self.root / "bin"
        scripts.mkdir()
        fake = scripts / "rsync"
        marker = self.root / "rsync-ready"
        fake.write_text("#!/usr/bin/env python3\nimport pathlib, signal, sys, time\n"
                        "signal.signal(signal.SIGINT, lambda *_: sys.exit(20))\n"
                        f"pathlib.Path({str(marker)!r}).touch()\n"
                        "while True: time.sleep(0.1)\n")
        fake.chmod(0o755)
        (self.dest / "existing").write_text("keep")
        env = dict(os.environ, PATH=str(scripts) + os.pathsep + os.environ["PATH"])
        process = subprocess.Popen([sys.executable, str(ROOT / "syncToCeph"), "--state-dir", str(self.state),
                                    "run", "--source", str(self.source), "--dest", str(self.dest)],
                                   cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            self.wait_for(marker.exists)
            self.cli("stop")
            _, stderr = process.communicate(timeout=5)
            self.assertEqual(process.returncode, 130, stderr)
            state = self.status()
            self.assertFalse(state["last_run"]["verified"])
            self.assertEqual(state["last_run"]["rsync_exit_code"], 20)
            self.assertEqual((self.dest / "existing").read_text(), "keep")
        finally:
            if process.poll() is None:
                process.terminate()
                process.communicate(timeout=5)

    def test_identity_mismatch_cannot_signal_current_process(self):
        process = identity(os.getpid())
        process["start_ticks"] = "invalid"
        self.assertFalse(signal_process(process))

    def test_state_is_read_only_for_idle_monitoring(self):
        self.assertEqual(self.status()["state"], "idle")
        self.cli("logs")
        self.cli("stop")
        self.assertFalse(self.state.exists())

    def test_scheduler_recovers_on_next_interval_after_missing_destination(self):
        self.dest.rmdir()
        try:
            self.cli(*self.scheduler_args("--every", "1s", "--immediate", "--background"))
            self.wait_for(lambda: self.status().get("last_run") is not None)
            self.assertEqual(self.status()["last_run"]["exit_code"], 1)
            self.dest.mkdir()
            self.wait_for(lambda: self.status().get("last_run", {}).get("verified"))
        finally:
            self.cli("stop")

    def test_orphan_lock_is_reported_without_signalling_unknown_process(self):
        self.state.mkdir()
        with Lock(self.state / "job.lock"):
            self.assertEqual(self.status()["state"], "orphaned")
            self.cli("stop", expected=1)


class ConfigurationTests(WorkspaceTest):
    def test_intervals_and_times(self):
        self.assertEqual(interval_seconds("4h"), 14400)
        self.assertEqual(interval_seconds("30m"), 1800)
        self.assertEqual(daily_time("02:00"), (2, 0))
        for value in ("0s", "-1h", "4", "1.5h", "inf"):
            with self.assertRaises(ValueError):
                interval_seconds(value)
        for value in ("24:00", "2:00", "12:60"):
            with self.assertRaises(ValueError):
                daily_time(value)

    def test_config_precedence_and_unknown_keys(self):
        config = self.root / "config.toml"
        config.write_text(f'source = "{self.source}"\ndest = "{self.dest}"\ndry_run = true\n')
        self.cli("--config", str(config), "run")
        self.assertTrue(self.status()["last_run"]["dry_run"])
        config.write_text("delete = true\n")
        with self.assertRaisesRegex(SyncError, "unknown"):
            load_config(str(config))

    def test_dst_skips_missing_daily_time(self):
        now = datetime(2026, 3, 29, 0, 0, tzinfo=timezone.utc)
        result = next_daily(now, "01:30", ZoneInfo("Europe/London"))
        self.assertEqual(result, datetime(2026, 3, 30, 0, 30, tzinfo=timezone.utc))

    def test_dst_repeated_time_only_once(self):
        now = datetime(2026, 10, 25, 0, 31, tzinfo=timezone.utc)
        result = next_daily(now, "01:30", ZoneInfo("Europe/London"), "2026-10-25")
        self.assertEqual(result, datetime(2026, 10, 26, 1, 30, tzinfo=timezone.utc))


if __name__ == "__main__":
    unittest.main()
