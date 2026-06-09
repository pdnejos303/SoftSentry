// ไฟล์นี้กำหนด subcommand `install` สำหรับลงทะเบียน agent เป็น OS service
// รองรับทั้ง Windows SCM service และ macOS LaunchDaemon
// สามารถส่ง --enrollment-token เพื่อ enroll พร้อมกันในขั้นตอนเดียว
package main

import (
	"fmt"          // ใช้สร้างข้อความ error และ format ข้อความผลลัพธ์
	"os"           // ใช้อ่าน path ของ executable ปัจจุบัน
	"path/filepath" // ใช้แปลง path ให้เป็น absolute path

	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/service" // ใช้ติดตั้งและจัดการ OS service
)

// installCmd สร้าง subcommand `install` ที่ลงทะเบียน agent เป็น OS service
// ต้องการสิทธิ์ admin/root เพื่อลงทะเบียน service กับ OS
func installCmd() *cobra.Command {
	var (
		token  string // enrollment token สำหรับ enroll ก่อน install (ไม่บังคับ)
		server string // URL ของ backend — จำเป็นเมื่อระบุ --enrollment-token
	)
	// กำหนด subcommand พร้อมคำอธิบายและ handler หลัก
	c := &cobra.Command{
		Use:   "install",                                                   // ชื่อ subcommand
		Short: "Install + start the agent as an OS service (admin/root required)", // คำอธิบายสั้น
		Long: `Register the agent as a Windows SCM service / macOS LaunchDaemon so it
starts on boot and restarts on failure. Pass --enrollment-token to enroll this
machine first, so the service starts already connected to the server.`,
		// (TH) ลงทะเบียน agent เป็น Windows SCM service / macOS LaunchDaemon เพื่อให้
		// เริ่มทำงานอัตโนมัติตอนบูต และ restart เองเมื่อล้มเหลว ส่ง --enrollment-token
		// เพื่อ enroll เครื่องก่อน ทำให้ service เริ่มพร้อม connection กับ server ทันที
		RunE: func(cmd *cobra.Command, _ []string) error {
			// หา absolute path ของไบนารีที่กำลังรันอยู่ — ใช้เป็น path ของ service
			exe, err := resolveSelfPath()
			if err != nil {
				// หา path ของตัวเองไม่ได้ — ไม่สามารถลงทะเบียน service ได้
				return err
			}

			// ถ้าระบุ token มา ให้ enroll ก่อนติดตั้ง service
			// เพื่อให้ service เริ่มต้นพร้อม agent token และ config พร้อมใช้ทันที
			if token != "" {
				if err := enrollMachine(cmd.Context(), cmd.OutOrStdout(), token, server); err != nil {
					// enroll ล้มเหลว — ยกเลิกการติดตั้ง service ด้วย
					return err
				}
			}

			// ลงทะเบียน agent เป็น OS service โดยส่ง path ไบนารีและ argument "run"
			// service.Install จะสร้างและเริ่ม service อัตโนมัติ
			if err := service.Install(service.Config{ExePath: exe, Args: []string{"run"}}); err != nil {
				// ลงทะเบียน service ล้มเหลว (เช่น ไม่มีสิทธิ์ admin)
				return err
			}
			// แสดงผลสำเร็จพร้อมชื่อ service และ path ของไบนารี
			cmd.Printf("✓ Service %q installed and started (binary: %s).\n", service.Name, exe)
			return nil
		},
	}
	// ลงทะเบียน flag --enrollment-token สำหรับ enroll ก่อน install
	c.Flags().StringVar(&token, "enrollment-token", "", "one-time enrollment token (enroll before installing)")
	// ลงทะเบียน flag --server สำหรับระบุ backend URL (จำเป็นเมื่อใช้ --enrollment-token)
	c.Flags().StringVar(&server, "server", "", "backend URL (required with --enrollment-token)")
	return c
}

// resolveSelfPath returns the absolute path to the running agent binary, which
// the service manager will invoke on boot.
// (TH) คืนค่า absolute path ของไบนารี agent ที่กำลังรันอยู่ ซึ่ง service manager
// จะเรียกใช้ตอนบูตเครื่อง — ต้องเป็น absolute path เพื่อให้ service หาไฟล์ได้เสมอ
// ไม่ว่า working directory ในขณะบูตจะเป็นอะไร
func resolveSelfPath() (string, error) {
	// os.Executable คืน path ของไฟล์ executable ที่กำลังรัน (อาจเป็น symlink)
	exe, err := os.Executable()
	if err != nil {
		// ไม่สามารถหา path ของตัวเองได้ — ไม่ปกติมาก
		return "", fmt.Errorf("resolve own path: %w", err)
	}
	// แปลง path ให้เป็น absolute path (resolve symlink และ relative component)
	abs, err := filepath.Abs(exe)
	if err != nil {
		// แปลงเป็น absolute path ล้มเหลว
		return "", fmt.Errorf("absolute path: %w", err)
	}
	return abs, nil
}
