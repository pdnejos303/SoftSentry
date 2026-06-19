//go:build !windows

// บน macOS/Linux ไฟล์ที่กำลังรันถูกลบได้ทันที (ไม่ถูกล็อกแบบ Windows) จึงลบ
// ทั้งโฟลเดอร์ได้ตรงๆ
package installer

import "os"

// RemoveInstallDir ลบโฟลเดอร์ติดตั้งทั้งหมด
func RemoveInstallDir(dir string) error { return os.RemoveAll(dir) }
