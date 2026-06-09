//go:build darwin
// ไฟล์นี้ compile เฉพาะบน macOS (darwin) เท่านั้น

// Package service จัดการ launchd daemon บน macOS
package service

import (
	"context"   // ใช้ context.WithTimeout เพื่อจำกัดเวลาการรัน launchctl command
	"fmt"       // ใช้ fmt.Errorf สำหรับ wrap error พร้อมข้อความอธิบาย
	"os"        // ใช้ os.MkdirAll, os.WriteFile, os.Remove, os.Stat สำหรับจัดการ file system
	"os/exec"   // ใช้ exec.CommandContext สำหรับรัน launchctl command
	"path/filepath" // ใช้ filepath.Dir สำหรับดึง directory จาก path
	"time"      // ใช้ time.Second สำหรับกำหนด timeout ของ launchctl

	"github.com/softsentry/agent/internal/runner" // ใช้ runner.Options สำหรับ parameter ของ RunIfService
)

// ค่าคงที่สำหรับ path ของไฟล์ที่ใช้บน macOS
const (
	// plistPath คือ path ของ plist file ที่ติดตั้งใน LaunchDaemons directory
	// LaunchDaemons ใช้สำหรับ daemon ที่รันเป็น root (ต้องการสิทธิ์ sudo ในการเขียน)
	plistPath = "/Library/LaunchDaemons/" + LaunchdLabel + ".plist"
	// logPath คือ path ของ log file ที่ daemon จะเขียน stdout/stderr ไป
	logPath = "/Library/Logs/SoftSentry/agent.log"
)

// Install เขียน LaunchDaemon plist ลงดิสก์และสั่ง launchd load daemon (spec 1.8)
// ต้องรันด้วยสิทธิ์ root (sudo) เพราะ /Library/LaunchDaemons/ เป็นของ system
//
// Parameter:
//   - cfg: Config ที่ระบุ ExePath และ Args ของ agent binary
//
// Return: error ถ้าสร้าง log directory, เขียน plist, หรือ load daemon ล้มเหลว
func Install(cfg Config) error {
	// สร้าง directory สำหรับ log file ถ้ายังไม่มี (รวม parent directories ด้วย)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		// เกิด error ไม่สามารถสร้าง log directory ได้
		return fmt.Errorf("create log dir: %w", err)
	}
	// สร้าง plist XML content จาก label, binary path, arguments, และ log path
	plist := launchdPlist(LaunchdLabel, cfg.ExePath, cfg.Args, logPath)
	// เขียน plist ลงไฟล์ด้วย permission 0644 (world-readable ตาม convention ของ daemon plist)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil { //nolint:gosec // root-owned daemon plist is world-readable by convention
		// เกิด error เขียน plist ไม่ได้ อาจเป็นเพราะไม่มีสิทธิ์ (ต้องการ sudo)
		return fmt.Errorf("write %s (need sudo?): %w", plistPath, err)
	}
	// สั่ง launchctl load plist เพื่อให้ launchd รู้จัก daemon และเริ่มรัน
	if err := launchctl("load", "-w", plistPath); err != nil {
		// เกิด error ไม่สามารถ load daemon ได้ผ่าน launchctl
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

// Uninstall สั่ง unload และลบ LaunchDaemon plist ออก
// ต้องรันด้วยสิทธิ์ root เพราะต้องลบไฟล์ใน /Library/LaunchDaemons/
//
// Return: error ถ้าลบ plist ล้มเหลว (ยกเว้นกรณีไฟล์ไม่มีอยู่แล้ว)
func Uninstall() error {
	// Unload เป็น best-effort: daemon ที่ไม่ได้ load อยู่ไม่ควรบล็อกการลบ
	// จึงไม่สนใจ error จาก unload
	_ = launchctl("unload", "-w", plistPath)
	// ลบ plist file; ถ้าไม่มีอยู่แล้ว (os.IsNotExist) ให้ถือว่าสำเร็จ
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		// เกิด error จริงๆ ไม่ใช่แค่ไฟล์ไม่มี
		return fmt.Errorf("remove %s: %w", plistPath, err)
	}
	return nil
}

// Status รายงานสถานะว่า daemon ติดตั้งและ load อยู่หรือไม่
//
// Return:
//   - "not installed": ไม่พบ plist file
//   - "installed (not loaded)": มี plist แต่ daemon ไม่ได้ load อยู่
//   - "running": daemon ถูก load และรันอยู่
//   - error: เกิดข้อผิดพลาดในการตรวจสอบ
func Status() (string, error) {
	// ตรวจสอบว่า plist file มีอยู่หรือไม่ โดยใช้ os.Stat
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		// ไม่พบ plist file แสดงว่ายังไม่ได้ติดตั้ง
		return "not installed", nil
	}
	// `launchctl list <label>` คืน exit 0 เมื่อ job ถูก load อยู่
	if err := launchctl("list", LaunchdLabel); err != nil {
		// launchctl list ล้มเหลว แสดงว่า daemon ติดตั้งแล้วแต่ไม่ได้ load
		return "installed (not loaded)", nil
	}
	// plist มีอยู่และ daemon load แล้ว แสดงว่ากำลังรันอยู่
	return "running", nil
}

// RunIfService เป็น no-op บน macOS: launchd รัน binary เป็น foreground process ปกติ
// (ผ่าน `run` command) ดังนั้น loop ปกติจึงใช้ได้เลย ไม่ต้องมี service wrapper
//
// Parameter:
//   - opts: runner.Options (ไม่ได้ใช้บน macOS)
//
// Return: (false, nil) เสมอ บอก caller ว่าให้ใช้ loop ปกติแทน
func RunIfService(_ runner.Options) (bool, error) {
	return false, nil
}

// launchctl รัน launchctl command ด้วย argument ที่กำหนด พร้อม timeout 15 วินาที
// เพื่อป้องกันการค้างเมื่อ launchd ไม่ตอบสนอง
//
// Parameter:
//   - args: argument list ที่จะส่งให้ launchctl (เช่น "load", "-w", plistPath)
//
// Return: error ถ้า launchctl ออกด้วย non-zero exit code หรือ timeout
func launchctl(args ...string) error {
	// สร้าง context พร้อม timeout 15 วินาทีเพื่อป้องกัน launchctl ค้างไม่มีกำหนด
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	// เรียก cancel เสมอเพื่อ release resource เมื่อฟังก์ชันจบ
	defer cancel()
	// รัน launchctl พร้อม capture output ทั้ง stdout และ stderr รวมกัน
	out, err := exec.CommandContext(ctx, "launchctl", args...).CombinedOutput()
	if err != nil {
		// launchctl ออกด้วย error ให้ wrap error พร้อมแสดง argument และ output
		return fmt.Errorf("launchctl %v: %v: %s", args, err, out)
	}
	return nil
}
