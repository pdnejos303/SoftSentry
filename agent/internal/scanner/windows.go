//go:build windows

package scanner

import (
	"context"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// uninstallHive identifies one "Programs and Features" registry location.
type uninstallHive struct {
	root registry.Key
	path string
	arch string // arch we attribute to entries under this hive
}

// hives lists the registry locations Windows records installed software in
// (spec 1.3 / protocol §3). WOW6432Node holds 32-bit apps on a 64-bit OS;
// HKCU holds per-user installs. Duplicates across hives are deduped later.
var hives = []uninstallHive{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "x64"},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, "x86"},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "x64"},
}

type windowsScanner struct {
	verifier *authenticodeVerifier
}

// New returns the Windows software scanner.
func New() Scanner { return &windowsScanner{verifier: newAuthenticodeVerifier()} }

func (s *windowsScanner) Scan(ctx context.Context) ([]Software, error) {
	var all []Software
	for _, h := range hives {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		all = append(all, s.scanHive(h)...)
	}
	// Verify signatures for entries that resolved to an executable path.
	for i := range all {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if all[i].InstallPath == "" {
			continue
		}
		if sig := s.verifier.verify(all[i].InstallPath); sig != nil {
			all[i].Signature = sig
		}
	}
	return all, nil
}

// scanHive reads every uninstall subkey under one hive, skipping entries that
// have no DisplayName or are flagged as OS components (spec 1.3).
func (s *windowsScanner) scanHive(h uninstallHive) []Software {
	root, err := registry.OpenKey(h.root, h.path, registry.READ)
	if err != nil {
		return nil // hive absent (e.g. no WOW6432Node) is not an error
	}
	defer root.Close()

	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}

	out := make([]Software, 0, len(names))
	for _, name := range names {
		if sw, ok := parseEntry(root, name, h.arch); ok {
			out = append(out, sw)
		}
	}
	return out
}

// parseEntry reads one uninstall subkey into a Software, or returns false to
// skip it (missing DisplayName, SystemComponent=1).
func parseEntry(root registry.Key, subkey, arch string) (Software, bool) {
	k, err := registry.OpenKey(root, subkey, registry.QUERY_VALUE)
	if err != nil {
		return Software{}, false
	}
	defer k.Close()

	if sc, _, err := k.GetIntegerValue("SystemComponent"); err == nil && sc == 1 {
		return Software{}, false
	}

	name := strings.TrimSpace(regString(k, "DisplayName"))
	if name == "" {
		return Software{}, false
	}

	sw := Software{
		Name:        name,
		Version:     strings.TrimSpace(regString(k, "DisplayVersion")),
		Publisher:   strings.TrimSpace(regString(k, "Publisher")),
		InstallDate: normalizeInstallDate(regString(k, "InstallDate")),
		InstallPath: resolveExecutable(k),
		Arch:        arch,
		Source:      "registry",
	}
	if sz, _, err := k.GetIntegerValue("EstimatedSize"); err == nil {
		sw.InstallSizeKB = int64(sz) // EstimatedSize is already in KB
	}
	return sw, true
}

// regString reads a string value, tolerating absence and REG_EXPAND_SZ.
func regString(k registry.Key, name string) string {
	if v, _, err := k.GetStringValue(name); err == nil {
		if expanded, err := registry.ExpandString(v); err == nil {
			return expanded
		}
		return v
	}
	return ""
}

// resolveExecutable picks a filesystem path to verify the signature against,
// preferring an explicit DisplayIcon (often the main .exe) then InstallLocation.
func resolveExecutable(k registry.Key) string {
	if icon := regString(k, "DisplayIcon"); icon != "" {
		// DisplayIcon is often "C:\path\app.exe,0" — strip the icon index.
		if idx := strings.LastIndex(icon, ","); idx > 1 {
			icon = icon[:idx]
		}
		icon = strings.Trim(icon, `"`)
		if strings.HasSuffix(strings.ToLower(icon), ".exe") {
			return icon
		}
	}
	return strings.TrimSpace(regString(k, "InstallLocation"))
}

// normalizeInstallDate converts the registry's YYYYMMDD form to YYYY-MM-DD.
// It returns "" for anything that isn't a real Gregorian date in a sane range:
// installers on Thai-locale machines sometimes write Buddhist-era years or
// outright garbage (e.g. "25674421"), which the backend rightly rejects.
func normalizeInstallDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) != 8 {
		return ""
	}
	t, err := time.Parse("20060102", raw)
	if err != nil {
		return ""
	}
	if t.Year() < 1980 || t.Year() > time.Now().Year()+1 {
		return "" // out of plausible range (likely Buddhist-era or corrupt)
	}
	return t.Format("2006-01-02")
}
