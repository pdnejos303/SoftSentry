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
	"golang.org/x/sys/windows/registry"
)

// fsOptions คุมพฤติกรรมการเดิน filesystem
type fsOptions struct {
	deep       bool     // true = กวาดทั้ง system drive รวม Windows
	extraRoots []string // root เพิ่มเติมจาก config
}

// collectFilesystem คำนวณ roots จาก options แล้วเดินหา .exe ที่ไม่อยู่ใน skip set
// step (อาจเป็น nil) ถูกเรียกต่อหนึ่ง .exe ที่พบ เพื่อรายงานความคืบหน้า
func collectFilesystem(ctx context.Context, opt fsOptions, skip map[string]struct{}, step func(string)) ([]Software, error) {
	var out []Software
	err := walkExeRoots(ctx, scanRoots(opt), skip, func(abs string) {
		if step != nil {
			step(abs)
		}
		out = append(out, softwareFromExe(abs)) // อ่าน PE version (ส่วนที่แพง) — ทำเฉพาะ pass นี้
	})
	return out, err
}

// countFilesystem นับจำนวน .exe ที่จะถูกเก็บ (pass 1) โดยไม่อ่าน PE/ไม่ verify
// จึงเร็วกว่า collectFilesystem มาก ใช้คำนวณ Total เพื่อแสดง % ที่แท้จริง
func countFilesystem(ctx context.Context, opt fsOptions, skip map[string]struct{}) (int, error) {
	n := 0
	err := walkExeRoots(ctx, scanRoots(opt), skip, func(string) { n++ })
	return n, err
}

// walkExeRoots เดินทุก root ด้วย filepath.WalkDir เรียก visit(abs) ต่อหนึ่ง .exe
// ที่ไม่อยู่ใน skip set best-effort: error ระดับไฟล์/โฟลเดอร์ข้ามไป; ตรวจ ctx
// ทุก 256 entry เพื่อ honor timeout เป็น walker ที่ใช้ร่วมกันทั้ง count และ scan
func walkExeRoots(ctx context.Context, roots []string, skip map[string]struct{}, visit func(abs string)) error {
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
				if underWindowsDir(path) {
					// ข้าม C:\Windows เสมอ (รวม deep mode): เป็นไฟล์ระบบ ไม่ใช่แอป
					// และช่วยลดเวลาสแกนมหาศาลเมื่อเดินทั้งไดรฟ์
					return filepath.SkipDir
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
			visit(abs)
			return nil
		})
		// ถ้า walk ถูกตัดเพราะ ctx → คืน error เพื่อให้ scan รอบนี้ถือว่าไม่สำเร็จ
		if walkErr != nil && ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// scanRoots คืนรายการโฟลเดอร์ที่จะเดิน:
//   - deep mode: ทั้ง system drive (+ extra roots)
//   - default:   curated roots (Program Files / ProgramData / per-user dirs) ข้าม Windows
func scanRoots(opt fsOptions) []string {
	if opt.deep {
		// deep mode = "สแกนทุกที่ในเครื่อง": เดินรากของไดรฟ์ fixed ทุกตัว (C:\, D:\, ...)
		// + extra roots; C:\Windows ถูกข้ามใน walkRoots อยู่แล้ว
		return append(fixedDrives(), opt.extraRoots...)
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

// userProfiles คืน path ของทุก user profile ใต้รากของ profiles (ปกติ C:\Users)
// เดินทุก user เพราะ agent รันเป็น Windows Service (LocalSystem) จึงต้องสแกน AppData
// ของ user จริงทุกคน ไม่ใช่แค่ของบัญชี service เอง
func userProfiles() []string {
	return userProfilesIn(profilesDir())
}

// userProfilesIn เก็บทุกโฟลเดอร์ profile ภายใต้ usersDir ข้าม template/บัญชีระบบ
// อ่าน usersDir ไม่ได้ หรือไม่พบ profile จริง → fallback เป็น %USERPROFILE% (ถ้ามี)
func userProfilesIn(usersDir string) []string {
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return currentProfileFallback()
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
		return currentProfileFallback()
	}
	return out
}

// currentProfileFallback คืน %USERPROFILE% เป็น slice (หรือ nil ถ้าไม่มี)
func currentProfileFallback() []string {
	if profile := os.Getenv("USERPROFILE"); profile != "" {
		return []string{profile}
	}
	return nil
}

// profilesDir คืนรากของ user profiles จริง (ปกติ C:\Users) โดยไม่ขึ้นกับบัญชีที่รัน
// process — %USERPROFILE% ใช้ไม่ได้เมื่อรันเป็น LocalSystem (จะชี้ไป systemprofile)
// แหล่งข้อมูลที่ถูกต้องคือ ProfileList\ProfilesDirectory; ถ้าอ่านไม่ได้ fallback เป็น
// %SystemDrive%\Users
func profilesDir() string {
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`,
		registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if dir := strings.TrimRight(regString(k, "ProfilesDirectory"), `\`); dir != "" {
			return dir
		}
	}
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	return filepath.Join(drive+`\`, "Users")
}

// fixedDrives คืน root ของไดรฟ์ชนิด fixed ทุกตัว (C:\, D:\, ...) ใช้ตอน deep mode
// เพื่อสแกน "ทุกที่ในเครื่อง" โดยข้ามไดรฟ์ removable/network/CD-ROM เพื่อไม่ให้
// สแกนค้างกับสื่อที่ถอดได้หรือไดรฟ์เครือข่าย ถ้าหาไม่เจอเลย fallback เป็น SystemDrive
func fixedDrives() []string {
	var roots []string
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if windows.GetDriveType(p) == windows.DRIVE_FIXED {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		drive := os.Getenv("SystemDrive")
		if drive == "" {
			drive = "C:"
		}
		roots = append(roots, drive+`\`)
	}
	return roots
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
