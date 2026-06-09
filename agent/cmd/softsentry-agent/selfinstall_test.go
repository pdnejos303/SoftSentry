// ไฟล์นี้มี unit test สำหรับ self-install flow โดยเฉพาะ:
//   - installDisclosure: ตรวจสอบว่าข้อความ consent ครอบคลุมข้อเท็จจริงสำคัญครบ
//   - confirmInstall: ตรวจสอบว่ารับ "y"/"yes" และปฏิเสธทุกอย่างอื่น (รวมถึง Enter เฉยๆ)
//
// test เหล่านี้ไม่ต้องการ OS service หรือ filesystem จริง เพราะฟังก์ชันที่ test
// ถูกออกแบบมาเป็น pure function ที่รับ io.Reader / io.Writer แทน
package main

import (
	"bytes"   // ใช้สร้าง in-memory buffer สำหรับ capture output ใน test
	"strings" // ใช้ตรวจสอบว่า output มี substring ที่ต้องการหรือไม่, และสร้าง test input

	"testing" // Go standard testing framework

	"github.com/softsentry/agent/internal/installer" // ใช้ประเภท installer.Embedded สำหรับ test fixture
)

// testEmbedded คืน installer.Embedded แบบ fixed สำหรับใช้เป็น test fixture
// ใช้ค่าที่รู้ชัดเจนเพื่อให้ assert ผลลัพธ์ได้แน่นอน
func testEmbedded() installer.Embedded {
	return installer.Embedded{
		// ตั้งค่า Config ด้วยค่า test ที่รู้ชัดเจน
		Config: installer.Config{
			ServerURL: "http://192.168.1.88:8001", // server URL ที่คาดว่าจะแสดงใน disclosure
			Token:     "secret-token",             // token ที่ต้องไม่ถูกแสดงใน disclosure
		},
	}
}

// The disclosure must state, in plain language, every material fact a user
// needs to give informed consent: where it installs, the admin permission, the
// server it reports to, what it collects, and how to remove it.
//
// (TH) ข้อมูลเปิดเผยต้องระบุด้วยภาษาธรรมดาในทุกข้อเท็จจริงสำคัญที่ผู้ใช้ต้องรู้
// เพื่อให้ความยินยอมแบบรับทราบข้อมูล: ติดตั้งที่ไหน ต้องใช้สิทธิ์ผู้ดูแลระบบ
// รายงานไปยังเซิร์ฟเวอร์ใด เก็บข้อมูลอะไรบ้าง และวิธีถอดถอน
func TestInstallDisclosureStatesKeyFacts(t *testing.T) {
	// สร้าง disclosure text จาก test fixture
	text := installDisclosure(testEmbedded())
	// ตรวจสอบว่า disclosure มีข้อมูลสำคัญทุกอย่างครบ
	for _, want := range []string{
		"http://192.168.1.88:8001", // where it reports — (TH) ต้องแสดง server URL ที่จะรายงานไป
		"Administrator",            // permission it will ask for — (TH) ต้องระบุว่าต้องใช้สิทธิ์ Administrator
		"installed software",       // what it collects — (TH) ต้องระบุว่าเก็บรายการซอฟต์แวร์
		"uninstall",                // how to remove it — (TH) ต้องบอกวิธีถอดถอน
	} {
		// ถ้า disclosure ไม่มี substring นี้ ถือว่า test ล้มเหลว
		if !strings.Contains(text, want) {
			t.Errorf("disclosure missing %q\n---\n%s", want, text)
		}
	}
}

// The deployment token is a secret and must never be printed to the user.
// (TH) deployment token เป็นความลับ ห้ามแสดงให้ผู้ใช้เห็นเด็ดขาด —
// ถ้า token รั่วใน disclosure ผู้ไม่หวังดีที่เห็นหน้าจออาจนำไปใช้ enroll
// เครื่องอื่นโดยไม่ได้รับอนุญาต
func TestInstallDisclosureHidesToken(t *testing.T) {
	// ตรวจสอบว่า disclosure ไม่มี "secret-token" อยู่เลย
	if strings.Contains(installDisclosure(testEmbedded()), "secret-token") {
		t.Error("disclosure leaked the deployment token")
	}
}

// TestConfirmInstallAccepts ตรวจสอบว่า confirmInstall ยอมรับ input ทุกรูปแบบที่
// เป็น "ใช่" — "y", "Y", "yes", "yes" พร้อมช่องว่าง, "YES\r\n" (Windows line ending)
func TestConfirmInstallAccepts(t *testing.T) {
	// ทดสอบทุก variant ของ "ใช่" ที่ควรได้รับการยอมรับ
	for _, in := range []string{"y\n", "Y\n", "yes\n", "  yes \n", "YES\r\n"} {
		var out bytes.Buffer // buffer สำหรับ capture output (ไม่ใช้ค่า แต่ต้องส่งให้ฟังก์ชัน)
		// ยืนยันว่า confirmInstall คืน true สำหรับ input "ใช่"
		if !confirmInstall(&out, strings.NewReader(in), testEmbedded()) {
			t.Errorf("input %q should accept install", in)
		}
	}
}

// Anything other than an explicit yes cancels — informed consent must be
// affirmative, so a bare Enter or empty input does NOT install.
//
// (TH) สิ่งใดก็ตามที่ไม่ใช่ "ใช่" อย่างชัดเจนจะยกเลิก — การยินยอมแบบรับทราบ
// ข้อมูลต้องเป็นคำตอบเชิงบวกเท่านั้น การกด Enter เฉยๆ หรือไม่ป้อนอะไรเลยจะ
// "ไม่" ติดตั้ง — ออกแบบโดยเจตนาเพื่อป้องกันการติดตั้งโดยบังเอิญ
func TestConfirmInstallDeclines(t *testing.T) {
	// ทดสอบทุก input ที่ควรถูกปฏิเสธ รวมถึง Enter เฉยๆ, "n", "no", "maybe"
	for _, in := range []string{"\n", "n\n", "no\n", "x\n", "", "maybe\n"} {
		var out bytes.Buffer // buffer สำหรับ capture output
		// ยืนยันว่า confirmInstall คืน false สำหรับ input ที่ไม่ใช่ "ใช่"
		if confirmInstall(&out, strings.NewReader(in), testEmbedded()) {
			t.Errorf("input %q should cancel install", in)
		}
	}
}

// The prompt itself must be shown so the user knows a decision is required.
// (TH) ต้องแสดง prompt ให้เห็น เพื่อให้ผู้ใช้รู้ว่าต้องตัดสินใจ —
// test นี้ตรวจสอบว่า disclosure ถูกแสดงจริง โดยเช็คว่า server URL
// (ซึ่งอยู่ใน disclosure) ปรากฏอยู่ใน output
func TestConfirmInstallShowsPrompt(t *testing.T) {
	var out bytes.Buffer // buffer สำหรับ capture output ที่ confirmInstall เขียน
	// เรียก confirmInstall พร้อม input "n" (ปฏิเสธ) เพื่อ capture output
	confirmInstall(&out, strings.NewReader("n\n"), testEmbedded())
	// ตรวจสอบว่า output มี server URL จาก disclosure (บ่งชี้ว่า disclosure ถูกแสดง)
	if !strings.Contains(out.String(), "http://192.168.1.88:8001") {
		t.Error("confirm prompt did not render the disclosure")
	}
}
