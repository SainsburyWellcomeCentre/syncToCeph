// Tests for reading the mount table.
package platform

import "testing"

func TestParseMountinfoLine(t *testing.T) {
	line := `158 78 0:83 / /mnt/z rw,noatime - 9p Z:\134 rw,dirsync,aname=drvfs;path=Z:\;uid=1000`
	m, ok := parseMountinfoLine(line)
	if !ok || m.Point != "/mnt/z" || m.FSType != "9p" || m.Source != `Z:\` || !IsWindowsDrive(m) || !IsNetwork(m) {
		t.Fatalf("got %+v", m)
	}
	m, ok = parseMountinfoLine(`36 35 98:0 / /mnt/lab\040share rw shared:1 - cifs //srv/lab rw`)
	if !ok || m.Point != "/mnt/lab share" || IsWindowsDrive(m) || !IsNetwork(m) {
		t.Fatalf("got %+v", m)
	}
	if _, ok := parseMountinfoLine("garbage"); ok {
		t.Fatal("garbage parsed")
	}
}

func TestMountFor(t *testing.T) {
	mounts := []Mount{{Point: "/", FSType: "ext4"}, {Point: "/mnt/z", FSType: "autofs"},
		{Point: "/mnt/z", FSType: "cifs"}, {Point: "/mnt/zz", FSType: "ext4"}}
	if m, _ := MountFor(mounts, "/mnt/z/lab"); m.FSType != "cifs" {
		t.Errorf("the last mount on a point wins, got %+v", m)
	}
	if m, _ := MountFor(mounts, "/mnt/zz/x"); m.Point != "/mnt/zz" {
		t.Errorf("prefix must match whole path parts, got %+v", m)
	}
	if !IsMountPoint(mounts, "/mnt/z") || IsMountPoint(mounts, "/mnt/z/lab") {
		t.Error("IsMountPoint is wrong")
	}
}

func TestDriveLetter(t *testing.T) {
	for path, want := range map[string]string{"/mnt/z": "Z", "/mnt/d/data": "D", "/mnt/ceph": "", "/home/z": ""} {
		if got := DriveLetter(path); got != want {
			t.Errorf("DriveLetter(%q) = %q", path, got)
		}
	}
}
