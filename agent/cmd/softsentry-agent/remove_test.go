package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallCmdPurgeFlag(t *testing.T) {
	cmd := uninstallCmd()
	if cmd.Flags().Lookup("purge") == nil {
		t.Fatal("uninstall command missing --purge flag")
	}
	if cmd.Flags().Lookup("keep-config") == nil {
		t.Fatal("uninstall command missing --keep-config flag")
	}
}

func TestRemoveAgentPurgeDeletesDataDir(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "SoftSentry")
	if err := os.MkdirAll(filepath.Join(dataDir, "queue"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// inject data dir + ปิดการแตะ service/registry/install-dir จริงระหว่าง test
	prevData, prevSvc, prevReg, prevTray, prevInstall := dataDirFn, removeServiceFn, unregisterFn, unregisterTrayFn, removeInstallFn
	dataDirFn = func() (string, error) { return dataDir, nil }
	removeServiceFn = func() error { return nil }
	unregisterFn = func() error { return nil }
	unregisterTrayFn = func() error { return nil }
	removeInstallFn = func(string) error { return nil }
	t.Cleanup(func() {
		dataDirFn, removeServiceFn, unregisterFn, unregisterTrayFn, removeInstallFn = prevData, prevSvc, prevReg, prevTray, prevInstall
	})

	var out bytes.Buffer
	if err := removeAgent(&out, removeOptions{Purge: true}); err != nil {
		t.Fatalf("removeAgent: %v", err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("data dir still present after purge: %v", err)
	}
}
