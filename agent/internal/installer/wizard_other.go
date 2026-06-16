//go:build !windows

// ไฟล์นี้เป็น stub ของ wizard GUI สำหรับแพลตฟอร์มที่ไม่ใช่ Windows (macOS/Linux)
// wizard แบบหลายหน้าจอเป็นฟีเจอร์เฉพาะ Windows — บนแพลตฟอร์มอื่น ฟังก์ชันเหล่านี้
// รายงานว่า "แสดง GUI ไม่ได้" (shown=false) เพื่อให้ self-installer ตกไปใช้ console flow
package installer

// DefaultLang นอก Windows คืนค่าไทยเสมอ (ไม่มี API ตรวจภาษา UI ที่ portable)
func DefaultLang() Lang { return LangTH }

// RunWizard นอก Windows คืน shown=false เสมอ → ผู้เรียกใช้ console disclosure แทน
func RunWizard(_ string, _ Lang, _ string) (WizardResult, bool) {
	return WizardResult{}, false
}

// RunInstallProgress นอก Windows คืน shown=false เสมอ → ผู้เรียกแสดง progress ผ่าน console
func RunInstallProgress(_ Lang, _ func(setStep func(int)) error) (error, bool) {
	return nil, false
}
