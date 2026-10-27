package daemon

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestSetupCommandsDelegateAuthentication(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() == 0 {
		t.Skip("requires unprivileged Linux user")
	}
	dir := t.TempDir()
	for _, name := range []string{"sudo", "loginctl", "systemctl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	cmd, err := SetupCommand("disable legacy service")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sudo", "--", "systemctl", "disable", "--now", "sho-rt.service"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("got %v", cmd.Args)
	}
	cmd, err = SetupCommand("enable boot startup")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cmd.Args[:4], []string{"sudo", "--", "loginctl", "enable-linger"}) {
		t.Fatalf("got %v", cmd.Args)
	}
	if _, err := SetupCommand("arbitrary command"); err == nil {
		t.Fatal("unknown action accepted")
	}
}

func TestLegacyServiceCheck(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	path := filepath.Join(dir, "systemctl")
	for _, tc := range []struct {
		code string
		want bool
	}{{"1", false}, {"0", true}} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit "+tc.code+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		if got := LegacyServiceActive(); got != tc.want {
			t.Fatalf("got %v want %v", got, tc.want)
		}
	}
}
