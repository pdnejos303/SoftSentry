// ไฟล์นี้กำหนด subcommand `uninstall` สำหรับถอดถอน agent ออกจากเครื่อง
// จะหยุดและลบ OS service, ลบ config file และ agent token
// แต่จะคง queue directory ไว้เสมอ (ตาม spec 1.8) เพราะอาจมีผลสแกนที่รอ upload
package main

import (
	"errors" // ใช้ตรวจสอบว่า error เป็นประเภท ErrNotExist (ไฟล์ไม่มีอยู่) หรือไม่
	"os"     // ใช้ลบไฟล์ด้วย os.Remove

	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/config"  // ใช้หา path ของ config file
	"github.com/softsentry/agent/internal/service" // ใช้หยุดและลบ OS service
	"github.com/softsentry/agent/internal/storage" // ใช้หา path ของ agent token file
)

// uninstallCmd สร้าง subcommand `uninstall` ที่หยุดและถอดถอน agent service
// ต้องการสิทธิ์ admin/root เพื่อจัดการ OS service
// รองรับ flag --keep-config เพื่อเก็บ config และ token ไว้สำหรับการติดตั้งใหม่
func uninstallCmd() *cobra.Command {
	var keepConfig bool // flag สำหรับเก็บ config + token ไว้ (ไม่ลบ)
	c := &cobra.Command{
		Use:   "uninstall",                                               // ชื่อ subcommand
		Short: "Stop + remove the agent service (admin/root required)", // คำอธิบายสั้น
		Long: `Stop and delete the OS service. By default the local config + agent token
are also removed; the retry queue dir (scans waiting to upload) is always kept.
The agent binary itself is left in place. Use --keep-config to retain config.`,
		// (TH) หยุดและลบ OS service ค่า default จะลบ config + agent token ด้วย
		// แต่ queue directory (ผลสแกนที่รอ upload) จะถูกเก็บไว้เสมอ
		// ไบนารีของ agent เองจะไม่ถูกลบ ใช้ --keep-config เพื่อเก็บ config ไว้
		RunE: func(cmd *cobra.Command, _ []string) error {
			// หยุดและลบ OS service (ต้องการสิทธิ์ admin)
			if err := service.Uninstall(); err != nil {
				// ลบ service ล้มเหลว (เช่น ไม่มีสิทธิ์ หรือ service ไม่ได้ติดตั้ง)
				return err
			}
			// แสดงผลสำเร็จพร้อมชื่อ service
			cmd.Printf("✓ Service %q removed.\n", service.Name)

			// ถ้าระบุ --keep-config ให้คง config + token ไว้ (ไม่ลบ)
			if keepConfig {
				cmd.Println("  Config + token kept (--keep-config).")
				return nil
			}
			// ลบ config file และ agent token (กรณี default ที่ไม่มี --keep-config)
			removed, err := removeLocalState()
			if err != nil {
				// ลบ local state ล้มเหลว
				return err
			}
			// แสดงรายการไฟล์ที่ถูกลบ เพื่อให้ผู้ใช้รู้ว่าอะไรถูกลบไปบ้าง
			for _, p := range removed {
				cmd.Printf("  removed %s\n", p)
			}
			// แจ้งว่า queue directory ยังคงอยู่ (ตาม spec 1.8)
			cmd.Println("  Retry queue kept (in case scans are still pending upload).")
			return nil
		},
	}
	// ลงทะเบียน flag --keep-config สำหรับเก็บ config + token ไว้
	// มีประโยชน์เมื่อต้องการ reinstall โดยไม่ต้อง enroll ใหม่
	c.Flags().BoolVar(&keepConfig, "keep-config", false, "keep local config + agent token")
	return c
}

// removeLocalState deletes the config file + agent token, preserving the
// queue/ subdirectory (spec 1.8: keep pending scans). Returns the paths it
// removed.
//
// (TH) ลบไฟล์ config + agent token โดยคงไดเรกทอรีย่อย queue/ ไว้ (ตาม spec 1.8:
// เก็บผลสแกนที่ยังค้างอยู่) คืนค่ารายการ path ที่ถูกลบ
// ถ้าไฟล์ใดไม่มีอยู่ (ErrNotExist) จะข้ามไปโดยไม่ถือเป็น error
func removeLocalState() ([]string, error) {
	var removed []string // สะสม path ของไฟล์ที่ถูกลบสำเร็จ เพื่อรายงานกลับ

	// หา path ของ agent token file
	tokenPath, err := storage.TokenPath()
	if err != nil {
		// หา path ของ token file ล้มเหลว
		return nil, err
	}
	// หา path ของ config file
	cfgPath, err := config.Path()
	if err != nil {
		// หา path ของ config file ล้มเหลว
		return nil, err
	}
	// วน loop ลบทั้ง token file และ config file
	for _, p := range []string{tokenPath, cfgPath} {
		if err := os.Remove(p); err != nil {
			// ตรวจสอบว่าไฟล์ไม่มีอยู่ — ถือว่าปกติ (อาจไม่เคยถูกสร้าง)
			if errors.Is(err, os.ErrNotExist) {
				continue // ข้ามไปไฟล์ถัดไป ไม่ถือเป็น error
			}
			// error อื่นๆ เช่น permission denied — คืน error พร้อมรายการที่ลบได้แล้ว
			return removed, err
		}
		// ลบสำเร็จ — เพิ่ม path ลงในรายการ
		removed = append(removed, p)
	}
	return removed, nil
}
