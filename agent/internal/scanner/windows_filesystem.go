//go:build windows

// windows_filesystem.go เดิน filesystem หา .exe ที่ "ไม่ได้อยู่ใน registry" เพื่อให้
// ตรวจ signature ของโปรแกรม portable/unregistered ได้ด้วย (จุดที่ malware ชอบอยู่)
// เก็บเฉพาะ curated roots เป็น default (ข้าม C:\Windows), มี deep mode กวาดทั้งไดรฟ์
package scanner

import (
	"context"
	"debug/pe"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// fsOptions คุมพฤติกรรมการเดิน filesystem
type fsOptions struct {
	deep       bool     // true = กวาดทั้ง system drive รวม Windows
	extraRoots []string // root เพิ่มเติมจาก config
}

// collectFilesystem คำนวณ roots จาก options แล้วเดินหา .exe ที่ไม่อยู่ใน skip set
func collectFilesystem(ctx context.Context, opt fsOptions, skip map[string]struct{}) ([]Software, error) {
	return walkRoots(ctx, scanRoots(opt), skip)
}

// walkRoots เดินทุก root ด้วย filepath.WalkDir เก็บ .exe เป็น Software entry
// best-effort: error ระดับไฟล์/โฟลเดอร์ข้ามไป; ตรวจ ctx ทุก 256 entry เพื่อ honor timeout
func walkRoots(ctx context.Context, roots []string, skip map[string]struct{}) ([]Software, error) {
	var out []Software
	count := 0
	for _, root := range roots {
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return filepath.SkipDir // โฟลเดอร์อ่านไม่ได้ — ข้ามทั้งกิ่ง
				}
				return nil // ไฟล์อ่านไม่ได้ — ข้ามตัวเดียว
			}
			count++
			if count%256 == 0 {
				if cerr := ctx.Err(); cerr != nil {
					return cerr // honor cancellation/timeout
				}
			}
			if d.IsDir() {
				if isReparse(d) {
					return filepath.SkipDir // junction/symlink — ไม่เดินตาม (กัน loop/ออกนอก root)
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".exe") {
				return nil // เก็บเฉพาะ .exe รอบนี้
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return nil
			}
			if _, skipped := skip[cacheKey(abs)]; skipped {
				return nil // registry รายงานไปแล้ว — กันซ้ำ + ประหยัด verify
			}
			out = append(out, softwareFromExe(abs))
			return nil
		})
		// ถ้า walk ถูกตัดเพราะ ctx → คืน error เพื่อให้ scan รอบนี้ถือว่าไม่สำเร็จ
		if walkErr != nil && ctx.Err() != nil {
			return out, ctx.Err()
		}
	}
	return out, nil
}

// scanRoots คืนรายการโฟลเดอร์ที่จะเดิน:
//   - deep mode: ทั้ง system drive (+ extra roots)
//   - default:   curated roots (Program Files / ProgramData / per-user dirs) ข้าม Windows
func scanRoots(opt fsOptions) []string {
	if opt.deep {
		drive := os.Getenv("SystemDrive")
		if drive == "" {
			drive = "C:"
		}
		return append([]string{drive + `\`}, opt.extraRoots...)
	}

	seen := map[string]struct{}{}
	var roots []string
	add := func(p string) {
		if p == "" {
			return
		}
		k := strings.ToLower(p)
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		roots = append(roots, p)
	}

	add(os.Getenv("ProgramFiles"))
	add(os.Getenv("ProgramFiles(x86)"))
	add(os.Getenv("ProgramData"))
	for _, u := range userProfiles() {
		add(filepath.Join(u, `AppData\Local`))
		add(filepath.Join(u, `AppData\Roaming`))
		add(filepath.Join(u, "Desktop"))
		add(filepath.Join(u, "Downloads"))
	}
	for _, r := range opt.extraRoots {
		add(r)
	}
	return roots
}

// userProfiles คืน path ของทุก user profile ใต้ C:\Users (parent ของ %USERPROFILE%)
// ข้าม junction/template ของระบบ; อ่านไม่ได้ → คืนแค่ user ปัจจุบัน
func userProfiles() []string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		return nil
	}
	usersDir := filepath.Dir(profile) // ปกติ C:\Users
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return []string{profile}
	}
	skip := map[string]struct{}{
		"all users": {}, "default": {}, "default user": {}, "public": {},
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, bad := skip[strings.ToLower(e.Name())]; bad {
			continue
		}
		out = append(out, filepath.Join(usersDir, e.Name()))
	}
	if len(out) == 0 {
		return []string{profile}
	}
	return out
}

// underWindowsDir คืน true ถ้า path อยู่ภายใต้ %SystemRoot% (เช่น C:\Windows)
// ใช้กัน extra root / symlink ที่อาจพา walker เข้า Windows ตอน deep=false
func underWindowsDir(path string) bool {
	win := os.Getenv("SystemRoot")
	if win == "" {
		win = `C:\Windows`
	}
	p := strings.ToLower(filepath.Clean(path))
	w := strings.ToLower(filepath.Clean(win))
	return p == w || strings.HasPrefix(p, w+string(filepath.Separator))
}

// isReparse คืน true ถ้า dir entry เป็น symlink หรือ junction (reparse point)
func isReparse(d fs.DirEntry) bool {
	info, err := d.Info()
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if sys, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return sys.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}

// softwareFromExe สร้าง Software entry จากไฟล์ .exe ที่พบ (signature เติมทีหลังใน verify)
func softwareFromExe(abs string) Software {
	info := readPEVersion(abs)
	return Software{
		Name:        info.displayName(baseName(abs)),
		Version:     strings.TrimSpace(info.FileVersion),
		Publisher:   strings.TrimSpace(info.CompanyName),
		InstallPath: abs,
		Arch:        peArch(abs),
		Source:      "filesystem",
	}
}

// peArch อ่าน machine header ของ PE เพื่อระบุสถาปัตยกรรม คืน "" ถ้าอ่านไม่ได้
func peArch(path string) string {
	f, err := pe.Open(path) // #nosec G304 — path มาจากการเดิน filesystem ภายใน
	if err != nil {
		return ""
	}
	defer f.Close()
	return archFromMachine(f.Machine)
}

// archFromMachine map IMAGE_FILE_MACHINE_* เป็น "x64"/"x86" (pure, testable)
func archFromMachine(machine uint16) string {
	switch machine {
	case pe.IMAGE_FILE_MACHINE_AMD64, pe.IMAGE_FILE_MACHINE_ARM64:
		return "x64"
	case pe.IMAGE_FILE_MACHINE_I386, pe.IMAGE_FILE_MACHINE_ARM:
		return "x86"
	default:
		return ""
	}
}
