//go:build windows
// build tag นี้ทำให้ไฟล์นี้ถูก compile เฉพาะบน Windows เท่านั้น

// แพ็กเกจ installer จัดการ flow การดาวน์โหลดและติดตั้งตัวเอง
// ไฟล์นี้ให้ implementation เฉพาะ Windows โดยใช้ UAC elevation ผ่าน ShellExecute
package installer

import (
	"fmt"          // ใช้จัดรูปแบบ error message
	"path/filepath" // ใช้ดึง directory ของ executable จาก path
	"strings"      // ใช้ join arguments เป็น string เดียว

	"golang.org/x/sys/windows" // ใช้ Windows API: GetCurrentProcessToken, ShellExecute, UTF16PtrFromString
)

// IsElevated reports whether the current process has an elevated (admin) token.
// (TH) IsElevated รายงานว่าโพรเซสปัจจุบันมี token แบบ elevated (สิทธิ์ผู้ดูแลระบบ) หรือไม่
// ใช้ Windows API เพื่อตรวจสอบ token ของโพรเซสตัวเอง
// คืน true ถ้ารันด้วยสิทธิ์ Administrator, false ถ้าเป็น user ปกติ
func IsElevated() bool {
	// ดึง token ของโพรเซสปัจจุบัน แล้วตรวจสอบว่าเป็น elevated token หรือไม่
	return windows.GetCurrentProcessToken().IsElevated()
}

// RelaunchElevated re-launches exePath with the given args via ShellExecute
// "runas", which raises the Windows UAC consent prompt. It returns once the
// elevated process has been launched (not when it finishes); the caller should
// then exit so only the elevated instance continues.
//
// (TH) RelaunchElevated เปิดโปรแกรมที่ exePath ขึ้นมาใหม่พร้อม args ที่กำหนด
// ผ่าน ShellExecute "runas" ซึ่งจะเรียก UAC consent prompt ของ Windows ขึ้นมา
// ฟังก์ชันนี้จะ return ทันทีหลังโพรเซสที่ elevated ถูกเปิดขึ้น (ไม่รอให้ทำงานเสร็จ)
// ผู้เรียกควรออกจากโปรแกรมหลังจากนี้ เพื่อให้เหลือเพียง instance ที่ elevated ทำงานต่อ
//
// parameter:
//   - exePath: path เต็มของ executable ที่ต้องการ relaunch ด้วยสิทธิ์ admin
//   - args: รายการ arguments ที่จะส่งให้ executable ที่ relaunch
//
// return: error ถ้า ShellExecute ล้มเหลว, nil ถ้าสำเร็จ
func RelaunchElevated(exePath string, args []string) error {
	// แปลง "runas" เป็น UTF-16 pointer สำหรับ Windows API
	// "runas" คือ verb ที่บอก Windows ให้เปิดโปรแกรมด้วยสิทธิ์ admin
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		// ถ้าแปลงสตริงเป็น UTF-16 ไม่ได้ (ไม่ควรเกิดขึ้นกับสตริงคงที่)
		return err
	}
	// แปลง path ของ executable เป็น UTF-16 pointer สำหรับ Windows API
	exe, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		// ถ้า path มีอักขระที่ไม่สามารถแปลงเป็น UTF-16 ได้
		return err
	}
	// argPtr จะเป็น nil ถ้าไม่มี arguments (ShellExecute รับ nil สำหรับ args ว่าง)
	var argPtr *uint16
	// รวม arguments ทั้งหมดเป็นสตริงเดียวคั่นด้วย space (รูปแบบที่ Windows API ต้องการ)
	if joined := strings.Join(args, " "); joined != "" {
		// แปลง arguments string เป็น UTF-16 pointer ถ้ามี argument อย่างน้อยหนึ่งตัว
		if argPtr, err = windows.UTF16PtrFromString(joined); err != nil {
			// ถ้าแปลง arguments เป็น UTF-16 ไม่ได้
			return err
		}
	}
	// ดึง directory ของ executable เพื่อใช้เป็น working directory ของโพรเซสใหม่
	cwd, err := windows.UTF16PtrFromString(filepath.Dir(exePath))
	if err != nil {
		// ถ้าแปลง directory path เป็น UTF-16 ไม่ได้
		return err
	}
	// เรียก ShellExecute เพื่อเปิดโปรแกรมด้วยสิทธิ์ admin
	// parameter: hwnd=0 (ไม่มี parent window), verb="runas", exe=path, args, cwd, showCmd=1
	// showCmd 1 == SW_SHOWNORMAL — เปิดหน้าต่างแบบปกติ (ไม่ minimize ไม่ maximize)
	if err := windows.ShellExecute(0, verb, exe, argPtr, cwd, 1); err != nil {
		// ถ้าผู้ใช้กด "No" บน UAC prompt หรือ ShellExecute ล้มเหลวด้วยเหตุผลอื่น
		return fmt.Errorf("request elevation: %w", err)
	}
	return nil
}
