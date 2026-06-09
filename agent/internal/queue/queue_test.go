// Package queue — ไฟล์นี้ทดสอบการทำงานของ queue บัฟเฟอร์ retry แบบถาวร
// ครอบคลุมพฤติกรรมหลัก: enqueue, due, remove, reschedule, cap enforcement,
// persistence, backoff calculation, และ key uniqueness
package queue

import (
	"testing" // ใช้ t.Fatal, t.Errorf และ t.TempDir สำหรับ test infrastructure
	"time"    // ใช้จัดการเวลาในการทดสอบ backoff และ due time

	"github.com/softsentry/agent/internal/transport" // ใช้ ScanRequest เป็น payload ตัวอย่างในการทดสอบ
)

// sampleReq สร้าง ScanRequest ตัวอย่างที่มีค่าคงที่ เพื่อใช้ในการทดสอบต่างๆ
// trigger คือค่าที่กำหนดประเภทของการสแกน เช่น "manual" หรือ "auto"
func sampleReq(trigger string) transport.ScanRequest {
	return transport.ScanRequest{
		StartedAt:   "2026-05-30T00:00:00Z",                                         // เวลาเริ่มสแกน (ค่าคงที่สำหรับ test)
		CompletedAt: "2026-05-30T00:00:10Z",                                         // เวลาสิ้นสุดสแกน (ใช้เวลา 10 วินาที)
		ScanType:    "auto",                                                          // ประเภทการสแกนเป็น auto เสมอในตัวอย่างนี้
		Trigger:     trigger,                                                         // ค่าที่รับมาจากผู้เรียก เพื่อแยกแยะ test case
		Software:    []transport.SoftwareItem{{Name: "Demo", Version: "1.0"}},       // รายการซอฟต์แวร์ตัวอย่าง 1 รายการ
	}
}

// TestEnqueueThenDueReturnsItem ทดสอบว่าเมื่อ enqueue item แล้ว
// การเรียก Due() ด้วยเวลาที่ผ่าน backoff ไปแล้วต้องคืนค่า item นั้นกลับมาพร้อมข้อมูลครบถ้วน
func TestEnqueueThenDueReturnsItem(t *testing.T) {
	// เปิดคิวใหม่ในไดเรกทอรีชั่วคราวที่ test framework สร้างให้
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err) // ถ้าเปิดคิวไม่ได้ถือว่า test นี้ fail ทันที
	}
	// ใส่ item เข้าคิวด้วย trigger "manual" และ idempotency key "key-1"
	if err := q.Enqueue(sampleReq("manual"), "key-1"); err != nil {
		t.Fatalf("enqueue: %v", err) // ถ้า enqueue ล้มเหลวถือว่า test fail
	}
	// ความพยายามครั้งแรกถูกตั้งเวลาให้ห่างออกไป 1 วินาที (backoff หลังจาก POST ล้มเหลว)
	// เรียก Due() ด้วยเวลาที่เลยไปแล้ว 2 วินาที เพื่อให้ item นี้ถูกรวมในผลลัพธ์
	due, err := q.Due(time.Now().Add(2 * time.Second))
	if err != nil {
		t.Fatalf("due: %v", err) // ถ้าอ่านคิวล้มเหลวถือว่า test fail
	}
	// ต้องได้ item กลับมาพอดี 1 รายการ
	if len(due) != 1 {
		t.Fatalf("want 1 due item, got %d", len(due))
	}
	// ตรวจสอบว่าข้อมูลใน item ตรงกับที่ enqueue ไป (IdempotencyKey และ Trigger)
	if due[0].IdempotencyKey != "key-1" || due[0].Request.Trigger != "manual" {
		t.Fatalf("round-trip mismatch: %+v", due[0])
	}
}

// TestDueRespectsBackoffSchedule ทดสอบว่า Due() ไม่คืน item กลับมา
// ก่อนที่จะถึงเวลา backoff ที่กำหนดไว้ (ป้องกัน premature retry)
func TestDueRespectsBackoffSchedule(t *testing.T) {
	// เปิดคิวใหม่ในไดเรกทอรีชั่วคราว
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// ใส่ item เข้าคิว (NextAttempt จะถูกตั้งให้ห่างออกไป 1 วินาทีจากตอนนี้)
	if err := q.Enqueue(sampleReq("auto"), "k"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// ที่ t=0 (ปัจจุบัน) item ยังไม่ถึงเวลา retry (ตั้งเวลาไว้ห่างออกไป 1 วินาที)
	// Due() ต้องคืนค่า slice ว่าง
	if due, _ := q.Due(time.Now()); len(due) != 0 {
		t.Fatalf("item should not be due immediately, got %d", len(due))
	}
}

// TestMaxQueueDropsOldest ทดสอบว่าเมื่อ enqueue เกิน maxItems รายการ
// คิวจะตัดรายการเก่าสุดทิ้งเพื่อรักษาขนาดไว้ที่ maxItems (FIFO policy)
func TestMaxQueueDropsOldest(t *testing.T) {
	// เปิดคิวใหม่ในไดเรกทอรีชั่วคราว
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// ใส่ item เกิน limit ไป 5 รายการ (maxItems + 5 รายการรวม)
	for i := range maxItems + 5 {
		req := sampleReq("auto")
		// กำหนด StartedAt ที่ต่างกันเพื่อให้แต่ละ item มี timestamp ไม่ซ้ำกัน
		req.StartedAt = time.Now().Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		if err := q.Enqueue(req, ""); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		time.Sleep(time.Millisecond) // รอให้ creation timestamp ต่างกัน เพื่อรักษาลำดับ FIFO
	}
	// ตรวจสอบว่าความลึกของคิวถูกจำกัดไว้ที่ maxItems
	depth, err := q.Depth()
	if err != nil {
		t.Fatalf("depth: %v", err)
	}
	// ความลึกต้องเท่ากับ maxItems พอดี
	if depth != maxItems {
		t.Fatalf("want depth capped at %d, got %d", maxItems, depth)
	}
}

// TestRemoveDeletesItem ทดสอบว่า Remove() ลบ item ออกจากคิวได้จริง
// และคิวว่างเปล่าหลังจากลบ item เดียวที่มีอยู่ออก
func TestRemoveDeletesItem(t *testing.T) {
	// เปิดคิวใหม่ในไดเรกทอรีชั่วคราว
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// ใส่ item เข้าคิว 1 รายการ (ไม่ต้องตรวจสอบ error เพราะ test อื่นครอบคลุมแล้ว)
	_ = q.Enqueue(sampleReq("auto"), "")
	// ดึง item ที่ due (ใช้เวลา +1 ชั่วโมงเพื่อให้ทุก item ถือว่า due แล้ว)
	due, _ := q.Due(time.Now().Add(time.Hour))
	if len(due) != 1 {
		t.Fatalf("expected 1 item")
	}
	// ลบ item ออกจากคิวด้วย ID ของมัน
	if err := q.Remove(due[0].ID); err != nil {
		t.Fatalf("remove: %v", err) // ถ้าลบล้มเหลวถือว่า test fail
	}
	// หลังลบแล้ว คิวต้องว่างเปล่า (depth = 0)
	if d, _ := q.Depth(); d != 0 {
		t.Fatalf("want empty after remove, got %d", d)
	}
}

// TestRescheduleAdvancesBackoff ทดสอบว่า Reschedule() เพิ่มจำนวน attempt
// และเลื่อนเวลา retry ออกไปตาม exponential backoff ได้ถูกต้อง
func TestRescheduleAdvancesBackoff(t *testing.T) {
	// เปิดคิวใหม่ในไดเรกทอรีชั่วคราว
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// ใส่ item เข้าคิว 1 รายการ (attempt=1, NextAttempt = now+1s)
	_ = q.Enqueue(sampleReq("auto"), "")
	now := time.Now()
	// ดึง item ที่ due ด้วยเวลา +1 ชั่วโมง เพื่อให้ได้ item กลับมา
	due, _ := q.Due(now.Add(time.Hour))
	id := due[0].ID // เก็บ ID ไว้ใช้ใน Reschedule
	// เรียก Reschedule เพื่อเพิ่ม attempt เป็น 2 และตั้ง NextAttempt = now + backoff(2) = now + 2s
	if err := q.Reschedule(id, now); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	// จำนวนครั้งเปลี่ยนจาก 1 → 2 ทำให้ NextAttempt = now + 2s
	// ที่ now+1s item ยังไม่ถึงเวลา retry (ต้องรอถึง now+2s)
	if d, _ := q.Due(now.Add(time.Second)); len(d) != 0 {
		t.Fatalf("rescheduled item should not be due at +1s")
	}
	// ที่ now+3s item ต้องถึงเวลา retry แล้ว และ Attempts ต้องเป็น 2
	got, _ := q.Due(now.Add(3 * time.Second))
	if len(got) != 1 || got[0].Attempts != 2 {
		t.Fatalf("want 1 item with attempts=2, got %+v", got)
	}
}

// TestPersistenceAcrossReopen ทดสอบว่า item ที่ enqueue ไว้ยังคงอยู่
// แม้จะสร้าง Queue instance ใหม่จากไดเรกทอรีเดิม (จำลองการ restart process)
func TestPersistenceAcrossReopen(t *testing.T) {
	// ใช้ไดเรกทอรีชั่วคราวเดิม (ไม่สร้างใหม่) เพื่อจำลอง persistence ข้ามการ reopen
	dir := t.TempDir()
	// เปิดคิวแรก ใส่ item เข้าไป
	q1, _ := Open(dir)
	_ = q1.Enqueue(sampleReq("auto"), "persist") // ใช้ idempotency key "persist" เพื่อตรวจสอบ
	// เปิดคิวใหม่จากไดเรกทอรีเดิม (จำลองว่า process รีสตาร์ท)
	q2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	// ดึง item ด้วยเวลา +1 ชั่วโมง เพื่อให้ได้ item กลับมาแน่นอน
	due, _ := q2.Due(time.Now().Add(time.Hour))
	// ต้องได้ item กลับมา 1 รายการ และ IdempotencyKey ต้องตรงกับที่ enqueue ไว้
	if len(due) != 1 || due[0].IdempotencyKey != "persist" {
		t.Fatalf("item did not survive reopen: %+v", due)
	}
}

// TestBackoffCapsAtFiveMinutes ทดสอบว่าฟังก์ชัน backoff คำนวณค่าได้ถูกต้อง
// สำหรับจำนวน attempt ต่างๆ และไม่เกิน 5 นาที (maxBackoff)
func TestBackoffCapsAtFiveMinutes(t *testing.T) {
	// ตารางค่าที่คาดหวัง: attempt → duration
	cases := map[int]time.Duration{
		1:  1 * time.Second, // attempt=1: 1s (baseBackoff * 2^0)
		2:  2 * time.Second, // attempt=2: 2s (baseBackoff * 2^1)
		3:  4 * time.Second, // attempt=3: 4s (baseBackoff * 2^2)
		10: 5 * time.Minute, // attempt=10: 2^9 s = 512s > cap → ใช้ maxBackoff
		50: 5 * time.Minute, // attempt=50: ต้องไม่ overflow และต้องได้ maxBackoff
	}
	// วนทดสอบทุก case
	for attempts, want := range cases {
		if got := backoff(attempts); got != want {
			t.Errorf("backoff(%d) = %v, want %v", attempts, got, want) // รายงานทุก case ที่ fail โดยไม่หยุด
		}
	}
}

// TestNewKeyIsUniqueAndNonEmpty ทดสอบว่า NewKey() คืนค่าสตริงที่ไม่ว่าง
// และสองการเรียกติดต่อกันได้ค่าที่แตกต่างกัน
func TestNewKeyIsUniqueAndNonEmpty(t *testing.T) {
	// สร้าง key 2 ตัวติดกัน
	a, b := NewKey(), NewKey()
	// ทั้งคู่ต้องไม่เป็นสตริงว่าง
	if a == "" || b == "" {
		t.Fatal("NewKey returned empty")
	}
	// ทั้งคู่ต้องไม่เท่ากัน (ไม่ซ้ำกัน)
	if a == b {
		t.Fatal("NewKey not unique")
	}
}
