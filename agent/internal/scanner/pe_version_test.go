package scanner

import "testing"

func TestDisplayNamePrefersProductName(t *testing.T) {
	info := peVersionInfo{ProductName: "Visual Studio Code", FileDescription: "Code", CompanyName: "Microsoft"}
	if got := info.displayName("Code.exe"); got != "Visual Studio Code" {
		t.Errorf("want ProductName, got %q", got)
	}
}

func TestDisplayNameFallsBackToFileDescription(t *testing.T) {
	info := peVersionInfo{FileDescription: "7-Zip File Manager"}
	if got := info.displayName("7zFM.exe"); got != "7-Zip File Manager" {
		t.Errorf("want FileDescription, got %q", got)
	}
}

func TestDisplayNameFallsBackToFilenameWithoutExt(t *testing.T) {
	info := peVersionInfo{}
	if got := info.displayName("portable_tool.EXE"); got != "portable_tool" {
		t.Errorf("want stripped filename, got %q", got)
	}
}

func TestDisplayNameIgnoresWhitespaceOnlyFields(t *testing.T) {
	info := peVersionInfo{ProductName: "   ", FileDescription: "\t"}
	if got := info.displayName("app.exe"); got != "app" {
		t.Errorf("want filename fallback when fields blank, got %q", got)
	}
}
