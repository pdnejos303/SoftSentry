// ไฟล์นี้กำหนด subcommand `status` สำหรับแสดงสถานะปัจจุบันของ agent
// แสดงข้อมูลสองส่วนแยกกัน: สถานะ OS service และสถานะการ enroll
// เป็น read-only command — ไม่มีการเปลี่ยนแปลงใดๆ เกิดขึ้น
package main

import (
	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/config"  // ใช้โหลด config เพื่อตรวจสอบสถานะ enroll
	"github.com/softsentry/agent/internal/service" // ใช้ query สถานะ OS service จาก SCM/launchctl
)

// statusCmd prints two independent pieces of state:
//   - OS service state (running / stopped / not installed) — from the SCM / launchctl
//   - Enrollment state — inferred from config.ServerURL being non-empty, which
//     is only set after a successful enroll handshake
//
// (TH) แสดงสถานะที่เป็นอิสระต่อกันสองอย่าง:
//   - สถานะ OS service (running / stopped / not installed) — จาก SCM / launchctl
//   - สถานะการ enroll — อนุมานจากค่า config.ServerURL ว่าไม่ว่าง ซึ่งจะถูกตั้งค่า
//     ก็ต่อเมื่อ enroll handshake สำเร็จเท่านั้น
func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",                           // ชื่อ subcommand
		Short: "Show service + enrollment status", // คำอธิบายสั้น
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Query สถานะ OS service จาก service manager (SCM บน Windows, launchctl บน macOS)
			st, err := service.Status()
			if err != nil {
				// Query สถานะ service ล้มเหลว (เช่น ไม่มีสิทธิ์ หรือ service manager ไม่ตอบ)
				return err
			}
			// แสดงสถานะ service เช่น "running", "stopped", "not installed"
			cmd.Printf("Service:  %s\n", st)

			// โหลด config เพื่อตรวจสอบสถานะการ enroll
			cfg, err := config.Load()
			if err != nil {
				// โหลด config ล้มเหลว
				return err
			}
			// ServerURL is written by enrollMachine on first successful enroll;
			// its absence is the canonical "not enrolled" signal (no separate flag needed).
			//
			// (TH) ServerURL ถูกเขียนโดย enrollMachine ตอน enroll สำเร็จครั้งแรก
			// การที่มันว่างคือสัญญาณบ่งชี้ "ยังไม่ enroll" อย่างเป็นทางการ
			// (ไม่ต้องมี flag แยกต่างหาก) — ออกแบบให้เรียบง่ายและไม่มีสถานะซ้ำซ้อน
			if cfg.ServerURL == "" {
				// แสดงข้อความแนะนำวิธี enroll เพื่อช่วย IT admin ที่เพิ่ง install
				cmd.Println("Enrolled: no — run `softsentry-agent enroll` or `install --enrollment-token`")
				return nil
			}
			// แสดงรายละเอียด enrollment ทั้งหมดสำหรับเครื่องที่ enroll แล้ว
			cmd.Printf("Enrolled: yes\n")
			cmd.Printf("  Server:       %s\n", cfg.ServerURL)        // URL ของ backend server
			cmd.Printf("  Machine UUID: %s\n", cfg.MachineUUID)      // UUID ของเครื่องนี้ใน system
			cmd.Printf("  Scan every:   %dh\n", cfg.ScanIntervalHours) // ความถี่การสแกน (ชั่วโมง)
			cmd.Printf("  Auto-update:  %t\n", cfg.AutoUpdateEnabled) // สถานะการ auto-update
			return nil
		},
	}
}
