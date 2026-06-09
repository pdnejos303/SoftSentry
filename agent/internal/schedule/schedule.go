// Package schedule ครอบคลุม logic การจัดตารางสแกนที่ไม่ขึ้นกับ platform (spec 1.1):
// จำกัดค่า interval ให้อยู่ในช่วงที่รองรับ และคำนวณว่าต้องรอนานแค่ไหนจึงจะถึงเวลาสแกนครั้งถัดไป
// โดยอิงจากเวลาสแกนล่าสุด เพื่อให้การ restart ไม่ทำให้สแกนซ้ำเร็วเกินไป
package schedule

import "time" // ใช้ time.Duration และ time.Time สำหรับการคำนวณเวลาและช่วงเวลา

// ค่าคงที่สำหรับ interval ของการสแกนอัตโนมัติ (spec 1.1)
const (
	DefaultIntervalHours = 6  // ค่า default เมื่อไม่ได้ระบุ interval หรือระบุค่าที่ไม่ถูกต้อง = 6 ชั่วโมง
	MinIntervalHours     = 1  // ค่าต่ำสุดที่อนุญาต = 1 ชั่วโมง เพื่อป้องกันการสแกนถี่เกินไป
	MaxIntervalHours     = 24 // ค่าสูงสุดที่อนุญาต = 24 ชั่วโมง เพื่อให้สแกนอย่างน้อยวันละครั้ง
)

// Interval รับค่า interval ที่ตั้งค่าไว้ (หน่วยชั่วโมง) แล้วจำกัดให้อยู่ในช่วงที่รองรับ
// จากนั้นแปลงเป็น time.Duration เพื่อใช้งานต่อ
//
// Parameter:
//   - hours: จำนวนชั่วโมงที่ตั้งค่าไว้สำหรับ interval; ค่าศูนย์หรือติดลบจะถูก fallback เป็น default
//
// Return: time.Duration ที่แทนค่า interval จริงที่จะใช้
func Interval(hours int) time.Duration {
	// ถ้า hours เป็น 0 หรือติดลบ ให้ใช้ค่า default แทน
	if hours <= 0 {
		hours = DefaultIntervalHours
	}
	// ถ้า hours น้อยกว่าค่าต่ำสุด ให้ clamp ขึ้นมาเป็นค่าต่ำสุด
	if hours < MinIntervalHours {
		hours = MinIntervalHours
	}
	// ถ้า hours มากกว่าค่าสูงสุด ให้ clamp ลงมาเป็นค่าสูงสุด
	if hours > MaxIntervalHours {
		hours = MaxIntervalHours
	}
	// แปลงจำนวนชั่วโมงเป็น time.Duration แล้วคืนค่ากลับ
	return time.Duration(hours) * time.Hour
}

// DueIn รายงานว่าต้องรอนานแค่ไหนจนถึงเวลาสแกนครั้งถัดไป
// ถ้า lastScan เป็นค่าศูนย์ (ยังไม่เคยสแกน) หรือ interval ผ่านไปแล้ว
// จะคืนค่า 0 ซึ่งหมายความว่า "สแกนเดี๋ยวนี้เลย"
//
// Parameter:
//   - lastScan: เวลาที่สแกนครั้งล่าสุด; zero value หมายถึงยังไม่เคยสแกน
//   - interval: ช่วงเวลาระหว่างการสแกน ได้จาก Interval()
//   - now: เวลาปัจจุบันที่ใช้เปรียบเทียบ (รับเป็น parameter เพื่อให้ testable)
//
// Return: time.Duration ที่เหลืออยู่จนถึงเวลาสแกนถัดไป หรือ 0 ถ้าถึงเวลาแล้ว
func DueIn(lastScan time.Time, interval time.Duration, now time.Time) time.Duration {
	// ถ้า lastScan เป็น zero value แสดงว่ายังไม่เคยสแกน ให้สแกนทันที
	if lastScan.IsZero() {
		return 0
	}
	// คำนวณเวลาที่ควรสแกนครั้งถัดไป โดยบวก interval เข้ากับเวลาสแกนล่าสุด
	next := lastScan.Add(interval)
	// ถ้าเวลาสแกนถัดไปไม่ได้อยู่หลัง now แสดงว่าถึงเวลาหรือเลยเวลาแล้ว ให้สแกนทันที
	if !next.After(now) {
		return 0
	}
	// คืนค่าความแตกต่างระหว่างเวลาสแกนถัดไปกับเวลาปัจจุบัน (เวลาที่เหลือ)
	return next.Sub(now)
}
