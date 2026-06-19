package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// removeDataTree เป็น helper เลียนแบบ config.Dir โดยให้ test ชี้ไป temp dir ได้
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
	prevData, prevSvc, prevReg, prevInstall := dataDirFn, removeServiceFn, unregisterFn, removeInstallFn
	dataDirFn = func() (string, error) { return dataDir, nil }
	removeServiceFn = func() error { return nil }
	unregisterFn = func() error { return nil }
	removeInstallFn = func(string) error { return nil }
	t.Cleanup(func() {
		dataDirFn, removeServiceFn, unregisterFn, removeInstallFn = prevData, prevSvc, prevReg, prevInstall
	})

	var out bytes.Buffer
	if err := removeAgent(&out, removeOptions{Purge: true}); err != nil {
		t.Fatalf("removeAgent: %v", err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("data dir still present after purge: %v", err)
	}
}
