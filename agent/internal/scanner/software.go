// Package scanner แจกแจงซอฟต์แวร์ที่ติดตั้งอยู่บนเครื่องและตรวจสอบลายเซ็นดิจิทัล
// การรวบรวมข้อมูลเฉพาะแพลตฟอร์มอยู่ในไฟล์ที่มี build tag (windows.go, darwin.go)
// ไฟล์นี้เก็บ type ที่ใช้ร่วมกันและ helper function (dedupe, sort)
package scanner

import "sort" // ใช้ฟังก์ชันเรียงลำดับมาตรฐานของ Go

// SignatureStatus คือ enum ที่แสดงสถานะลายเซ็นดิจิทัลของซอฟต์แวร์
// ค่าเหล่านี้ต้องตรงกับที่ backend รับได้ (ดู backend app/schemas/scans.py SignatureStatus)
type SignatureStatus string

const (
	SigValid    SignatureStatus = "valid"    // ลายเซ็นถูกต้องและยังไม่หมดอายุ
	SigExpired  SignatureStatus = "expired"  // ลายเซ็นหมดอายุแล้ว (cert NotAfter ผ่านไปแล้ว)
	SigInvalid  SignatureStatus = "invalid"  // ลายเซ็นไม่ถูกต้อง เช่น ไฟล์ถูกแก้ไขหลังลงนาม
	SigUnsigned SignatureStatus = "unsigned" // ไฟล์ไม่มีลายเซ็นดิจิทัล
)

// Signature เก็บผลลัพธ์การตรวจสอบลายเซ็น Authenticode (Windows) หรือ codesign (macOS)
type Signature struct {
	Status     SignatureStatus `json:"status"`                    // สถานะลายเซ็น: valid/expired/invalid/unsigned
	Signer     string          `json:"signer,omitempty"`          // ชื่อผู้ลงนาม (leaf certificate CN)
	Issuer     string          `json:"issuer,omitempty"`          // ชื่อ CA ที่ออกใบรับรอง (issuer CN)
	Thumbprint string          `json:"cert_thumbprint,omitempty"` // fingerprint ของใบรับรอง (hex SHA-1)
}

// Software คือข้อมูลแอปพลิเคชันที่ติดตั้งอยู่บนเครื่อง ตามที่ agent ตรวจพบ
// ถูกส่งไปยัง backend ในรูปแบบ JSON
type Software struct {
	Name          string     `json:"name"`                        // ชื่อซอฟต์แวร์ที่แสดงในระบบ
	Version       string     `json:"version"`                     // เวอร์ชันของซอฟต์แวร์
	Publisher     string     `json:"publisher,omitempty"`         // ผู้พัฒนาหรือผู้เผยแพร่ซอฟต์แวร์
	InstallDate   string     `json:"install_date,omitempty"`      // วันที่ติดตั้งในรูปแบบ YYYY-MM-DD
	InstallPath   string     `json:"install_path,omitempty"`      // path ของไฟล์ executable หลัก
	InstallSizeKB int64      `json:"install_size_kb,omitempty"`   // ขนาดการติดตั้งเป็น kilobytes
	Arch          string     `json:"arch,omitempty"`              // สถาปัตยกรรม: x64 หรือ x86
	Source        string     `json:"source"`                      // แหล่งที่มาของข้อมูล: registry, appstore, หรือ plist
	Signature     *Signature `json:"signature,omitempty"`         // ผลการตรวจสอบลายเซ็นดิจิทัล (nil ถ้ายังไม่ตรวจ)
}

// dedupe ลบรายการซ้ำที่มี (name, version, install_path) เหมือนกันออก
// Windows registry มักรายงาน product เดียวกันซ้ำจาก WOW6432Node และ per-user hive (spec 1.3)
// รายการแรกที่พบจะถูกเก็บไว้ รายการซ้ำต่อมาจะถูกทิ้ง
// Parameter:
//   - in: รายการ Software ทั้งหมดที่อาจมีซ้ำ
//
// Return:
//   - []Software: รายการที่ไม่มีซ้ำแล้ว
func dedupe(in []Software) []Software {
	// สร้าง map ใช้ triple (name, version, install_path) เป็น key เพื่อติดตามรายการที่เห็นแล้ว
	seen := make(map[[3]string]struct{}, len(in))
	out := make([]Software, 0, len(in)) // pre-allocate slice ด้วยขนาดเท่า input เพื่อประสิทธิภาพ

	// วนซ้ำทุกรายการและข้ามรายการที่เคยเห็นแล้ว
	for _, s := range in {
		key := [3]string{s.Name, s.Version, s.InstallPath} // สร้าง key จาก 3 ฟิลด์
		if _, ok := seen[key]; ok {
			continue // key นี้เคยเห็นแล้ว — ข้ามรายการซ้ำนี้
		}
		seen[key] = struct{}{} // บันทึก key นี้ว่าเคยเห็นแล้ว
		out = append(out, s)   // เพิ่มรายการแรกที่พบเข้าผลลัพธ์
	}
	return out // คืนรายการที่ไม่มีซ้ำ
}

// sortStable เรียงซอฟต์แวร์ตามชื่อ (Name) ก่อน แล้วตามเวอร์ชัน (Version)
// ทำให้ผลลัพธ์การสแกนมีลำดับแน่นอนทุกครั้ง (deterministic output)
// ใช้ SliceStable เพื่อรักษาลำดับเดิมของรายการที่มี key เท่ากัน
// Parameter:
//   - in: รายการ Software ที่ต้องการเรียงลำดับ (เรียงในที่อยู่เดิม in-place)
func sortStable(in []Software) {
	// เรียงลำดับโดยเปรียบ Name ก่อน ถ้าชื่อเหมือนกันให้เปรียบ Version
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Name != in[j].Name {
			return in[i].Name < in[j].Name // เรียงตามชื่อ A→Z
		}
		return in[i].Version < in[j].Version // ถ้าชื่อเหมือนกัน เรียงตามเวอร์ชัน
	})
}
