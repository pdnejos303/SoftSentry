//go:build windows

// ไฟล์นี้ให้คำสั่ง (ซ่อนจาก help) สำหรับ "พรีวิว" หน้าจอ wizard ของ self-installer
// บน Windows โดยไม่ต้องมี config trailer / ไม่ขอสิทธิ์ admin / ไม่แตะดิสก์ — ใช้ทดสอบ
// เฉพาะ "หน้าตา/พฤติกรรมหน้าต่าง" (เช่น ตรึงขนาด, การเปลี่ยน step, การ reflow) ซ้ำๆ
// ได้สะดวกเวลาแก้ UI ของ wizard_windows.go โดยไม่ต้องสร้าง Setup.exe จริงทุกครั้ง
package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/installer"
)

// previewWizardCmd คืน subcommand `preview-wizard` (เฉพาะ build Windows) — เปิด
// wizard จริงด้วยข้อมูลตัวอย่าง แล้ว (ถ้าผู้ใช้กดติดตั้ง) ต่อด้วยหน้าความคืบหน้า
// ที่จำลองการทำงาน 4 ขั้น เพื่อทดสอบหน้าต่างทั้งสองที่ถูกตรึงขนาด
func previewWizardCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "preview-wizard",
		Short:  "Preview the installer wizard UI (debug, no install)",
		Hidden: true, // ไม่แสดงใน help — เป็นเครื่องมือ QA ภายใน
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, _ := installDirDefault()
			res, shown := installer.RunWizard("https://softsentry.example.local", installer.DefaultLang(), dir)
			if !shown {
				return fmt.Errorf("wizard GUI could not be shown")
			}
			cmd.Printf("wizard result: proceed=%v lang=%s dir=%q\n", res.Proceed, res.Lang, res.InstallDir)
			if !res.Proceed {
				return nil // ผู้ใช้กดยกเลิก/ปิด — ไม่ต้องแสดงหน้าความคืบหน้า
			}
			// จำลองการติดตั้ง: เดินทีละขั้น (0..3) ขั้นละ ~1.2 วินาที เพื่อให้เห็น
			// หน้าความคืบหน้าค้างพอให้ลองกด maximize/ลากขอบ (ต้องทำไม่ได้)
			fakeRun := func(setStep func(int)) error {
				for i := 0; i < 4; i++ {
					setStep(i)
					time.Sleep(1200 * time.Millisecond)
				}
				return nil
			}
			if _, ok := installer.RunInstallProgress(res.Lang, fakeRun); !ok {
				return fmt.Errorf("progress GUI could not be shown")
			}
			return nil
		},
	}
}
