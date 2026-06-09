//go:build darwin
// build tag นี้บอก Go compiler ว่าไฟล์นี้ compile เฉพาะบน macOS เท่านั้น

package osutil

import (
	"context"  // ใช้สร้าง context พร้อม timeout เพื่อจำกัดเวลารันคำสั่ง sw_vers
	"os/exec"  // ใช้รันคำสั่ง external เช่น sw_vers เพื่อดึงเวอร์ชัน macOS
	"time"     // ใช้กำหนด timeout duration ให้กับ context
)

// detectOSVersion shells out to `sw_vers -productVersion`, the canonical way to
// read the macOS product version (e.g. "14.5"). Any failure degrades to
// fallbackVersion rather than failing enrollment.
//
// (TH) เรียกคำสั่ง `sw_vers -productVersion` ซึ่งเป็นวิธีมาตรฐานในการอ่านเวอร์ชัน
// ผลิตภัณฑ์ของ macOS (เช่น "14.5") หากล้มเหลวจะลดระดับลงไปใช้ fallbackVersion
// แทนที่จะทำให้การ enroll ล้มเหลว
//
// Returns: string เวอร์ชัน macOS เช่น "14.5" หรือ fallbackVersion ถ้าเกิด error
func detectOSVersion() string {
	// สร้าง context พร้อม timeout 5 วินาที เพื่อป้องกันการค้างถ้า sw_vers ไม่ตอบสนอง
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// เรียก cancel เสมอเมื่อ function จบ เพื่อปล่อย resource ของ context
	defer cancel()

	// รันคำสั่ง sw_vers -productVersion โดยใช้ context ที่มี timeout
	// Output() คืน stdout เป็น []byte หรือ error ถ้าคำสั่งล้มเหลว
	out, err := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output()
	// ถ้ารันคำสั่งไม่สำเร็จ (เช่น timeout หรือ sw_vers ไม่มี) ให้ใช้ fallback
	if err != nil {
		return fallbackVersion
	}
	// แปลง []byte เป็น string แล้วส่งให้ parseSwVers ทำความสะอาดและ validate
	return parseSwVers(string(out))
}
