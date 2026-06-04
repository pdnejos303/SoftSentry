package osutil

import "testing"

func TestFormatWindowsVersion(t *testing.T) {
	tests := []struct {
		name  string
		major uint64
		minor uint64
		build string
		want  string
	}{
		{"win11 full", 10, 0, "26200", "10.0.26200"},
		{"no build falls back to major.minor", 10, 0, "", "10.0"},
		{"nonzero minor", 6, 3, "9600", "6.3.9600"},
		{"missing everything yields fallback", 0, 0, "", fallbackVersion},
		{"build only (legacy DWORDs absent)", 0, 0, "9200", "9200"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatWindowsVersion(tt.major, tt.minor, tt.build); got != tt.want {
				t.Errorf("formatWindowsVersion(%d, %d, %q) = %q, want %q",
					tt.major, tt.minor, tt.build, got, tt.want)
			}
		})
	}
}

func TestParseSwVers(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"trailing newline", "14.5\n", "14.5"},
		{"surrounding whitespace", "  13.2.1 \n", "13.2.1"},
		{"empty yields fallback", "", fallbackVersion},
		{"whitespace only yields fallback", "  \n", fallbackVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSwVers(tt.out); got != tt.want {
				t.Errorf("parseSwVers(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}
