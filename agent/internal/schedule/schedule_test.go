// Package schedule รวม unit test สำหรับ logic การจัดตาราง schedule ของการสแกน
package schedule

import (
	"testing" // ใช้ testing.T สำหรับเขียน unit test ใน Go
	"time"    // ใช้ time.Duration และ time.Time ในการสร้าง test case
)

// TestInterval ทดสอบฟังก์ชัน Interval ว่า clamp ค่าได้ถูกต้องในทุกกรณีขอบ
func TestInterval(t *testing.T) {
	// กำหนด test case ทั้งหมดในรูปแบบ table-driven test
	cases := []struct {
		name  string        // ชื่อ test case อธิบายสถานการณ์ที่ทดสอบ
		hours int           // ค่า input จำนวนชั่วโมงที่ส่งเข้าฟังก์ชัน
		want  time.Duration // ค่า output ที่คาดหวังว่าจะได้รับกลับมา
	}{
		{"default when zero", 0, 6 * time.Hour},              // ค่า 0 ต้อง fallback เป็น default 6 ชั่วโมง
		{"default when negative", -3, 6 * time.Hour},         // ค่าติดลบต้อง fallback เป็น default 6 ชั่วโมง
		{"clamp below min", 0, 6 * time.Hour},                 // 0 -> default, ทดสอบซ้ำเพื่อยืนยัน
		{"min", 1, 1 * time.Hour},                             // ค่าต่ำสุดที่ถูกต้อง = 1 ชั่วโมง
		{"typical", 6, 6 * time.Hour},                         // ค่าปกติที่ใช้งานทั่วไป = 6 ชั่วโมง
		{"max", 24, 24 * time.Hour},                           // ค่าสูงสุดที่ถูกต้อง = 24 ชั่วโมง
		{"clamp above max", 48, 24 * time.Hour},               // ค่าเกิน max ต้อง clamp ลงมาที่ 24 ชั่วโมง
	}
	// วนลูปผ่านทุก test case
	for _, tc := range cases {
		// รัน sub-test แต่ละตัวแยกกัน ทำให้ระบุ failure ได้ชัดเจน
		t.Run(tc.name, func(t *testing.T) {
			// เรียก Interval ด้วยค่า input แล้วตรวจสอบว่าได้ผลลัพธ์ตามที่คาดหวัง
			if got := Interval(tc.hours); got != tc.want {
				t.Errorf("Interval(%d) = %s, want %s", tc.hours, got, tc.want)
			}
		})
	}
}

// TestDueIn ทดสอบฟังก์ชัน DueIn ว่าคำนวณเวลาที่เหลือก่อนสแกนถัดไปได้ถูกต้อง
func TestDueIn(t *testing.T) {
	// กำหนดเวลาปัจจุบันที่ใช้ในการทดสอบ (fixed time เพื่อให้ test reproducible)
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	// กำหนด interval สำหรับทดสอบ = 6 ชั่วโมง
	interval := 6 * time.Hour

	// ทดสอบกรณียังไม่เคยสแกนเลย (lastScan เป็น zero value)
	t.Run("never scanned is due now", func(t *testing.T) {
		// ส่ง zero time เป็น lastScan คาดว่าจะได้ 0 หมายถึงสแกนเดี๋ยวนี้
		if got := DueIn(time.Time{}, interval, now); got != 0 {
			t.Errorf("DueIn(zero) = %s, want 0", got)
		}
	})

	// ทดสอบกรณี interval ผ่านไปนานกว่าที่กำหนดแล้ว (เลยกำหนดไปแล้ว)
	t.Run("interval elapsed is due now", func(t *testing.T) {
		// สแกนครั้งล่าสุดเมื่อ 7 ชั่วโมงที่แล้ว เลย interval 6h ไปแล้ว
		last := now.Add(-7 * time.Hour)
		// คาดว่าได้ 0 หมายถึงถึงเวลาสแกนแล้ว
		if got := DueIn(last, interval, now); got != 0 {
			t.Errorf("DueIn(elapsed) = %s, want 0", got)
		}
	})

	// ทดสอบกรณีถึงเวลาสแกนพอดี (ผ่านมาครบ interval พอดี)
	t.Run("exactly due is due now", func(t *testing.T) {
		// สแกนครั้งล่าสุดเมื่อ 6 ชั่วโมงที่แล้ว ตรงกับ interval พอดี
		last := now.Add(-6 * time.Hour)
		// คาดว่าได้ 0 หมายถึงถึงเวลาพอดีแล้ว
		if got := DueIn(last, interval, now); got != 0 {
			t.Errorf("DueIn(exact) = %s, want 0", got)
		}
	})

	// ทดสอบกรณียังไม่ถึงเวลาสแกน ต้องรอต่อ
	t.Run("not yet due returns remaining time", func(t *testing.T) {
		// สแกนครั้งล่าสุดเมื่อ 2 ชั่วโมงที่แล้ว ยังเหลืออีก 4 ชั่วโมงจึงจะครบ interval
		last := now.Add(-2 * time.Hour) // เหลือ 4h อีก
		want := 4 * time.Hour           // ค่าที่คาดหวัง = 4 ชั่วโมง
		// ตรวจสอบว่า DueIn คืนค่าเวลาที่เหลือถูกต้อง
		if got := DueIn(last, interval, now); got != want {
			t.Errorf("DueIn(recent) = %s, want %s", got, want)
		}
	})
}
