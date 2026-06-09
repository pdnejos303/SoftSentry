// Package runner — ไฟล์นี้ทดสอบ loop.maybeUpdate ซึ่งเป็นหัวใจของการแก้ปัญหา
// "online then exits forever" (agent รีสตาร์ทวนไม่รู้จบหลัง self-update)
// ครอบคลุม: loop-breaker (SHA ซ้ำ), nil update, auto-update ปิด, version เดิม
package runner

import (
	"context"  // ใช้ context.Background() สำหรับ call ที่ไม่ต้องการ timeout ใน test
	"io"       // ใช้ io.Discard เพื่อทิ้ง output ทั้งหมดใน test loop
	"testing"  // ใช้ t.Fatal, t.Error และ t.Setenv สำหรับ test infrastructure

	"github.com/softsentry/agent/internal/storage"   // ใช้ SaveLastUpdate/LoadLastUpdate เพื่อ seed และตรวจสอบ loop-breaker marker
	"github.com/softsentry/agent/internal/transport" // ใช้ AgentUpdate struct เป็น input ให้ maybeUpdate
)

// useTempConfigDir ชี้ storage (ผ่าน config.Dir) ไปที่ตำแหน่งชั่วคราว
// เพื่อให้ loop-breaker marker (last_update) ถูกอ่าน/เขียนภายใต้ไดเรกทอรีของเทสต์เอง
// และไม่ปนกับข้อมูล config จริงของผู้ใช้บนเครื่อง
func useTempConfigDir(t *testing.T) {
	t.Helper() // บอก testing framework ว่านี่คือ helper function (เพื่อให้ stack trace ชัดขึ้น)
	dir := t.TempDir() // สร้างไดเรกทอรีชั่วคราวที่จะถูกลบอัตโนมัติเมื่อ test จบ
	t.Setenv("ProgramData", dir) // override บน Windows: config อยู่ใต้ %ProgramData%\SoftSentry
	t.Setenv("HOME", dir)        // override บน macOS/Linux: config อยู่ใต้ $HOME/.softsentry
	t.Setenv("USERPROFILE", dir) // override ค่า fallback ของ UserHomeDir บน Windows
}

// newTestLoop สร้าง loop instance สำหรับใช้ในการทดสอบ
// ใช้ HTTP client ที่ชี้ไปยัง address ที่ไม่มีจริง (127.0.0.1:0) เพื่อจำลอง network
// และ discard output ทั้งหมดเพื่อไม่ให้ test output รก
func newTestLoop() *loop {
	return &loop{
		client:     transport.New("http://127.0.0.1:0", ""), // client ที่จะ fail ทุก network call (ไม่มี server จริง)
		out:        io.Discard,                              // ทิ้ง output ปกติ
		errw:       io.Discard,                              // ทิ้ง error output
		autoUpdate: true,                                    // เปิด auto-update (สามารถ override ใน test ที่ต้องการ)
		exePath:    "/tmp/softsentry-agent",                 // path ปลอมสำหรับ self-update (ไม่ถูก apply จริงใน test)
		version:    "0.1.0",                                 // เวอร์ชันปัจจุบันของ agent ใน test
	}
}

// TestMaybeUpdateSkipsAlreadyAppliedSHA ทดสอบ "loop-breaker" หลัก:
// หัวใจของการแก้ปัญหา "online then exits forever"
//
// สถานการณ์: เซิร์ฟเวอร์ยังคงเสนอไบนารีที่ agent ติดตั้งไปแล้ว (SHA-256 ตรงกัน)
// แต่เวอร์ชันที่รายงานไม่เคยขยับเลย (manifest version > binary version)
// maybeUpdate ต้อง "ปฏิเสธ" การติดตั้งซ้ำ ไม่อย่างนั้นทุกครั้งที่รีสตาร์ทจะ
// ดาวน์โหลดไบนารีตัวเดิมซ้ำวนไม่รู้จบ
func TestMaybeUpdateSkipsAlreadyAppliedSHA(t *testing.T) {
	// ชี้ config directory ไปที่ temp dir เพื่อไม่ปนกับข้อมูลจริง
	useTempConfigDir(t)
	// Seed last_update marker ด้วย SHA "DEADBEEF" จำลองว่าเพิ่งติดตั้งไบนารีนี้ไป
	if err := storage.SaveLastUpdate("DEADBEEF"); err != nil {
		t.Fatalf("seed last update: %v", err) // ถ้า seed ล้มเหลวทดสอบต่อไม่ได้
	}
	r := newTestLoop() // สร้าง loop instance สำหรับ test
	// สร้าง update offer ที่มี SHA เดียวกัน (lowercase "deadbeef") กับที่บันทึกไว้
	// ต้องเปรียบเทียบแบบ case-insensitive (EqualFold) เพราะ hex อาจ upper/lower ปนกัน
	up := &transport.AgentUpdate{Version: "2.0.0", SHA256: "deadbeef", DownloadURL: "/x"}
	// เรียก maybeUpdate และตรวจสอบว่าคืนค่า false (ข้ามการติดตั้งซ้ำ)
	if r.maybeUpdate(context.Background(), up) {
		t.Fatal("maybeUpdate re-applied an already-installed binary (restart loop not broken)")
	}
}

// TestMaybeUpdateNilOrDisabled ทดสอบสองกรณีที่ maybeUpdate ต้องคืนค่า false ทันที:
// 1. up เป็น nil (ไม่มีการเสนออัปเดต)
// 2. autoUpdate เป็น false (ผู้ดูแลระบบปิด auto-update)
func TestMaybeUpdateNilOrDisabled(t *testing.T) {
	// ชี้ config directory ไปที่ temp dir
	useTempConfigDir(t)
	r := newTestLoop()
	// กรณีที่ 1: up เป็น nil — ไม่มีอะไรต้องทำ ต้องเป็น no-op
	if r.maybeUpdate(context.Background(), nil) {
		t.Error("maybeUpdate(nil) should be a no-op")
	}
	// กรณีที่ 2: ปิด auto-update แล้วส่ง update offer ที่ valid
	r.autoUpdate = false // ปิด auto-update
	up := &transport.AgentUpdate{Version: "2.0.0", SHA256: "abc"}
	// ต้องไม่ apply update เพราะ autoUpdate = false
	if r.maybeUpdate(context.Background(), up) {
		t.Error("maybeUpdate should not run when auto-update is disabled")
	}
}

// TestMaybeUpdateSkipsSameVersion ทดสอบว่า maybeUpdate ข้ามการ update
// เมื่อเวอร์ชันที่เสนอเหมือนกับเวอร์ชันที่กำลังรันอยู่
// ป้องกันการ download ไฟล์โดยไม่จำเป็นเมื่อ binary ปัจจุบันถูกต้องแล้ว
func TestMaybeUpdateSkipsSameVersion(t *testing.T) {
	// ชี้ config directory ไปที่ temp dir
	useTempConfigDir(t)
	r := newTestLoop() // r.version = "0.1.0"
	// สร้าง update offer ที่มีเวอร์ชันเดียวกับที่กำลังรัน (r.version = "0.1.0")
	up := &transport.AgentUpdate{Version: "0.1.0", SHA256: "abc"} // == r.version
	// ต้องไม่ apply update เพราะ version เดิมอยู่แล้ว
	if r.maybeUpdate(context.Background(), up) {
		t.Error("maybeUpdate should not update to the version already running")
	}
}
