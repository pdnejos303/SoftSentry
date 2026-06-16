// Package config loads, persists, and represents the agent's on-disk config.
// (TH) แพ็กเกจ config โหลด บันทึก และเก็บค่า config ของ agent ที่อยู่บนดิสก์
package config

import (
	"errors"        // ใช้ตรวจสอบประเภท error เช่น os.ErrNotExist
	"fmt"           // ใช้จัดรูปแบบ error message
	"os"            // ใช้อ่าน/เขียนไฟล์ และอ่าน environment variable
	"path/filepath" // ใช้สร้าง path ของไฟล์/ไดเรกทอรีแบบ cross-platform
	"runtime"       // ใช้ตรวจสอบ OS ที่รันอยู่ (windows หรือ อื่นๆ)

	"gopkg.in/yaml.v3" // ใช้ marshal/unmarshal ไฟล์ config แบบ YAML
)

// Config is the on-disk YAML config.
// (TH) Config คือโครงสร้างข้อมูล config ของ agent ที่บันทึกอยู่บนดิสก์ในรูปแบบ YAML
type Config struct {
	ServerURL         string `yaml:"server_url"`          // URL ของ backend server ที่ agent จะรายงานผล
	MachineUUID       string `yaml:"machine_uuid"`        // UUID เฉพาะของเครื่องนี้ที่ลงทะเบียนไว้กับ server
	ScanIntervalHours int    `yaml:"scan_interval_hours"` // ความถี่การสแกน software inventory (หน่วย: ชั่วโมง)
	AutoUpdateEnabled bool   `yaml:"auto_update_enabled"` // เปิด/ปิดการอัปเดต agent อัตโนมัติ
	LogLevel          string `yaml:"log_level"`           // ระดับ log ที่ต้องการ เช่น "info", "debug", "warn"

	// FilesystemScanEnabled เปิดการเดิน filesystem หา .exe นอก registry (default true)
	FilesystemScanEnabled bool `yaml:"filesystem_scan_enabled"`
	// FilesystemDeepMode สแกนทั้งไดรฟ์ระบบรวม Windows (default false = curated roots เท่านั้น)
	FilesystemDeepMode bool `yaml:"filesystem_deep_mode"`
	// FilesystemExtraRoots เพิ่ม root ที่จะเดินเองได้ (default [])
	FilesystemExtraRoots []string `yaml:"filesystem_extra_roots"`
	// FirstScanTimeoutMinutes งบเวลาของสแกนรอบแรก (cache ว่าง) เป็นนาที (default 15)
	FirstScanTimeoutMinutes int `yaml:"first_scan_timeout_minutes"`
}

// Default returns a Config initialized with sensible defaults.
// (TH) Default คืนค่า Config ที่ตั้งค่าเริ่มต้นแบบสมเหตุสมผล
// สแกนทุก 6 ชั่วโมง, เปิด auto-update, log ระดับ info
func Default() *Config {
	return &Config{
		ScanIntervalHours:       6,      // สแกน software inventory ทุก 6 ชั่วโมงโดยค่าเริ่มต้น
		AutoUpdateEnabled:       true,   // เปิดใช้งาน auto-update โดยค่าเริ่มต้น
		LogLevel:                "info", // log ระดับ info โดยค่าเริ่มต้น
		FilesystemScanEnabled:   true,   // เดิน filesystem หา .exe นอก registry โดยค่าเริ่มต้น
		FirstScanTimeoutMinutes: 15,     // งบเวลาสแกนรอบแรก 15 นาที (รอบถัดไปใช้ 2 นาทีใน runner)
	}
}

// Dir returns the per-OS config directory, creating it if missing.
// (TH) Dir คืนค่า path ของไดเรกทอรี config ตาม OS ที่รันอยู่
// บน Windows ใช้ %ProgramData%\SoftSentry, บน macOS/Linux ใช้ ~/.softsentry
// ถ้าไดเรกทอรียังไม่มีจะสร้างให้อัตโนมัติ
func Dir() (string, error) {
	var dir string
	// ตรวจสอบว่ารันบน Windows หรือไม่เพื่อเลือก path ที่เหมาะสม
	if runtime.GOOS == "windows" {
		// อ่าน path ของ ProgramData จาก environment variable
		programData := os.Getenv("ProgramData")
		// ถ้าไม่มีค่า ProgramData ให้ใช้ path เริ่มต้นของ Windows
		if programData == "" {
			programData = `C:\ProgramData`
		}
		// สร้าง path เป็น %ProgramData%\SoftSentry
		dir = filepath.Join(programData, "SoftSentry")
	} else {
		// บน macOS/Linux อ่าน home directory ของผู้ใช้ปัจจุบัน
		home, err := os.UserHomeDir()
		// ถ้าหา home directory ไม่ได้ให้คืน error
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		// สร้าง path เป็น ~/.softsentry
		dir = filepath.Join(home, ".softsentry")
	}
	// สร้างไดเรกทอรี config พร้อม parent directories ถ้ายังไม่มี
	// สิทธิ์ 0o700 คืออนุญาตเฉพาะเจ้าของไฟล์เท่านั้น (อ่าน/เขียน/execute)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", dir, err)
	}
	return dir, nil
}

// Path returns the YAML config path.
// (TH) Path คืนค่า path เต็มของไฟล์ config.yaml
// โดยนำ Dir() มาต่อกับชื่อไฟล์ "config.yaml"
func Path() (string, error) {
	// ดึง path ของไดเรกทอรี config ก่อน
	dir, err := Dir()
	// ถ้าหาไดเรกทอรีไม่ได้ให้คืน error ทันที
	if err != nil {
		return "", err
	}
	// ประกอบ path เต็มของไฟล์ config.yaml
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads config from disk; returns Default if the file is missing.
// (TH) Load อ่านค่า config จากไฟล์บนดิสก์
// ถ้าไม่พบไฟล์ config จะคืนค่า Default() แทน (ไม่ถือเป็น error)
// ถ้าไฟล์มีอยู่แต่ parse ไม่ได้จึงจะคืน error
func Load() (*Config, error) {
	// ดึง path ของไฟล์ config
	p, err := Path()
	// ถ้าหา path ไม่ได้ให้คืน error
	if err != nil {
		return nil, err
	}
	// อ่านเนื้อหาไฟล์ config ทั้งหมด
	// #nosec G304 — path มาจากการคำนวณภายใน ไม่ได้รับมาจากผู้ใช้ จึงปลอดภัย
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		// ถ้าไม่พบไฟล์ (ErrNotExist) ให้คืนค่า Default แทนการคืน error
		// เพราะเป็นเรื่องปกติที่ agent ยังไม่เคยถูก configure
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		// error อื่นๆ ที่ไม่ใช่ไฟล์ไม่มี ให้คืน error กลับไป
		return nil, fmt.Errorf("read config: %w", err)
	}
	// แปลงข้อมูล YAML ทับลงบนค่า Default (ฟิลด์ไหนไม่มีใน YAML จะยังคงค่า default)
	return parse(data)
}

// parse overlays YAML data onto a fresh Default config. Fields absent from the
// YAML keep their default, so configs written before new fields existed still
// get sensible values (e.g. FilesystemScanEnabled stays true).
// (TH) parse ทับค่า YAML ลงบน Default ฟิลด์ที่ไม่มีใน YAML จะคงค่า default ไว้
func parse(data []byte) (*Config, error) {
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes config back to disk (0600).
// (TH) Save เขียนค่า config กลับลงดิสก์
// ใช้สิทธิ์ไฟล์ 0600 คืออนุญาตเฉพาะเจ้าของอ่าน/เขียนเท่านั้น
// เพราะ config อาจมีข้อมูลสำคัญเช่น server URL
func Save(cfg *Config) error {
	// ดึง path ของไฟล์ config
	p, err := Path()
	// ถ้าหา path ไม่ได้ให้คืน error
	if err != nil {
		return err
	}
	// แปลง struct Config เป็น YAML bytes
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	// เขียนข้อมูล YAML ลงไฟล์ด้วยสิทธิ์ 0600 (เฉพาะเจ้าของอ่าน/เขียน)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
