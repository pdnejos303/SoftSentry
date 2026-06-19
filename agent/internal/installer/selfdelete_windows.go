//go:build windows

// ไฟล์นี้ลบโฟลเดอร์ติดตั้งบน Windows ทั้งที่ไบนารีของเราเองยังรันอยู่ในนั้น
// Windows ล็อกไฟล์ .exe ที่กำลังรัน ลบตรงๆ ไม่ได้ จึง (1) ลบทุกอย่างที่ลบได้ก่อน
// แล้ว (2) ปล่อย cmd /c แบบ detached ให้รอจน process เราออก (ปลดล็อก) แล้วค่อย rd
// ทิ้งส่วนที่เหลือ — ผู้ใช้จึงเห็นโฟลเดอร์หายไปภายในไม่กี่วินาทีโดยไม่ต้อง reboot
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// RemoveInstallDir ลบ dir แบบ best-effort ในโพรเซส แล้วตั้ง sweeper ลบส่วนที่ล็อก
func RemoveInstallDir(dir string) error {
	// ลบทุกไฟล์ที่ไม่ได้ถูกล็อก (config สำรอง, ไฟล์ .old-*, ฯลฯ) ทันที — ไม่ fatal
	_ = os.RemoveAll(dir)
	// ถ้าหายหมดแล้ว (ไม่มีไฟล์ถูกล็อก) ไม่ต้องตั้ง sweeper
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	// ปล่อย cmd /c: รอ ~2 วินาทีด้วย ping (ไม่พึ่ง timeout ที่ต้อง console) แล้ว rd
	// /s /q เพื่อกวาดโฟลเดอร์ที่ยังถูกไบนารีเราล็อกอยู่ หลังจาก process นี้ออก
	cmd := exec.Command("cmd", "/C",
		fmt.Sprintf(`ping 127.0.0.1 -n 3 >nul & rd /s /q "%s"`, dir))
	// แยกหน้าต่าง: ไม่ผูกกับ console ของเรา และรันต่อได้หลังเราตาย
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008} // DETACHED_PROCESS
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("schedule install-dir cleanup: %w", err)
	}
	// ไม่ Wait — ปล่อยให้ทำงานเองหลังเราออก
	return nil
}
