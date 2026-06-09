//go:build windows
// build tag นี้บอกให้ Go compiler รวมไฟล์นี้เฉพาะเมื่อ build สำหรับ Windows เท่านั้น

// Package scanner รวบรวมซอฟต์แวร์ที่ติดตั้งอยู่บน Windows
// โดยอ่านจาก registry hive ที่เก็บข้อมูล "Programs and Features"
package scanner

import (
	"context" // ใช้สำหรับรองรับ context cancellation ระหว่างการสแกน
	"strings" // ใช้ตัดแต่ง string และตรวจสอบนามสกุลไฟล์
	"time"    // ใช้แปลงและตรวจสอบวันที่ติดตั้ง

	"golang.org/x/sys/windows/registry" // ใช้เข้าถึง Windows Registry API
)

// uninstallHive ระบุตำแหน่งหนึ่งของ "Programs and Features" ใน registry
// Windows มีหลาย hive ที่เก็บข้อมูลซอฟต์แวร์ติดตั้งไว้
type uninstallHive struct {
	root registry.Key // root key ของ registry เช่น HKLM หรือ HKCU
	path string       // sub-path ภายใต้ root ที่เก็บข้อมูล Uninstall
	arch string       // สถาปัตยกรรม (x64/x86) ที่กำหนดให้รายการใน hive นี้
}

// hives คือรายการตำแหน่ง registry ที่ Windows บันทึกซอฟต์แวร์ที่ติดตั้งไว้
// (spec 1.3 / protocol §3)
// WOW6432Node เก็บแอป 32-bit บน OS 64-bit
// HKCU เก็บแอปที่ติดตั้งเฉพาะ user ปัจจุบัน
// รายการซ้ำที่พบในหลาย hive จะถูกกำจัดโดย dedupe() ในภายหลัง
var hives = []uninstallHive{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "x64"},          // แอป 64-bit ระดับ system
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, "x86"}, // แอป 32-bit บน OS 64-bit
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "x64"},           // แอปที่ติดตั้งเฉพาะ user ปัจจุบัน
}

// windowsScanner คือ Scanner implementation สำหรับ Windows
// มี verifier ที่ใช้ตรวจสอบลายเซ็น Authenticode ของไฟล์ .exe
type windowsScanner struct {
	verifier *authenticodeVerifier // ตัวตรวจสอบลายเซ็น Authenticode พร้อม cache
}

// New สร้างและคืนค่า Windows software scanner
// พร้อม authenticodeVerifier ที่ initialized แล้ว
func New() Scanner { return &windowsScanner{verifier: newAuthenticodeVerifier()} }

// Scan รวบรวมซอฟต์แวร์จากทุก registry hive และตรวจสอบลายเซ็นของ .exe ที่พบ
// Parameter:
//   - ctx: context สำหรับยกเลิกการสแกนกลางคันได้
//
// Return:
//   - []Software: รายการซอฟต์แวร์ทั้งหมดที่พบ (อาจมีซ้ำ, ถูก dedupe ภายหลัง)
//   - error: error ถ้า context ถูกยกเลิก
func (s *windowsScanner) Scan(ctx context.Context) ([]Software, error) {
	var all []Software // slice สะสมซอฟต์แวร์จากทุก hive

	// วนซ้ำแต่ละ hive เพื่ออ่านรายการซอฟต์แวร์
	for _, h := range hives {
		// ตรวจสอบ context ก่อนอ่าน hive แต่ละตัว เพื่อหยุดทันทีถ้าถูก cancel
		if err := ctx.Err(); err != nil {
			return nil, err // context ถูกยกเลิก — หยุดและคืน error
		}
		all = append(all, s.scanHive(h)...) // เพิ่มซอฟต์แวร์จาก hive นี้เข้า slice รวม
	}

	// ตรวจสอบลายเซ็น Authenticode สำหรับรายการที่มี path ของ executable
	for i := range all {
		// ตรวจสอบ context ก่อนตรวจสอบลายเซ็นแต่ละรายการ
		if err := ctx.Err(); err != nil {
			return nil, err // context ถูกยกเลิก — หยุดทันที
		}

		// ข้ามรายการที่ไม่มี InstallPath เพราะไม่มีไฟล์ให้ตรวจสอบ
		if all[i].InstallPath == "" {
			continue
		}

		// ตรวจสอบลายเซ็น Authenticode ของ executable นั้น
		if sig := s.verifier.verify(all[i].InstallPath); sig != nil {
			all[i].Signature = sig // บันทึกผลการตรวจสอบลายเซ็นเข้า Software
		}
	}
	return all, nil // คืนรายการซอฟต์แวร์ทั้งหมด
}

// scanHive อ่าน subkey ทุก uninstall key ภายใต้ hive ที่กำหนด
// ข้ามรายการที่ไม่มี DisplayName หรือถูก flag ว่าเป็น OS component (spec 1.3)
// Parameter:
//   - h: hive ที่ต้องการสแกน
//
// Return:
//   - []Software: รายการซอฟต์แวร์ที่พบใน hive นี้
func (s *windowsScanner) scanHive(h uninstallHive) []Software {
	// เปิด registry key ด้วยสิทธิ์ READ เท่านั้น
	root, err := registry.OpenKey(h.root, h.path, registry.READ)
	if err != nil {
		return nil // hive ไม่มีอยู่ (เช่น ไม่มี WOW6432Node บน 32-bit OS) — ไม่ใช่ error
	}
	defer root.Close() // ปิด registry key เมื่อฟังก์ชันนี้จบ

	// อ่านชื่อ subkey ทั้งหมด (-1 = อ่านทุกตัว)
	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil // อ่าน subkey ล้มเหลว — คืน slice ว่าง
	}

	out := make([]Software, 0, len(names)) // pre-allocate slice ด้วยขนาดเท่า subkey
	for _, name := range names {
		// parse subkey แต่ละตัวเป็น Software struct ถ้าสำเร็จให้เพิ่มเข้า slice
		if sw, ok := parseEntry(root, name, h.arch); ok {
			out = append(out, sw)
		}
	}
	return out // คืนรายการซอฟต์แวร์ที่พบใน hive นี้
}

// parseEntry อ่าน uninstall subkey หนึ่ง key และแปลงเป็น Software struct
// คืน false เพื่อข้ามรายการถ้า DisplayName ว่างเปล่าหรือเป็น SystemComponent
// Parameter:
//   - root: registry key พ่อแม่ที่เปิดแล้ว
//   - subkey: ชื่อ subkey ที่ต้องการอ่าน
//   - arch: สถาปัตยกรรมที่กำหนดให้รายการนี้
//
// Return:
//   - Software: ข้อมูลซอฟต์แวร์ที่อ่านได้
//   - bool: true ถ้า parse สำเร็จ, false ถ้าควรข้ามรายการนี้
func parseEntry(root registry.Key, subkey, arch string) (Software, bool) {
	// เปิด subkey ด้วยสิทธิ์ QUERY_VALUE เท่านั้น (อ่าน value)
	k, err := registry.OpenKey(root, subkey, registry.QUERY_VALUE)
	if err != nil {
		return Software{}, false // เปิด subkey ไม่สำเร็จ — ข้ามรายการนี้
	}
	defer k.Close() // ปิด subkey เมื่อ parse เสร็จ

	// ตรวจสอบว่าเป็น OS component หรือไม่ (SystemComponent=1 = ข้ามไป)
	if sc, _, err := k.GetIntegerValue("SystemComponent"); err == nil && sc == 1 {
		return Software{}, false // เป็น OS component — ไม่ต้องรายงาน
	}

	// อ่านชื่อแอป — ถ้าว่างเปล่าให้ข้ามเพราะไม่ใช่รายการที่มีประโยชน์
	name := strings.TrimSpace(regString(k, "DisplayName"))
	if name == "" {
		return Software{}, false // ไม่มีชื่อแสดงผล — ข้ามรายการนี้
	}

	// สร้าง Software struct จากค่าที่อ่านได้จาก registry
	sw := Software{
		Name:        name,
		Version:     strings.TrimSpace(regString(k, "DisplayVersion")),         // เวอร์ชันที่แสดงผล
		Publisher:   strings.TrimSpace(regString(k, "Publisher")),               // ชื่อผู้พัฒนา
		InstallDate: normalizeInstallDate(regString(k, "InstallDate")),           // วันที่ติดตั้งในรูปแบบ YYYY-MM-DD
		InstallPath: resolveExecutable(k),                                        // path ของ executable หลัก
		Arch:        arch,                                                        // สถาปัตยกรรม x64 หรือ x86
		Source:      "registry",                                                  // ระบุว่าข้อมูลมาจาก registry
	}

	// อ่านขนาดการติดตั้งถ้ามีค่าอยู่ (EstimatedSize unit คือ KB อยู่แล้ว)
	if sz, _, err := k.GetIntegerValue("EstimatedSize"); err == nil {
		sw.InstallSizeKB = int64(sz) // EstimatedSize เป็น KB อยู่แล้ว ไม่ต้องแปลง
	}
	return sw, true // parse สำเร็จ — คืน Software พร้อม flag true
}

// regString อ่านค่า string จาก registry key
// รองรับทั้ง REG_SZ และ REG_EXPAND_SZ (expand environment variables)
// ถ้าไม่มี value หรืออ่านไม่สำเร็จจะคืนค่าว่างเปล่า
// Parameter:
//   - k: registry key ที่เปิดแล้ว
//   - name: ชื่อ value ที่ต้องการอ่าน
//
// Return:
//   - string: ค่าที่อ่านได้ หรือ "" ถ้าไม่มีหรืออ่านไม่สำเร็จ
func regString(k registry.Key, name string) string {
	if v, _, err := k.GetStringValue(name); err == nil {
		// พยายาม expand environment variable เช่น %ProgramFiles% → C:\Program Files
		if expanded, err := registry.ExpandString(v); err == nil {
			return expanded // คืนค่าหลัง expand
		}
		return v // ถ้า expand ล้มเหลว คืนค่าดิบ
	}
	return "" // ไม่มี value นี้หรืออ่านไม่สำเร็จ
}

// resolveExecutable เลือก filesystem path ที่จะใช้ตรวจสอบลายเซ็น
// ให้ความสำคัญกับ DisplayIcon (มักเป็น .exe หลัก) ก่อน แล้วจึง InstallLocation
// Parameter:
//   - k: registry key ของแอปที่ต้องการ
//
// Return:
//   - string: path ของ executable หรือ path ของโฟลเดอร์ติดตั้ง
func resolveExecutable(k registry.Key) string {
	if icon := regString(k, "DisplayIcon"); icon != "" {
		// DisplayIcon มักอยู่ในรูป "C:\path\app.exe,0" — ต้องตัด icon index ออก
		if idx := strings.LastIndex(icon, ","); idx > 1 {
			icon = icon[:idx] // ตัดส่วน ",0" หรือ ",<n>" ที่ท้ายออก
		}
		icon = strings.Trim(icon, `"`) // ตัด quote รอบๆ path ออก
		// ใช้ path นี้เฉพาะถ้าเป็นไฟล์ .exe เท่านั้น
		if strings.HasSuffix(strings.ToLower(icon), ".exe") {
			return icon // คืน path ของ .exe
		}
	}
	// ถ้าไม่มี DisplayIcon ที่ใช้ได้ ใช้ InstallLocation แทน (อาจเป็นโฟลเดอร์)
	return strings.TrimSpace(regString(k, "InstallLocation"))
}

// normalizeInstallDate แปลงวันที่จาก registry รูปแบบ YYYYMMDD เป็น YYYY-MM-DD
// คืนค่าว่างเปล่าสำหรับวันที่ที่ไม่ถูกต้องหรืออยู่นอกช่วงที่สมเหตุสมผล
// เครื่องที่ใช้ locale ไทยบางครั้ง installer เขียนปี พ.ศ. หรือข้อมูลไม่ถูกต้อง
// เช่น "25674421" ซึ่ง backend จะปฏิเสธ
// Parameter:
//   - raw: สตริงวันที่ดิบจาก registry
//
// Return:
//   - string: วันที่ในรูปแบบ YYYY-MM-DD หรือ "" ถ้าไม่ถูกต้อง
func normalizeInstallDate(raw string) string {
	raw = strings.TrimSpace(raw) // ตัด whitespace รอบๆ ก่อน

	// วันที่ YYYYMMDD ต้องมีความยาวพอดี 8 ตัวอักษร
	if len(raw) != 8 {
		return "" // ความยาวไม่ถูกต้อง — ไม่ใช่วันที่ YYYYMMDD
	}

	// แปลง string เป็น time.Time ด้วย layout "20060102" (รูปแบบ YYYYMMDD ของ Go)
	t, err := time.Parse("20060102", raw)
	if err != nil {
		return "" // parse ล้มเหลว — ไม่ใช่วันที่ที่ถูกต้อง
	}

	// ตรวจสอบว่าปีอยู่ในช่วงที่สมเหตุสมผล (1980 ถึงปัจจุบัน+1)
	// ป้องกันปี พ.ศ. หรือข้อมูล corrupt เช่น ปี 2567 ของไทย
	if t.Year() < 1980 || t.Year() > time.Now().Year()+1 {
		return "" // ปีอยู่นอกช่วงที่สมเหตุสมผล — อาจเป็นปี พ.ศ. หรือข้อมูลผิด
	}

	return t.Format("2006-01-02") // แปลงเป็น YYYY-MM-DD ที่ backend ต้องการ
}
