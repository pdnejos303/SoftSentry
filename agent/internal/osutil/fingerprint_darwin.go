//go:build darwin
// build tag นี้บอก Go compiler ว่าไฟล์นี้ compile เฉพาะบน macOS เท่านั้น

package osutil

import (
	"context"     // ใช้สร้าง context พร้อม timeout เพื่อจำกัดเวลารันคำสั่ง ioreg
	"os/exec"     // ใช้รันคำสั่ง external (ioreg) เพื่อดึง hardware UUID
	"strings"     // ใช้ตัด/ค้นหา substring ในผลลัพธ์ ioreg
	"time"        // ใช้กำหนด timeout duration ให้กับ context
)

// detectFingerprint reads the macOS IOPlatformUUID — a stable per-machine
// hardware id — via `ioreg`. Mirrors the Windows MachineGuid role: the server
// dedups re-enrollment on it. Any failure returns "" so enrollment still works.
//
// (TH) อ่านค่า IOPlatformUUID ของ macOS ซึ่งเป็น hardware id ต่อเครื่องที่คงที่
// ผ่านคำสั่ง `ioreg` ทำหน้าที่เดียวกับ MachineGuid บน Windows: เซิร์ฟเวอร์ใช้ค่านี้
// dedup การ enroll ซ้ำ ถ้าล้มเหลวจะคืน "" เพื่อให้ enroll ยังทำงานได้
func detectFingerprint() string {
	// สร้าง context พร้อม timeout 5 วินาที กันคำสั่งค้าง
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// รัน `ioreg -rd1 -c IOPlatformExpertDevice` ซึ่งพิมพ์บรรทัดที่มี "IOPlatformUUID"
	out, err := exec.CommandContext(ctx, "ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return ""
	}
	return parseIOPlatformUUID(string(out))
}

// parseIOPlatformUUID extracts the UUID value from ioreg output, where the line
// looks like: `    "IOPlatformUUID" = "XXXXXXXX-...."`.
// (TH) ดึงค่า UUID จากผลลัพธ์ ioreg โดยบรรทัดมีรูปแบบ
// `    "IOPlatformUUID" = "XXXXXXXX-...."`
func parseIOPlatformUUID(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "IOPlatformUUID") {
			continue
		}
		// แยกค่าหลังเครื่องหมาย '=' แล้วตัด whitespace และ double-quote ออก
		_, value, found := strings.Cut(line, "=")
		if !found {
			return ""
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}
