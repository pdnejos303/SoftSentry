package main

import "testing"

func TestUninstallFlagDetection(t *testing.T) {
	if !hasFlag([]string{"--uninstall"}, uninstallFlag) {
		t.Fatal("should detect --uninstall")
	}
	if hasFlag([]string{"run"}, uninstallFlag) {
		t.Fatal("should not treat `run` as uninstall")
	}
	// elevated continuation marker
	if !hasFlag([]string{"--uninstall", "--accept-uninstall"}, acceptUninstallFlag) {
		t.Fatal("should detect --accept-uninstall")
	}
}
