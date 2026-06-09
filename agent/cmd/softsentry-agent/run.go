// ไฟล์นี้กำหนด subcommand `run` ซึ่งเป็น entry point หลักของ agent เมื่อรันเป็น service
// รองรับสองโหมด: long-running loop (default) และ one-shot (--one-shot flag)
// บน Windows จะตรวจจับว่าถูกเริ่มโดย SCM หรือไม่ และใช้ protocol ที่เหมาะสม
package main

import (
	"errors" // ใช้ตรวจสอบว่า error เป็นประเภท ErrRestartRequired หรือไม่

	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/runner"  // ใช้รัน agent loop (heartbeat + scan)
	"github.com/softsentry/agent/internal/service" // ใช้ตรวจสอบและรัน Windows service protocol
)

// runCmd สร้าง subcommand `run` ซึ่งเป็นลูปหลักของ agent
// เมื่อรันเป็น service จะถูกเรียกโดย OS service manager (SCM/launchd)
// เมื่อรันตรงจาก terminal จะทำงานเป็น foreground loop
func runCmd() *cobra.Command {
	var oneShot bool // flag สำหรับรัน scan+heartbeat ครั้งเดียวแล้วออก (ใช้สำหรับ debug)
	c := &cobra.Command{
		Use:   "run",                                                   // ชื่อ subcommand
		Short: "Run the agent loop (heartbeat + scheduled/triggered scan)", // คำอธิบายสั้น
		Long: `Long-running mode. Scans once on start, then heartbeats on the configured
interval. A manual_scan_requested flag in a heartbeat response triggers an
immediate scan + upload. Use --one-shot to scan + heartbeat once and exit.`,
		// (TH) โหมด long-running: สแกนครั้งแรกตอนเริ่ม จากนั้น heartbeat ตาม interval
		// ที่ตั้งค่าไว้ flag manual_scan_requested ใน heartbeat response จะ trigger
		// การสแกนและอัปโหลดทันที ใช้ --one-shot เพื่อสแกน+heartbeat ครั้งเดียวแล้วออก
		RunE: func(cmd *cobra.Command, _ []string) error {
			// กำหนด option สำหรับ runner loop
			opts := runner.Options{
				OneShot: oneShot,           // true = รันครั้งเดียวแล้วออก
				Out:     cmd.OutOrStdout(), // output writer สำหรับ log ปกติ
				Err:     cmd.ErrOrStderr(), // error writer สำหรับ log ข้อผิดพลาด
			}
			// On Windows, the binary is started by the SCM rather than a terminal.
			// RunIfService detects that context and runs the Windows service control
			// protocol (accepting Stop/Shutdown signals from the SCM) instead of the
			// plain foreground loop. On macOS / foreground, it is a no-op.
			//
			// (TH) บน Windows ไบนารีจะถูกเริ่มโดย SCM แทนที่จะเป็น terminal
			// RunIfService จะตรวจจับบริบทนั้นและรัน Windows service control
			// protocol (รับสัญญาณ Stop/Shutdown จาก SCM) แทนลูปแบบ foreground
			// ปกติ ส่วนบน macOS / โหมด foreground ฟังก์ชันนี้จะไม่ทำอะไรเลย
			if handled, err := service.RunIfService(opts); handled {
				// ถูกจัดการโดย Windows service protocol แล้ว — คืน error (หรือ nil)
				return err
			}
			// รัน foreground loop ปกติ (terminal หรือ non-Windows service)
			err := runner.Run(cmd.Context(), opts)
			if errors.Is(err, runner.ErrRestartRequired) {
				// Self-update completed: the new binary is in place. Exit cleanly
				// so the service manager's "restart on failure" recovery action (or
				// the user) relaunches the process onto the updated binary.
				//
				// (TH) self-update เสร็จสมบูรณ์: ไบนารีตัวใหม่อยู่ในตำแหน่งแล้ว
				// ออกจากโปรแกรมแบบสะอาด เพื่อให้กลไก "restart on failure" ของ
				// service manager (หรือผู้ใช้) รันโพรเซสขึ้นมาใหม่ด้วยไบนารีตัวที่
				// อัปเดตแล้ว — ออกโดยไม่ส่ง error เพราะเป็นการ restart ที่ตั้งใจ
				cmd.Println("Agent updated — exiting so the new binary takes over on next start.")
				return nil // ออกปกติ — service manager จะ restart ด้วยไบนารีใหม่
			}
			// คืน error อื่นๆ (เช่น context canceled จาก Ctrl+C หรือ SIGTERM)
			return err
		},
	}
	// ลงทะเบียน flag --one-shot สำหรับรัน scan+heartbeat ครั้งเดียวแล้วออก
	// เหมาะสำหรับ debug หรือ cron job
	c.Flags().BoolVar(&oneShot, "one-shot", false, "scan + heartbeat once then exit")
	return c
}
