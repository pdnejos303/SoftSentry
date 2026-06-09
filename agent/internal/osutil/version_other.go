//go:build !windows && !darwin
// build tag นี้บอก Go compiler ว่าไฟล์นี้ compile เฉพาะบนแพลตฟอร์ม
// ที่ไม่ใช่ Windows และไม่ใช่ macOS (เช่น Linux) เพื่อให้ cross-compile ได้

package osutil

// detectOSVersion has no real implementation on unsupported platforms (the
// agent targets Windows and macOS); it returns the generic fallback so the
// build stays cross-compilable.
//
// (TH) บนแพลตฟอร์มที่ไม่รองรับ (agent ตั้งเป้าไว้ที่ Windows และ macOS)
// detectOSVersion จะไม่มี implementation จริง มันจะคืนค่า fallback ทั่วไปแทน
// เพื่อให้ build ข้ามแพลตฟอร์มได้
//
// Returns: fallbackVersion เสมอ เพราะแพลตฟอร์มนี้ไม่ใช่ target จริงของ agent
func detectOSVersion() string {
	// คืน fallback โดยตรงเพราะแพลตฟอร์มนี้ไม่มีวิธีดึงเวอร์ชัน OS ที่ implement ไว้
	return fallbackVersion
}
