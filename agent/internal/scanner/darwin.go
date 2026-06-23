//go:build darwin

// build tag นี้บอกให้ Go compiler รวมไฟล์นี้เฉพาะเมื่อ build สำหรับ macOS เท่านั้น

// Package scanner รวมซอฟต์แวร์ที่ติดตั้งอยู่บนเครื่อง macOS
// และตรวจสอบลายเซ็นดิจิทัลของแต่ละแอปพลิเคชัน
package scanner

import (
	"context"       // ใช้สำหรับรองรับ context cancellation ระหว่างการสแกน
	"encoding/json" // ใช้ parse ข้อมูล JSON ที่ได้จาก plutil
	"os"            // ใช้อ่านรายการไฟล์/โฟลเดอร์ในระบบ
	"os/exec"       // ใช้เรียกคำสั่ง external เช่น plutil และ codesign
	"path/filepath" // ใช้สร้าง path ที่ถูกต้องข้ามแพลตฟอร์ม
	"strings"       // ใช้ตรวจสอบและตัดแต่ง string เช่น .app suffix
)

// appDirs คือตำแหน่งที่ macOS เก็บแอปพลิเคชัน GUI ไว้ตามมาตรฐาน (spec 1.3)
// /Applications คือโฟลเดอร์แอปที่ผู้ใช้ติดตั้ง
// /System/Applications คือโฟลเดอร์แอปที่ติดมากับระบบปฏิบัติการ
var appDirs = []string{"/Applications", "/System/Applications"}

// darwinScanner คือ struct ที่ implement interface Scanner สำหรับ macOS
// ไม่มี field ใดเพราะ macOS scanner ไม่ต้องการ state ภายใน
type darwinScanner struct{}

// New คืนค่า Scanner implementation สำหรับ macOS
// ถูกเรียกโดย Run() ใน scanner.go ผ่าน build tag
func New() Scanner { return &darwinScanner{} }

// Scan รวบรวมแอปพลิเคชันที่ติดตั้งอยู่บน macOS พร้อมข้อมูลลายเซ็น
// Parameter:
//   - ctx: context สำหรับยกเลิกการสแกนกลางคันได้
//
// Return:
//   - []Software: รายการซอฟต์แวร์ที่พบ
//   - error: error ถ้า context ถูกยกเลิก
//
// S is resiver Scan is name of FN
//   - report: callback รับความคืบหน้า (nil = ไม่รายงาน) — macOS รายงานแบบ
//     best-effort: นับ .app ก่อน (counting) แล้ว step ต่อแอปที่ verify (scanning)
func (s *darwinScanner) Scan(ctx context.Context, report ProgressFunc) ([]Software, error) {
	tr := newProgressTracker(report)
	var out []Software // ตัวแปรสะสมรายการซอฟต์แวร์ที่พบ

	// PASS 1 (counting): นับ .app bundle ทั้งหมดเพื่อคำนวณ Total
	tr.setPhase(PhaseCounting)
	tr.setTotal(countApps())

	// PASS 2 (scanning): อ่าน plist + verify codesign ต่อแอป พร้อม step
	tr.setPhase(PhaseScanning)
	// วนซ้ำแต่ละโฟลเดอร์แอปพลิเคชันที่กำหนดไว้ใน appDirs
	for _, dir := range appDirs {
		entries, err := os.ReadDir(dir) // อ่านรายการไฟล์/โฟลเดอร์ในไดเรกทอรีนั้น
		if err != nil {
			continue // ถ้าโฟลเดอร์ไม่มีหรืออ่านไม่ได้ ให้ข้ามไป — ไม่ใช่ error ร้ายแรง
		}

		// วนซ้ำแต่ละรายการในโฟลเดอร์
		for _, e := range entries {
			// ตรวจสอบว่า context ถูกยกเลิกหรือไม่ เพื่อหยุดการทำงานทันที
			if err := ctx.Err(); err != nil {
				return nil, err // คืน error เมื่อ context ถูก cancel หรือ timeout
			}

			// ข้ามรายการที่ไม่ใช่ .app bundle
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}

			appPath := filepath.Join(dir, e.Name()) // สร้าง path เต็มของ .app bundle
			tr.step(appPath)

			// อ่านข้อมูลแอปจาก Info.plist และเพิ่มเข้าผลลัพธ์ถ้าสำเร็จ
			if sw, ok := readApp(appPath); ok {
				sw.Signature = codesignVerify(appPath) // ตรวจสอบลายเซ็น codesign ของแอป
				out = append(out, sw)
			}
		}
	}
	return out, nil // คืนรายการซอฟต์แวร์ทั้งหมดที่พบ
}

// countApps นับจำนวน .app bundle ใน appDirs (pass 1) เพื่อใช้เป็น Total ของ progress
func countApps() int {
	n := 0
	for _, dir := range appDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".app") {
				n++
			}
		}
	}
	return n
}

// infoPlist คือ subset ของ Contents/Info.plist ที่เราสนใจเท่านั้น
// ใช้ tag json เพื่อ map กับ key ที่มีชื่อเฉพาะใน plist format
type infoPlist struct {
	Name      string `json:"CFBundleName"`               // ชื่อแอปพลิเคชันตาม Bundle
	Version   string `json:"CFBundleShortVersionString"` // เวอร์ชันที่แสดงผลต่อผู้ใช้
	Copyright string `json:"NSHumanReadableCopyright"`   // ข้อความลิขสิทธิ์ที่อ่านได้
}

// readApp parse ข้อมูลจาก Info.plist ของ .app bundle ผ่านคำสั่ง plutil (มาพร้อมกับ macOS)
// Parameter:
//   - appPath: path เต็มของ .app bundle เช่น /Applications/Safari.app
//
// Return:
//   - Software: ข้อมูลซอฟต์แวร์ที่อ่านได้
//   - bool: true ถ้าอ่านสำเร็จ, false ถ้าล้มเหลว
func readApp(appPath string) (Software, bool) {
	// สร้าง path ไปยัง Info.plist ภายใน .app bundle
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")

	// เรียก plutil เพื่อแปลง plist เป็น JSON แล้วรับผลลัพธ์ผ่าน stdout ("-o -")
	data, err := exec.Command("plutil", "-convert", "json", "-o", "-", plistPath).Output()
	if err != nil {
		return Software{}, false // plutil ล้มเหลว เช่น ไม่มี Info.plist
	}

	var p infoPlist
	// แปลง JSON ที่ได้จาก plutil เป็น struct infoPlist
	if err := json.Unmarshal(data, &p); err != nil {
		return Software{}, false // JSON parse ล้มเหลว — ข้ามรายการนี้
	}

	name := strings.TrimSpace(p.Name) // ตัด whitespace รอบๆ ชื่อแอป
	if name == "" {
		// ถ้า CFBundleName ว่างเปล่า ให้ใช้ชื่อโฟลเดอร์แทน (ตัดนามสกุล .app ออก)
		name = strings.TrimSuffix(filepath.Base(appPath), ".app")
	}

	// สร้าง Software struct จากข้อมูลที่อ่านได้
	return Software{
		Name:        name,
		Version:     strings.TrimSpace(p.Version),   // เวอร์ชันจาก CFBundleShortVersionString
		Publisher:   strings.TrimSpace(p.Copyright), // ผู้เผยแพร่จาก NSHumanReadableCopyright
		InstallPath: appPath,                        // path ของ .app bundle
		Source:      "plist",                        // ระบุว่าข้อมูลมาจาก plist
	}, true
}

// codesignVerify ตรวจสอบลายเซ็นดิจิทัลของแอป macOS ด้วยคำสั่ง codesign
// แปลง exit code ของ codesign เป็น status enum ของเรา (protocol §3 macOS mapping)
// Parameter:
//   - appPath: path เต็มของ .app bundle ที่ต้องการตรวจสอบ
//
// Return:
//   - *Signature: ผลลัพธ์การตรวจสอบลายเซ็น
func codesignVerify(appPath string) *Signature {
	// รัน codesign --verify พร้อม --deep (ตรวจทุกระดับ) และ --strict (เข้มงวด)
	cmd := exec.Command("codesign", "--verify", "--deep", "--strict", appPath)
	combined, err := cmd.CombinedOutput() // รวม stdout และ stderr ไว้ด้วยกัน

	if err == nil {
		// codesign exit code 0 = ลายเซ็นถูกต้อง
		return &Signature{Status: SigValid, Signer: codesignAuthority(appPath)}
	}

	// ตรวจสอบว่า output มีคำว่า "not signed" หรือไม่
	if strings.Contains(string(combined), "not signed") {
		return &Signature{Status: SigUnsigned} // แอปไม่มีลายเซ็น
	}

	// กรณีอื่นๆ ถือว่ามีลายเซ็นแต่ตรวจไม่ผ่าน — best-effort เดาเหตุผลจาก output ของ codesign
	return &Signature{Status: SigInvalid, StatusReason: codesignReason(string(combined))}
}

// codesignReason เดา reason code จากข้อความ codesign (best-effort)
// ข้อความที่ codesign คืนไม่คงรูปแบบเป๊ะ จึงจับเฉพาะ pattern ที่ชัดเจน
// ที่เหลือคืน "other" ให้ dashboard แสดงเป็น "ตรวจสอบไม่ผ่าน" ทั่วไป
// Parameter:
//   - out: รวม stdout+stderr ของ codesign --verify
//
// Return:
//   - string: reason code
func codesignReason(out string) string {
	l := strings.ToLower(out)
	switch {
	case strings.Contains(l, "resource") || strings.Contains(l, "modified") ||
		strings.Contains(l, "main executable failed") || strings.Contains(l, "sealed"):
		return ReasonTampered // ไฟล์/ทรัพยากรถูกแก้ไขหลังลงนาม
	case strings.Contains(l, "not trusted") || strings.Contains(l, "no certificate"):
		return ReasonUntrustedRoot
	default:
		return "other"
	}
}

// codesignAuthority ดึงชื่อ leaf signing authority จากผลลัพธ์ codesign -dvvv
// Parameter:
//   - appPath: path เต็มของ .app bundle
//
// Return:
//   - string: ชื่อ authority เช่น "Apple Inc." หรือ "" ถ้าหาไม่พบ
func codesignAuthority(appPath string) string {
	// รัน codesign -dvvv เพื่อดูรายละเอียดลายเซ็นทั้งหมด (รวม stderr ด้วย)
	out, err := exec.Command("codesign", "-dvvv", appPath).CombinedOutput()
	if err != nil {
		return "" // codesign ล้มเหลว — คืนค่าว่างเปล่า
	}

	// วิเคราะห์ output แต่ละบรรทัดเพื่อหาบรรทัดที่ขึ้นต้นด้วย "Authority="
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(line, "Authority=") {
			// ตัด prefix "Authority=" ออกแล้วคืนชื่อ authority
			return strings.TrimPrefix(strings.TrimSpace(line), "Authority=")
		}
	}
	return "" // ไม่พบ Authority ใน output
}
