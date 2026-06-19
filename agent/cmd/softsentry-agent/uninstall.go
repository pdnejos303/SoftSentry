// ไฟล์นี้กำหนด subcommand `uninstall` สำหรับถอดถอน agent ออกจากเครื่อง
// จะหยุดและลบ OS service, ลบ config file และ agent token
// แต่จะคง queue directory ไว้เสมอ (ตาม spec 1.8) เพราะอาจมีผลสแกนที่รอ upload
package main

import (
	"errors" // ใช้ตรวจสอบว่า error เป็นประเภท ErrNotExist (ไฟล์ไม่มีอยู่) หรือไม่
	"os"     // ใช้ลบไฟล์ด้วย os.Remove

	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/config"  // ใช้หา path ของ config file
	"github.com/softsentry/agent/internal/storage" // ใช้หา path ของ agent token file
)

// uninstallCmd สร้าง subcommand `uninstall` ที่หยุดและถอดถอน agent service
// ต้องการสิทธิ์ admin/root เพื่อจัดการ OS service
// รองรับ flag --keep-config เพื่อเก็บ config และ token ไว้สำหรับการติดตั้งใหม่
func uninstallCmd() *cobra.Command {
	var keepConfig bool // flag สำหรับเก็บ config + token ไว้ (ไม่ลบ)
	var purge bool      // flag สำหรับลบทุกอย่างรวม queue
	c := &cobra.Command{
		Use:   "uninstall",                                             // ชื่อ subcommand
		Short: "Stop + remove the agent service (admin/root required)", // คำอธิบายสั้น
		Long: `Stop and delete the OS service, remove the ARP (Apps & features) entry,
and delete the install directory including the agent binary.
By default the retry queue dir (scans waiting to upload) is kept.
Use --keep-config to retain config + token, or --purge to remove everything.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// --purge ลบทุกอย่างรวมโฟลเดอร์ข้อมูล+queue; ค่า default ยังคงพฤติกรรมเดิม
			// (เก็บ queue ตาม spec 1.8) และตอนนี้ลบ ARP entry + โฟลเดอร์ติดตั้งให้ด้วย
			opts := removeOptions{KeepConfig: keepConfig, Purge: purge}
			if err := removeAgent(cmd.OutOrStdout(), opts); err != nil {
				return err
			}
			if keepConfig {
				cmd.Println("  Config + token kept (--keep-config).")
			} else if !purge {
				cmd.Println("  Retry queue kept (in case scans are still pending upload).")
			}
			cmd.Println("✓ SoftSentry Agent uninstalled.")
			return nil
		},
	}
	// ลงทะเบียน flag --keep-config สำหรับเก็บ config + token ไว้
	// มีประโยชน์เมื่อต้องการ reinstall โดยไม่ต้อง enroll ใหม่
	c.Flags().BoolVar(&keepConfig, "keep-config", false, "keep local config + agent token")
	c.Flags().BoolVar(&purge, "purge", false, "also remove all stored data incl. the retry queue")
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
