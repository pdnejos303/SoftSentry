// Package updater ทดสอบ logic การ self-update ของ agent
// รวมถึง checksum verification, binary swap, และ full Apply flow
package updater

import (
	"context"       // ใช้ส่ง context สำหรับ Apply call ในการทดสอบ
	"crypto/sha256" // ใช้คำนวณ SHA-256 hash ของ test data
	"encoding/hex"  // ใช้แปลง hash bytes เป็น hex string
	"net/http"      // ใช้สร้าง HTTP handler สำหรับ test server
	"net/http/httptest" // ใช้สร้าง in-process HTTP test server
	"os"            // ใช้อ่าน/เขียนไฟล์ test และตรวจสอบไฟล์ที่ต้องลบ
	"path/filepath" // ใช้สร้าง path ของไฟล์ test ใน temp directory
	"strings"       // ใช้ ToUpper สำหรับทดสอบ case-insensitive checksum
	"testing"       // ใช้ testing framework ของ Go

	"github.com/softsentry/agent/internal/transport" // ใช้ transport.New และ AgentUpdate struct
)

// sha256hex คำนวณ SHA-256 ของ byte slice และคืนเป็น lowercase hex string
// ใช้เป็น helper สำหรับสร้างค่า expected checksum ในการทดสอบ
func sha256hex(b []byte) string {
	// คำนวณ hash แบบ one-shot
	s := sha256.Sum256(b)
	// แปลงเป็น hex string
	return hex.EncodeToString(s[:])
}

// TestVerifyChecksum ทดสอบฟังก์ชัน VerifyChecksum ใน 3 กรณี:
// 1. checksum ถูกต้อง (lowercase) - ต้องผ่าน
// 2. checksum ถูกต้อง (uppercase) - ต้องผ่าน (case-insensitive)
// 3. checksum ผิด - ต้องคืน error
func TestVerifyChecksum(t *testing.T) {
	// สร้าง temp directory สำหรับ test ที่ถูกลบอัตโนมัติเมื่อ test จบ
	dir := t.TempDir()
	// กำหนด path ของไฟล์ test
	p := filepath.Join(dir, "f")
	// เขียน test data ลงไฟล์
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// คำนวณ expected checksum ของ "hello"
	want := sha256hex([]byte("hello"))

	// กรณีที่ 1: checksum ตรงกัน (lowercase) ต้องไม่มี error
	if err := VerifyChecksum(p, want); err != nil {
		t.Errorf("matching checksum should pass: %v", err)
	}
	// กรณีที่ 2: checksum เดียวกันแต่เป็น uppercase ต้องไม่มี error (case-insensitive)
	if err := VerifyChecksum(p, strings.ToUpper(want)); err != nil {
		t.Errorf("checksum compare must be case-insensitive: %v", err)
	}
	// กรณีที่ 3: checksum ผิด ต้องคืน error
	if err := VerifyChecksum(p, "deadbeef"); err == nil {
		t.Error("mismatched checksum must return an error")
	}
}

// TestReplaceBinarySwapsAndBacksUp ทดสอบว่า replaceBinary:
// 1. สลับ binary ใหม่เข้ามาแทนที่ binary เก่าได้ถูกต้อง
// 2. ลบ .new ออกหลังสลับสำเร็จ
// 3. เก็บ backup ของ binary เก่าไว้เป็น .old
func TestReplaceBinarySwapsAndBacksUp(t *testing.T) {
	// สร้าง temp directory สำหรับ test
	dir := t.TempDir()
	// กำหนด path ของ binary ปัจจุบัน
	cur := filepath.Join(dir, "agent")
	// กำหนด path ของ binary ใหม่
	nw := cur + ".new"
	// เขียน "OLD" เป็น content ของ binary ปัจจุบัน
	if err := os.WriteFile(cur, []byte("OLD"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	// เขียน "NEW" เป็น content ของ binary ใหม่
	if err := os.WriteFile(nw, []byte("NEW"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	// เรียก replaceBinary เพื่อสลับ binary
	if err := replaceBinary(cur, nw); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}

	// ตรวจสอบว่า binary ปัจจุบันมี content "NEW"
	got, _ := os.ReadFile(cur) //nolint:gosec
	if string(got) != "NEW" {
		t.Errorf("current binary = %q, want NEW", got)
	}
	// ตรวจสอบว่าไฟล์ .new ถูกลบออกหลังสลับสำเร็จ (ไม่ควรมีเหลืออยู่)
	if _, err := os.Stat(nw); !os.IsNotExist(err) {
		t.Errorf(".new should have been consumed, still exists")
	}
	// ตรวจสอบว่าไฟล์ .old มีอยู่และมี content เป็น "OLD" (backup ของ binary เก่า)
	old, err := os.ReadFile(cur + ".old") //nolint:gosec
	if err != nil {
		t.Fatalf(".old backup should exist for rollback: %v", err)
	}
	// ยืนยันว่า backup มี content ของ binary เก่า
	if string(old) != "OLD" {
		t.Errorf(".old backup = %q, want OLD", old)
	}
}

// TestApplyDownloadsVerifiesReplaces ทดสอบ full happy path ของ Apply:
// download binary จาก test server, verify checksum, และสลับ binary สำเร็จ
func TestApplyDownloadsVerifiesReplaces(t *testing.T) {
	// กำหนด content ของ binary ใหม่
	newBin := []byte("NEW-AGENT-BINARY-BYTES")
	// สร้าง HTTP test server ที่ serve binary ที่ /dl
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ถ้า request มายัง /dl ส่ง binary กลับไป
		if r.URL.Path == "/dl" {
			_, _ = w.Write(newBin)
			return
		}
		// path อื่นๆ ตอบ 404
		w.WriteHeader(http.StatusNotFound)
	}))
	// ปิด test server เมื่อ test จบ
	defer srv.Close()

	// สร้าง temp directory สำหรับ test
	dir := t.TempDir()
	// กำหนด path ของ binary ปัจจุบัน
	cur := filepath.Join(dir, "agent")
	// เขียน "OLD" เป็น content ของ binary ปัจจุบัน
	if err := os.WriteFile(cur, []byte("OLD"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	// สร้าง transport client ที่ชี้ไปยัง test server
	client := transport.New(srv.URL, "tok")
	// สร้าง AgentUpdate struct พร้อม checksum ที่ถูกต้อง
	up := transport.AgentUpdate{Version: "0.2.0", DownloadURL: "/dl", SHA256: sha256hex(newBin)}

	// เรียก Apply เพื่อ download และ install binary ใหม่
	if err := Apply(context.Background(), client, cur, up); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// ตรวจสอบว่า binary ปัจจุบันมี content ของ binary ใหม่
	got, _ := os.ReadFile(cur) //nolint:gosec
	if string(got) != string(newBin) {
		t.Errorf("after Apply current = %q, want the downloaded bytes", got)
	}
}

// TestApplyRejectsBadChecksum ทดสอบว่า Apply ปฏิเสธ binary ที่ checksum ไม่ตรง:
// 1. Apply ต้องคืน error
// 2. binary ปัจจุบันต้องไม่ถูกแตะ
// 3. ไฟล์ .new ต้องถูกลบทิ้ง
func TestApplyRejectsBadChecksum(t *testing.T) {
	// สร้าง HTTP test server ที่ส่ง binary ที่ถูก tamper กลับมา
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// ส่ง "TAMPERED" แทนที่จะเป็น binary ที่ถูกต้อง
		_, _ = w.Write([]byte("TAMPERED"))
	}))
	// ปิด test server เมื่อ test จบ
	defer srv.Close()

	// สร้าง temp directory สำหรับ test
	dir := t.TempDir()
	// กำหนด path ของ binary ปัจจุบัน
	cur := filepath.Join(dir, "agent")
	// เขียน "OLD" เป็น content ของ binary ปัจจุบัน
	if err := os.WriteFile(cur, []byte("OLD"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	// สร้าง transport client ที่ชี้ไปยัง test server
	client := transport.New(srv.URL, "tok")
	// สร้าง AgentUpdate ที่มี expected checksum ของ "EXPECTED" (ไม่ตรงกับ "TAMPERED")
	up := transport.AgentUpdate{Version: "0.2.0", DownloadURL: "/x", SHA256: sha256hex([]byte("EXPECTED"))}

	// Apply ต้องล้มเหลวเพราะ checksum ไม่ตรง
	if err := Apply(context.Background(), client, cur, up); err == nil {
		t.Fatal("Apply must fail on checksum mismatch")
	}
	// ตรวจสอบว่า binary ปัจจุบันไม่ถูกเปลี่ยนแปลง (ยังเป็น "OLD")
	got, _ := os.ReadFile(cur) //nolint:gosec
	if string(got) != "OLD" {
		t.Errorf("current binary changed on failed update: %q", got)
	}
	// ตรวจสอบว่าไฟล์ .new ถูกลบทิ้งหลัง checksum ล้มเหลว (cleanup ที่ถูกต้อง)
	if _, err := os.Stat(cur + ".new"); !os.IsNotExist(err) {
		t.Errorf("temporary .new must be cleaned up on failure")
	}
}
