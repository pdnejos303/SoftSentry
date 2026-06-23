//go:build windows

// windows_registry.go เก็บ logic อ่าน software inventory จาก Windows registry
// (Programs and Features / Uninstall hives) ย้ายมาจาก windows.go เดิมไม่เปลี่ยนพฤติกรรม
package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

// uninstallHive ระบุตำแหน่งหนึ่งของ "Programs and Features" ใน registry
type uninstallHive struct {
	root registry.Key // root key เช่น HKLM หรือ HKCU
	path string       // sub-path ภายใต้ root ที่เก็บข้อมูล Uninstall
	arch string       // สถาปัตยกรรม (x64/x86) ของรายการใน hive นี้
}

// hives คือรายการตำแหน่ง registry ที่ Windows บันทึกซอฟต์แวร์ที่ติดตั้งไว้ (spec 1.3)
// WOW6432Node เก็บแอป 32-bit บน OS 64-bit; HKCU เก็บแอปเฉพาะ user ปัจจุบัน
var hives = []uninstallHive{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "x64"},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, "x86"},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "x64"},
}

// scanRegistry อ่านทุก hive แล้วคืนรายการ software จาก registry (source=registry)
// honor ctx cancellation ระหว่าง hive
func (s *windowsScanner) scanRegistry(ctx context.Context) ([]Software, error) {
	var all []Software
	for _, h := range hives {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		all = append(all, s.scanHive(h)...)
	}
	return all, nil
}

// scanHive อ่าน subkey ทุก uninstall key ภายใต้ hive ที่กำหนด
func (s *windowsScanner) scanHive(h uninstallHive) []Software {
	root, err := registry.OpenKey(h.root, h.path, registry.READ)
	if err != nil {
		return nil // hive ไม่มีอยู่ (เช่น ไม่มี WOW6432Node บน 32-bit OS) — ไม่ใช่ error
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

// parseEntry อ่าน uninstall subkey หนึ่ง key แปลงเป็น Software (false = ข้ามรายการ)
func parseEntry(root registry.Key, subkey, arch string) (Software, bool) {
	k, err := registry.OpenKey(root, subkey, registry.QUERY_VALUE)
	if err != nil {
		return Software{}, false
	}
	defer k.Close()

	if sc, _, err := k.GetIntegerValue("SystemComponent"); err == nil && sc == 1 {
		return Software{}, false // OS component — ไม่รายงาน
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

	// เวลาติดตั้งจริง (best-effort): registry InstallDate ให้แค่ "วันที่" (บางตัว
	// ไม่มีเลย) — เติมเวลาที่แท้จริงจากเวลา "สร้าง" ของโฟลเดอร์ติดตั้ง/main exe
	// ซึ่งตรงกับตอนที่ installer วางไฟล์ลงเครื่อง. ถ้าหาไม่ได้ปล่อยว่าง (backend
	// fallback ไปใช้ install_date)
	sw.InstalledAt = resolveInstalledAt(k, sw.InstallPath)

	if sz, _, err := k.GetIntegerValue("EstimatedSize"); err == nil {
		sw.InstallSizeKB = int64(sz)
	}
	return sw, true
}

// resolveInstalledAt คืนเวลาติดตั้งจริงเป็น RFC3339 (UTC) แบบ best-effort:
// ลองเวลาสร้างของ InstallLocation (โฟลเดอร์ติดตั้ง) ก่อน แล้วค่อย main exe ที่
// resolve ได้ — โฟลเดอร์มักสะท้อนเวลา extract ของ installer ได้แม่นกว่า. คืน ""
// ถ้าทั้งสองอ่านไม่ได้ (เช่น path หาย/แอป store)
func resolveInstalledAt(k registry.Key, exePath string) string {
	loc := strings.TrimSpace(regString(k, "InstallLocation"))
	for _, p := range []string{loc, exePath} {
		if p == "" {
			continue
		}
		if t, ok := fileCreationTime(p); ok {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// fileCreationTime อ่านเวลา "สร้าง" ของไฟล์/โฟลเดอร์บน Windows ผ่าน
// Win32FileAttributeData (os.Stat ปกติให้แค่ ModTime). คืน false ถ้า stat ไม่ได้
// หรือ field ไม่ใช่ชนิดที่คาด (กันพังข้ามแพลตฟอร์ม/ระบบไฟล์แปลก)
func fileCreationTime(path string) (time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	attr, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, attr.CreationTime.Nanoseconds()), true
}

// regString อ่านค่า string จาก registry (รองรับ REG_EXPAND_SZ)
func regString(k registry.Key, name string) string {
	if v, _, err := k.GetStringValue(name); err == nil {
		if expanded, err := registry.ExpandString(v); err == nil {
			return expanded
		}
		return v
	}
	return ""
}

// resolveExecutable เลือก path ที่จะใช้ตรวจลายเซ็น (DisplayIcon .exe → main exe → โฟลเดอร์)
func resolveExecutable(k registry.Key) string {
	if icon := cleanIconPath(regString(k, "DisplayIcon")); icon != "" {
		if strings.HasSuffix(strings.ToLower(icon), ".exe") {
			return icon
		}
	}

	loc := strings.TrimSpace(regString(k, "InstallLocation"))
	if loc == "" {
		return ""
	}
	if exe := findMainExe(loc, regString(k, "DisplayName")); exe != "" {
		return exe
	}
	return loc
}

// cleanIconPath ตัด icon index (",0") และ quote ออกจาก DisplayIcon
func cleanIconPath(icon string) string {
	if icon == "" {
		return ""
	}
	if idx := strings.LastIndex(icon, ","); idx > 1 {
		icon = icon[:idx]
	}
	return strings.Trim(strings.TrimSpace(icon), `"`)
}

// findMainExe เลือก main .exe ในโฟลเดอร์ติดตั้งผ่าน chooseExecutable
func findMainExe(dir, displayName string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	cands := make([]exeCandidate, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			continue
		}
		var size int64
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		cands = append(cands, exeCandidate{Path: filepath.Join(dir, e.Name()), Size: size})
	}
	return chooseExecutable(displayName, cands)
}

// normalizeInstallDate แปลง YYYYMMDD → YYYY-MM-DD (กรองปีนอกช่วง / ปี พ.ศ.)
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
		return ""
	}
	return t.Format("2006-01-02")
}
