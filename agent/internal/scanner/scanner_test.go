package scanner

import "testing"

func TestDedupeCollapsesIdenticalEntries(t *testing.T) {
	in := []Software{
		{Name: "7-Zip", Version: "22.01", InstallPath: `C:\7z\7z.exe`},
		{Name: "7-Zip", Version: "22.01", InstallPath: `C:\7z\7z.exe`}, // WOW6432Node dup
		{Name: "7-Zip", Version: "19.00", InstallPath: `C:\7z\7z.exe`}, // different version: kept
		{Name: "Chrome", Version: "120", InstallPath: ""},
	}
	got := dedupe(in)
	if len(got) != 3 {
		t.Fatalf("dedupe: want 3 entries, got %d: %+v", len(got), got)
	}
}

func TestDedupeKeepsFirstOccurrence(t *testing.T) {
	in := []Software{
		{Name: "App", Version: "1.0", InstallPath: "/a", Publisher: "first"},
		{Name: "App", Version: "1.0", InstallPath: "/a", Publisher: "second"},
	}
	got := dedupe(in)
	if len(got) != 1 || got[0].Publisher != "first" {
		t.Fatalf("dedupe should keep first occurrence, got %+v", got)
	}
}

func TestSortStableOrdersByNameThenVersion(t *testing.T) {
	in := []Software{
		{Name: "Zoom", Version: "5.0"},
		{Name: "Acrobat", Version: "23"},
		{Name: "Acrobat", Version: "22"},
	}
	sortStable(in)
	if in[0].Name != "Acrobat" || in[0].Version != "22" || in[2].Name != "Zoom" {
		t.Fatalf("unexpected order: %+v", in)
	}
}
