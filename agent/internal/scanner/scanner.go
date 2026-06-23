// scanner.go คือจุดเริ่มต้นที่ไม่ขึ้นกับแพลตฟอร์ม (platform-agnostic entry point)
// ฟังก์ชัน New() ถูกกำหนดไว้ในไฟล์ที่มี build tag (windows.go / darwin.go / unsupported.go)
// และคืนค่า Scanner implementation ที่เหมาะกับ OS นั้นๆ
// ส่วน type ที่ใช้ร่วมกันและ helper สำหรับประมวลผลภายหลัง (dedupe, sort) อยู่ใน software.go
//
// (TH) scanner.go คือจุดเริ่มต้นที่ไม่ขึ้นกับแพลตฟอร์ม ฟังก์ชัน New() ถูก
// กำหนดไว้ในไฟล์ที่มี build tag (windows.go / darwin.go / unsupported.go) และ
// คืนค่า Scanner implementation ที่เหมาะกับ OS นั้นๆ ส่วน type ที่ใช้ร่วมกัน
// และ helper สำหรับประมวลผลภายหลัง (dedupe, sort) อยู่ใน software.go

// Package scanner แจกแจงซอฟต์แวร์ที่ติดตั้งอยู่บนเครื่อง host
// และตรวจสอบลายเซ็นดิจิทัลของไฟล์ executable
package scanner

import (
	"context" // ใช้สำหรับรองรับ context cancellation ในระหว่างการสแกน
	"time"    // ใช้บันทึกเวลาเริ่มต้นและสิ้นสุดการสแกน
)

// Result คือผลลัพธ์ของการสแกน inventory หนึ่งครั้ง
// เก็บข้อมูลเวลาเริ่ม-สิ้นสุดและรายการซอฟต์แวร์ที่พบ
// (TH) Result คือผลลัพธ์ของการสแกน inventory หนึ่งครั้ง
type Result struct {
	StartedAt  time.Time  // เวลาที่เริ่มต้นการสแกน
	FinishedAt time.Time  // เวลาที่การสแกนเสร็จสมบูรณ์
	Software   []Software // รายการซอฟต์แวร์ทั้งหมดที่พบในการสแกน
}

// Duration คำนวณและคืนค่าระยะเวลาที่ใช้ในการสแกน
// โดยลบ StartedAt ออกจาก FinishedAt
// (TH) Duration คือระยะเวลาที่ใช้ในการสแกน
func (r Result) Duration() time.Duration { return r.FinishedAt.Sub(r.StartedAt) }

// Scanner คือ interface สำหรับแจกแจงซอฟต์แวร์ที่ติดตั้งอยู่บนเครื่อง host
// การเลือก implementation ทำตอน compile time ผ่าน build tags
// (ดู windows.go / darwin.go / unsupported.go)
// (TH) Scanner แจกแจงรายการซอฟต์แวร์ที่ติดตั้งอยู่ในเครื่อง การเลือก
// implementation ทำตอน compile time ผ่าน build tags (ดู windows.go / darwin.go)
type Scanner interface {
	// Scan รวบรวมซอฟต์แวร์ที่ติดตั้งพร้อมข้อมูลลายเซ็นดิจิทัล
	// ทำงานแบบ best-effort: รายการที่ล้มเหลวจะถูกข้ามไป ไม่ถือเป็น fatal error
	// Parameter:
	//   - ctx: context สำหรับยกเลิกการสแกนกลางคัน
	// Return:
	//   - []Software: รายการซอฟต์แวร์ที่พบ
	//   - error: error ถ้า context ถูกยกเลิกหรือเกิดข้อผิดพลาดร้ายแรง
	//
	// (TH) รวบรวมซอฟต์แวร์ที่ติดตั้งพร้อมข้อมูลลายเซ็น ทำแบบ best-effort:
	// รายการที่ล้มเหลวจะถูกข้ามไป ไม่ถือเป็น fatal
	//   - report: callback รับความคืบหน้า (nil = ไม่รายงาน)
	Scan(ctx context.Context, report ProgressFunc) ([]Software, error)
}

// Run คือฟังก์ชันสาธารณะจุดเดียวที่ caller ควรใช้
// ขั้นตอนการทำงาน:
//  1. เรียก New() เพื่อรับ Scanner ที่เหมาะกับแพลตฟอร์มปัจจุบัน
//  2. ลบรายการซ้ำที่มี (name, version, install_path) เหมือนกัน
//  3. เรียงผลลัพธ์ตามชื่อแล้วเวอร์ชัน เพื่อให้ได้ output ที่แน่นอนทุกครั้ง
//
// (TH) Run คือจุดเรียกใช้สาธารณะจุดเดียว มันจะ:
//  1. ส่งต่อไปยัง Scanner ของแพลตฟอร์มนั้น (New)
//  2. ลบรายการซ้ำที่มี (name, version, install_path) เหมือนกัน
//  3. เรียงผลลัพธ์เพื่อให้ได้ output ที่แน่นอนทุกครั้งก่อนคืนค่า
func Run(ctx context.Context, report ProgressFunc) (Result, error) {
	started := time.Now() // บันทึกเวลาเริ่มต้นการสแกน

	// เรียก Scanner ที่เหมาะกับแพลตฟอร์มปัจจุบัน (เลือกโดย build tag)
	sw, err := New().Scan(ctx, report)
	if err != nil {
		return Result{}, err // คืน error ถ้าการสแกนล้มเหลว (เช่น context ถูก cancel)
	}

	sw = dedupe(sw)    // ลบรายการซ้ำที่ (name, version, install_path) เหมือนกัน
	sortStable(sw)     // เรียงซอฟต์แวร์ตามชื่อแล้วเวอร์ชัน เพื่อให้ผลลัพธ์คงที่ทุกครั้ง

	// สร้างและคืนค่า Result พร้อมเวลาเริ่มต้น-สิ้นสุดและรายการซอฟต์แวร์
	return Result{StartedAt: started, FinishedAt: time.Now(), Software: sw}, nil
}
