//go:build !windows && !darwin
// build tag นี้บอก Go compiler ว่าไฟล์นี้ compile เฉพาะบนแพลตฟอร์ม
// ที่ไม่ใช่ Windows และไม่ใช่ macOS (เช่น Linux) เพื่อให้ cross-compile ได้

package osutil

// detectFingerprint has no real implementation on unsupported platforms (the
// agent targets Windows and macOS); it returns "" so the build stays
// cross-compilable and the server simply falls back to always-create.
//
// (TH) บนแพลตฟอร์มที่ไม่รองรับ (agent ตั้งเป้าไว้ที่ Windows และ macOS)
// detectFingerprint จะคืน "" เพื่อให้ build ข้ามแพลตฟอร์มได้ และเซิร์ฟเวอร์
// จะกลับไปสร้าง record ใหม่เสมอ
func detectFingerprint() string {
	return ""
}
