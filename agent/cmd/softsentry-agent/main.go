// Command softsentry-agent is the SoftSentry endpoint agent.
// (TH) คำสั่ง softsentry-agent คือ endpoint agent ของ SoftSentry —
// จุดเริ่มต้นของโปรแกรมทั้งหมด รับ signal จาก OS และเลือกเส้นทางการทำงาน
// ระหว่าง self-install (ดับเบิลคลิก Setup.exe) กับ CLI ปกติ (Cobra)
package main

import (
	"context"  // ใช้สร้าง root context และยกเลิกเมื่อได้รับ signal
	"errors"   // ใช้ตรวจสอบประเภทของ error (เช่น context.Canceled)
	"fmt"      // ใช้พิมพ์ข้อความ error ไปยัง stderr
	"os"       // ใช้ออกจากโปรแกรมด้วย exit code และเขียนไปยัง stderr
	"os/signal" // ใช้ดักจับ OS signal เช่น SIGINT (Ctrl+C)
	"syscall"  // ใช้ระบุ SIGTERM สำหรับการหยุด service/daemon
)

func main() {
	// Cancel the root context on Ctrl+C (SIGINT) or service/daemon stop
	// (SIGTERM) so the run loop shuts down gracefully.
	//
	// (TH) ยกเลิก root context เมื่อกด Ctrl+C (SIGINT) หรือเมื่อ service/daemon
	// ถูกสั่งหยุด (SIGTERM) เพื่อให้ run loop ปิดตัวเองได้อย่างนุ่มนวล
	// signal.NotifyContext คืนค่า ctx ที่ถูก cancel อัตโนมัติเมื่อได้รับ signal
	// และ stop ที่ใช้หยุดการฟัง signal เมื่อไม่ต้องการแล้ว
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop() // ปล่อย resource ที่ใช้ฟัง signal เมื่อ main กลับ

	// If this is a downloaded SoftSentry-Setup.exe (double-clicked, with an
	// embedded config trailer), install ourselves instead of running the CLI.
	//
	// (TH) ถ้านี่คือ SoftSentry-Setup.exe ที่ดาวน์โหลดมา (ถูกดับเบิลคลิก และมี
	// config trailer ฝังอยู่) ให้ติดตั้งตัวเองแทนที่จะรัน CLI ปกติ
	// maybeSelfInstall ตรวจสอบว่าไบนารีมี config trailer หรือไม่
	// ถ้ามี จะจัดการ self-install แล้วคืน handled=true
	if handled, err := maybeSelfInstall(ctx); err != nil {
		// การติดตั้งตัวเองล้มเหลว — แสดง error และรอผู้ใช้กด Enter ก่อนปิด
		fmt.Fprintln(os.Stderr, "error:", err)
		waitForExit(os.Stdout) // รอผู้ใช้กด Enter เพื่อไม่ให้หน้าต่างปิดทันที
		os.Exit(1)             // ออกด้วย exit code 1 เพื่อบ่งชี้ว่าเกิดข้อผิดพลาด
	} else if handled {
		// self-install จัดการเรียบร้อยแล้ว — ไม่ต้องรัน CLI ปกติ
		return
	}

	// รัน CLI ปกติผ่าน Cobra — ให้ root command แยกแยะ subcommand ต่างๆ
	if err := rootCmd().ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return // graceful shutdown, not an error
			// (TH) ปิดตัวแบบนุ่มนวลเนื่องจาก signal ยกเลิก — ไม่ถือเป็น error
		}
		// error จริงจาก CLI — แสดงข้อความและออกด้วย exit code 1
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
