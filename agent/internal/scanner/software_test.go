package scanner

import "testing"

func TestMergeWindowsRegistryWinsOnNameVersionCollision(t *testing.T) {
	reg := []Software{{Name: "7-Zip", Version: "23.01", InstallPath: `C:\Program Files\7-Zip\7z.exe`, Source: "registry"}}
	fs := []Software{{Name: "7-Zip", Version: "23.01", InstallPath: `C:\Tools\7z.exe`, Source: "filesystem"}}

	out := mergeWindows(reg, fs)
	if len(out) != 1 {
		t.Fatalf("collision should drop fs entry; got %d entries: %+v", len(out), out)
	}
	if out[0].Source != "registry" {
		t.Errorf("registry must win, got source %q", out[0].Source)
	}
}

func TestMergeWindowsKeepsDistinctFilesystemEntry(t *testing.T) {
	reg := []Software{{Name: "Office", Version: "16.0", Source: "registry"}}
	fs := []Software{{Name: "PortableApp", Version: "2.0", InstallPath: `C:\Tools\pa.exe`, Source: "filesystem"}}

	out := mergeWindows(reg, fs)
	if len(out) != 2 {
		t.Fatalf("want both entries, got %d: %+v", len(out), out)
	}
}

func TestMergeWindowsDedupsFilesystemAgainstItself(t *testing.T) {
	fs := []Software{
		{Name: "Tool", Version: "1.0", InstallPath: `C:\a\tool.exe`, Source: "filesystem"},
		{Name: "Tool", Version: "1.0", InstallPath: `C:\b\tool.exe`, Source: "filesystem"},
	}
	out := mergeWindows(nil, fs)
	if len(out) != 1 {
		t.Fatalf("same name+version fs entries must collapse to one; got %d: %+v", len(out), out)
	}
	if out[0].InstallPath != `C:\a\tool.exe` {
		t.Errorf("first fs entry should be kept, got %q", out[0].InstallPath)
	}
}
