// ไฟล์นี้รวม logic การถอนการติดตั้งที่ใช้ร่วมทั้งหน้า GUI (selfuninstall) และคำสั่ง
// CLI `uninstall` — ลบ service, ARP entry, โฟลเดอร์ติดตั้ง (รวมไบนารีที่กำลังรัน) และ
// (เมื่อ Purge) ลบโฟลเดอร์ข้อมูล/ตั้งค่าใน ProgramData ทั้งหมด
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/installer"
	"github.com/softsentry/agent/internal/service"
)

// seams ที่ test แทนค่าได้ — production ชี้ไปฟังก์ชันจริง
var (
	dataDirFn        = config.Dir
	removeServiceFn  = removeServiceTolerant
	unregisterFn     = installer.UnregisterUninstall
	unregisterTrayFn = installer.UnregisterTrayAutostart
	removeInstallFn  = installer.RemoveInstallDir
)

// removeOptions ควบคุมระดับการลบ
type removeOptions struct {
	KeepConfig bool // เก็บ config + token (สำหรับ reinstall) — ใช้กับ CLI default-ish
	Purge      bool // ลบโฟลเดอร์ข้อมูลทั้งหมด รวม queue (ผู้ใช้สั่งถอนแบบสะอาด)
}

// removeAgent ทำการถอนตามระดับที่กำหนด เขียน log ความคืบหน้าไป out
func removeAgent(out io.Writer, opts removeOptions) error {
	// 1) หยุด + ลบ service (ทนต่อกรณีไม่ได้ติดตั้ง)
	if err := removeServiceFn(); err != nil {
		return fmt.Errorf("remove service: %w", err)
	}
	fmt.Fprintf(out, "  service %q removed\n", service.Name)

	// 2) ลบ ARP entry (best-effort — ไม่ fatal)
	if err := unregisterFn(); err != nil {
		fmt.Fprintf(out, "  (Apps & features entry: %v)\n", err)
	} else {
		fmt.Fprintln(out, "  Apps & features entry removed")
	}

	// 2b) ลบ tray autostart Run key (best-effort — ไม่ fatal)
	if err := unregisterTrayFn(); err != nil {
		fmt.Fprintf(out, "  (tray autostart entry: %v)\n", err)
	}

	// 3) ลบ config/token หรือ purge ทั้งโฟลเดอร์ข้อมูล
	if opts.Purge {
		dir, err := dataDirFn()
		if err != nil {
			return fmt.Errorf("locate data dir: %w", err)
		}
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove data dir: %w", err)
		}
		fmt.Fprintf(out, "  removed %s (all data)\n", dir)
	} else if !opts.KeepConfig {
		removed, err := removeLocalState() // ลบ token + config.yaml, คง queue (spec 1.8)
		if err != nil {
			return fmt.Errorf("remove local state: %w", err)
		}
		for _, p := range removed {
			fmt.Fprintf(out, "  removed %s\n", p)
		}
	}

	// 4) ลบโฟลเดอร์ติดตั้ง (รวมไบนารีที่กำลังรัน) — ทำเป็นขั้นสุดท้ายเสมอ เพราะบน
	// Windows มันจะ schedule sweeper แล้วเราต้องออกจากโปรแกรมเพื่อปลดล็อก
	exe, err := resolveSelfPath()
	if err != nil {
		return fmt.Errorf("resolve own path: %w", err)
	}
	installDir := filepath.Dir(exe)
	if err := removeInstallFn(installDir); err != nil {
		return fmt.Errorf("remove install dir: %w", err)
	}
	fmt.Fprintf(out, "  removed %s\n", installDir)
	return nil
}

// removeServiceTolerant ลบ service แต่ไม่ถือว่า "ไม่ได้ติดตั้ง" เป็น error
func removeServiceTolerant() error {
	if st, err := service.Status(); err == nil && st == "not installed" {
		return nil
	}
	return service.Uninstall()
}
