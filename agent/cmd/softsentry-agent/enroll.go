// ไฟล์นี้กำหนด subcommand `enroll` และฟังก์ชัน enrollMachine ที่ใช้ร่วมกัน
// กับ install/selfinstall — ทำ handshake กับ backend เพื่อแลก enrollment token
// เป็น agent token ถาวร แล้วบันทึก config + token ลงดิสก์
package main

import (
	"context" // ใช้ส่งผ่าน cancellation context ให้ HTTP request
	"fmt"     // ใช้สร้างข้อความ error และพิมพ์ความคืบหน้า
	"io"      // ใช้รับ output writer เพื่อให้ test ได้ inject writer ทดสอบได้
	"time"    // ใช้กำหนด timeout ให้ HTTP request

	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/config"    // ใช้โหลดและบันทึก config ของ agent
	"github.com/softsentry/agent/internal/osutil"    // ใช้ตรวจจับข้อมูลเครื่อง (hostname, OS, arch)
	"github.com/softsentry/agent/internal/storage"   // ใช้บันทึกและล้าง agent token + last_scan
	"github.com/softsentry/agent/internal/transport" // ใช้สร้าง HTTP client และส่ง EnrollRequest
)

// enrollCmd สร้าง subcommand `enroll` สำหรับลงทะเบียนเครื่องกับ SoftSentry server
// รับ flag --token (enrollment token จาก admin) และ --server (URL ของ backend)
func enrollCmd() *cobra.Command {
	var (
		token  string // enrollment token จาก admin dashboard — ใช้ครั้งเดียว
		server string // URL ของ backend เช่น https://softsentry.example.com
	)
	// กำหนด subcommand พร้อมคำอธิบายและ handler หลัก
	c := &cobra.Command{
		Use:   "enroll",                                       // ชื่อ subcommand
		Short: "Enroll this machine with the SoftSentry server", // คำอธิบายสั้น
		Long: `Exchange a one-time enrollment token (issued by an admin in the
dashboard) for a permanent agent token. The token is stored locally with
restrictive permissions and used for all subsequent API calls.`,
		// (TH) แลก one-time enrollment token (ออกให้โดย admin ใน dashboard)
		// เป็น agent token ถาวร token ถูกเก็บไว้ในเครื่องด้วย permission จำกัด
		// และใช้สำหรับ API call ทั้งหมดในอนาคต
		RunE: func(cmd *cobra.Command, _ []string) error {
			// เรียก enrollMachine โดยส่ง context, output writer, token และ server URL
			return enrollMachine(cmd.Context(), cmd.OutOrStdout(), token, server)
		},
	}
	// ลงทะเบียน flag --token สำหรับรับ enrollment token จาก admin
	c.Flags().StringVar(&token, "token", "", "one-time enrollment token from admin")
	// ลงทะเบียน flag --server สำหรับระบุ backend URL
	c.Flags().StringVar(&server, "server", "", "backend URL (e.g. https://softsentry.example.com)")
	return c
}

// enrollMachine performs the enrollment handshake and persists the resulting
// agent token + config. Shared by the `enroll` and `install` commands.
//
// (TH) ทำ handshake การ enroll และบันทึก agent token + config ที่ได้ลงดิสก์
// ใช้ร่วมกันระหว่างคำสั่ง `enroll`, `install` และ selfinstall (one-click installer)
//
// Parameters:
//   - ctx: context สำหรับยกเลิก HTTP request เมื่อจำเป็น
//   - out: writer สำหรับแสดงความคืบหน้า (stdout หรือ io.Discard สำหรับ silent mode)
//   - token: enrollment token จาก admin
//   - server: URL ของ backend
func enrollMachine(ctx context.Context, out io.Writer, token, server string) error {
	// ตรวจสอบว่า token ถูกระบุมา — ขาดไม่ได้เพราะใช้ยืนยันตัวตนกับ server
	if token == "" {
		return fmt.Errorf("--token is required")
	}
	// ตรวจสอบว่า server URL ถูกระบุมา — ขาดไม่ได้เพราะเป็นปลายทางของ HTTP request
	if server == "" {
		return fmt.Errorf("--server is required (or set in config first)")
	}

	// ตรวจจับข้อมูลเครื่อง (hostname, OS, arch, OS version) เพื่อส่งให้ server
	host, err := osutil.Detect()
	if err != nil {
		// ตรวจจับข้อมูลเครื่องล้มเหลว — ไม่สามารถ enroll ได้
		return fmt.Errorf("detect host: %w", err)
	}

	// แสดงข้อมูลที่จะ enroll เพื่อให้ผู้ใช้ทราบก่อนส่ง request
	fmt.Fprintf(out, "Enrolling %s (%s/%s) with %s ...\n",
		host.Hostname, host.OS, host.Arch, server)

	// สร้าง context ที่มี timeout 30 วินาที เพื่อป้องกัน HTTP request ค้างนาน
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel() // คืน resource ของ context เมื่อฟังก์ชันกลับ

	// สร้าง HTTP client โดยไม่มี agent token (ใช้ enrollment token แทน)
	client := transport.New(server, "")
	// ส่ง enrollment request ไปยัง backend พร้อมข้อมูลเครื่องและ token
	resp, err := client.Enroll(reqCtx, transport.EnrollRequest{
		EnrollmentToken: token,           // one-time token จาก admin
		Hostname:        host.Hostname,   // ชื่อเครื่อง
		OS:              host.OS,         // ระบบปฏิบัติการ เช่น "windows", "darwin"
		OSVersion:       host.OSVersion,  // เวอร์ชัน OS เช่น "10.0.22631"
		Arch:            host.Arch,       // สถาปัตยกรรม เช่น "amd64"
		AgentVersion:    agentVersion,    // เวอร์ชัน agent ที่กำลังรัน
	})
	if err != nil {
		// server ปฏิเสธ หรือมีปัญหาเครือข่าย
		return fmt.Errorf("enroll: %w", err)
	}

	// โหลด config ปัจจุบันเพื่ออัปเดตค่าที่ได้จาก server
	cfg, err := config.Load()
	if err != nil {
		// โหลด config ล้มเหลว — ไม่สามารถบันทึกผลการ enroll ได้
		return err
	}
	cfg.ServerURL = server           // บันทึก server URL สำหรับ request ในอนาคต
	cfg.MachineUUID = resp.MachineUUID // บันทึก UUID ของเครื่องที่ server กำหนดให้
	// ใช้ scan interval ที่ server กำหนดมา ถ้ามีค่า (ไม่เป็น 0)
	if resp.ScanIntervalHours > 0 {
		cfg.ScanIntervalHours = resp.ScanIntervalHours // อัปเดต scan interval จาก server policy
	}
	// บันทึก config ที่อัปเดตแล้วลงดิสก์
	if err := config.Save(cfg); err != nil {
		// บันทึก config ล้มเหลว
		return err
	}
	// บันทึก agent token (ที่ได้รับจาก server) ลงดิสก์ด้วย permission จำกัด
	if err := storage.SaveToken(resp.AgentToken); err != nil {
		// บันทึก token ล้มเหลว
		return err
	}
	// A (re-)enrollment gets a fresh agent token and a server-side machine record
	// that has never received a scan. Drop any stale last_scan from a previous
	// install so the next `run` treats this as a first scan and uploads inventory
	// immediately — otherwise the machine appears online but with 0 software until
	// the next scheduled auto-scan (up to scan_interval_hours later).
	//
	// (TH) การ (re-)enroll จะได้ agent token ใหม่ และ record เครื่องฝั่งเซิร์ฟเวอร์
	// ที่ยังไม่เคยถูกสแกนเลย จึงต้องล้างค่า last_scan เก่าจากการติดตั้งครั้งก่อน
	// เพื่อให้ `run` รอบถัดไปมองว่านี่คือการสแกนครั้งแรกและอัปโหลด inventory
	// ทันที — ไม่อย่างนั้นเครื่องจะขึ้นสถานะออนไลน์แต่มีซอฟต์แวร์ 0 รายการ จนกว่า
	// จะถึงรอบ auto-scan ที่ตั้งเวลาไว้ครั้งถัดไป (อาจนานถึง scan_interval_hours)
	if err := storage.ClearLastScan(); err != nil {
		// ล้าง last_scan ล้มเหลว — อาจทำให้ scan ครั้งแรกถูกข้ามไป
		return fmt.Errorf("reset scan schedule: %w", err)
	}

	// แสดงผลลัพธ์การ enroll ที่สำเร็จ
	fmt.Fprintf(out, "✓ Enrolled. Machine UUID: %s\n", resp.MachineUUID)
	// แสดง scan interval ที่จะใช้ (จาก server หรือ default)
	fmt.Fprintf(out, "  Token stored. Scan interval: %dh\n", cfg.ScanIntervalHours)
	return nil
}
