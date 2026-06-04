//go:build darwin

package scanner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// appDirs are the locations macOS records GUI applications (spec 1.3).
var appDirs = []string{"/Applications", "/System/Applications"}

type darwinScanner struct{}

// New returns the macOS software scanner.
func New() Scanner { return &darwinScanner{} }

func (s *darwinScanner) Scan(ctx context.Context) ([]Software, error) {
	var out []Software
	for _, dir := range appDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // directory may not exist on every macOS layout
		}
		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			appPath := filepath.Join(dir, e.Name())
			if sw, ok := readApp(appPath); ok {
				sw.Signature = codesignVerify(appPath)
				out = append(out, sw)
			}
		}
	}
	return out, nil
}

// infoPlist is the subset of Contents/Info.plist we care about.
type infoPlist struct {
	Name      string `json:"CFBundleName"`
	Version   string `json:"CFBundleShortVersionString"`
	Copyright string `json:"NSHumanReadableCopyright"`
}

// readApp parses an .app bundle's Info.plist via `plutil` (ships with macOS).
func readApp(appPath string) (Software, bool) {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	data, err := exec.Command("plutil", "-convert", "json", "-o", "-", plistPath).Output()
	if err != nil {
		return Software{}, false
	}
	var p infoPlist
	if err := json.Unmarshal(data, &p); err != nil {
		return Software{}, false
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(appPath), ".app")
	}
	return Software{
		Name:        name,
		Version:     strings.TrimSpace(p.Version),
		Publisher:   strings.TrimSpace(p.Copyright),
		InstallPath: appPath,
		Source:      "plist",
	}, true
}

// codesignVerify maps `codesign --verify` results to our status enum
// (protocol §3 macOS mapping).
func codesignVerify(appPath string) *Signature {
	cmd := exec.Command("codesign", "--verify", "--deep", "--strict", appPath)
	combined, err := cmd.CombinedOutput()
	if err == nil {
		return &Signature{Status: SigValid, Signer: codesignAuthority(appPath)}
	}
	if strings.Contains(string(combined), "not signed") {
		return &Signature{Status: SigUnsigned}
	}
	return &Signature{Status: SigInvalid}
}

// codesignAuthority pulls the leaf signing authority from `codesign -dvvv`.
func codesignAuthority(appPath string) string {
	out, err := exec.Command("codesign", "-dvvv", appPath).CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Authority=") {
			return strings.TrimPrefix(strings.TrimSpace(line), "Authority=")
		}
	}
	return ""
}
