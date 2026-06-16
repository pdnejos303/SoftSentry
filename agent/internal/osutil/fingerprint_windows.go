//go:build windows
// build tag นี้บอก Go compiler ว่าไฟล์นี้ compile เฉพาะบน Windows เท่านั้น

package osutil

import "golang.org/x/sys/windows/registry" // ใช้อ่านค่า MachineGuid จาก Windows Registry

// detectFingerprint reads the Windows MachineGuid — a stable, per-install GUID
// at HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid that survives reboots and
// agent re-installs. The server uses it to map a re-enrolling machine back onto
// its existing row instead of creating a duplicate. Any read failure returns ""
// (no fingerprint → server falls back to always-create), so enrollment never
// fails just because we couldn't read it.
//
// (TH) อ่านค่า MachineGuid ของ Windows — GUID ต่อการติดตั้งหนึ่งครั้งที่อยู่ที่
// HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid คงที่ข้าม reboot และการ
// ติดตั้ง agent ใหม่ เซิร์ฟเวอร์ใช้ค่านี้เพื่อจับคู่เครื่องที่ enroll ซ้ำกลับไปยัง
// record เดิมแทนการสร้างซ้ำ ถ้าอ่านไม่ได้จะคืน "" (ไม่มี fingerprint → เซิร์ฟเวอร์
// กลับไปสร้าง record ใหม่เสมอ) เพื่อไม่ให้การ enroll ล้มเหลวเพราะอ่านค่านี้ไม่ได้
func detectFingerprint() string {
	// เปิด registry key ของ Cryptography ภายใต้ HKEY_LOCAL_MACHINE แบบอ่านอย่างเดียว
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		// เปิด key ไม่ได้ (permission/ไม่มี key) → ไม่มี fingerprint
		return ""
	}
	// ปิด handle เสมอเมื่อ function จบ เพื่อกัน handle leak
	defer k.Close()

	// อ่านค่า MachineGuid เป็น string เช่น "a1b2c3d4-...."; error → คืน "" ผ่าน guid ว่าง
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return guid
}
