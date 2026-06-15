// แพ็กเกจ config_test ทดสอบฟังก์ชันใน package config
// ใช้ white-box testing (อยู่ใน package เดียวกัน) เพื่อทดสอบ internal state
package config

import (
	"os"           // ใช้ลบไฟล์ในเทสต์ TestLoadMissingReturnsDefault
	"path/filepath" // ใช้สร้าง path ของไดเรกทอรีชั่วคราว
	"testing"      // framework สำหรับเขียน unit test ใน Go
)

// TestDefaults ตรวจสอบว่า Default() คืนค่าเริ่มต้นที่ถูกต้อง
// ได้แก่ ScanIntervalHours=6, AutoUpdateEnabled=true, LogLevel="info"
func TestDefaults(t *testing.T) {
	// เรียก Default() เพื่อรับค่า config เริ่มต้น
	c := Default()
	// ตรวจสอบว่า ScanIntervalHours ค่าเริ่มต้นเป็น 6 ชั่วโมง
	if c.ScanIntervalHours != 6 {
		t.Errorf("want 6h default, got %d", c.ScanIntervalHours)
	}
	// ตรวจสอบว่า AutoUpdateEnabled ค่าเริ่มต้นเป็น true
	if !c.AutoUpdateEnabled {
		t.Errorf("auto-update should default true")
	}
	// ตรวจสอบว่า LogLevel ค่าเริ่มต้นเป็น "info"
	if c.LogLevel != "info" {
		t.Errorf("log level default should be info, got %q", c.LogLevel)
	}
}

// TestSaveAndLoad ตรวจสอบ round-trip: บันทึก config ลงดิสก์แล้วอ่านขึ้นมา
// ต้องได้ค่าเหมือนเดิมทุกฟิลด์
func TestSaveAndLoad(t *testing.T) {
	// สร้างไดเรกทอรีชั่วคราวสำหรับเทสต์ จะถูกลบอัตโนมัติหลังเทสต์จบ
	tmp := t.TempDir()
	// กำหนด HOME ให้ชี้ไปที่ไดเรกทอรีชั่วคราว เพื่อไม่ให้เทสต์กระทบไฟล์จริง
	t.Setenv("HOME", tmp)
	// กำหนด ProgramData ให้ชี้ไปที่ไดเรกทอรีชั่วคราวบน Windows
	t.Setenv("ProgramData", filepath.Join(tmp, "ProgramData"))

	// สร้าง config ด้วยค่าเริ่มต้นก่อน แล้วแก้ไขบางฟิลด์
	cfg := Default()
	cfg.ServerURL = "https://example.test"                         // กำหนด URL ของ server ทดสอบ
	cfg.MachineUUID = "11111111-2222-3333-4444-555555555555"        // กำหนด UUID ของเครื่องทดสอบ
	cfg.ScanIntervalHours = 12                                      // เปลี่ยนเป็นสแกนทุก 12 ชั่วโมง

	// บันทึก config ลงดิสก์ ถ้าล้มเหลวให้หยุดเทสต์ทันที
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// อ่าน config จากดิสก์กลับขึ้นมา ถ้าล้มเหลวให้หยุดเทสต์ทันที
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// ตรวจสอบว่าฟิลด์ ServerURL และ MachineUUID ตรงกับที่บันทึกไว้
	if loaded.ServerURL != cfg.ServerURL || loaded.MachineUUID != cfg.MachineUUID {
		t.Errorf("loaded mismatch: %+v", loaded)
	}
	// ตรวจสอบว่า ScanIntervalHours ถูกบันทึกและอ่านกลับมาได้ถูกต้อง
	if loaded.ScanIntervalHours != 12 {
		t.Errorf("scan interval not persisted")
	}
}

// TestLoadMissingReturnsDefault ตรวจสอบว่าเมื่อไม่มีไฟล์ config
// Load() จะคืนค่า Default() แทน โดยไม่คืน error
func TestLoadMissingReturnsDefault(t *testing.T) {
	// สร้างไดเรกทอรีชั่วคราวสำหรับเทสต์
	tmp := t.TempDir()
	// กำหนด HOME ให้ชี้ไปที่ไดเรกทอรีชั่วคราว
	t.Setenv("HOME", tmp)
	// กำหนด ProgramData ให้ชี้ไปที่ไดเรกทอรีชั่วคราวบน Windows
	t.Setenv("ProgramData", filepath.Join(tmp, "ProgramData"))

	// ดึง path ของไฟล์ config เพื่อนำมาลบ
	p, _ := Path()
	// ลบไฟล์ config ถ้ามีอยู่ เพื่อจำลองสถานะที่ยังไม่มี config
	_ = os.Remove(p)

	// เรียก Load() กับ config ที่ไม่มีไฟล์ ควรได้ค่า Default() คืนมา
	c, err := Load()
	// ต้องไม่มี error เพราะไฟล์ไม่มีถือว่า OK สำหรับ Load()
	if err != nil {
		t.Fatalf("expected no error for missing file: %v", err)
	}
	// ตรวจสอบว่า ScanIntervalHours คือค่า default (6 ชั่วโมง)
	if c.ScanIntervalHours != 6 {
		t.Errorf("expected default scan interval, got %d", c.ScanIntervalHours)
	}
}

// TestParseOldConfigKeepsFilesystemDefaults ตรวจสอบว่า config เก่าที่ไม่มี key
// filesystem ยังได้ค่า default ที่เปิด filesystem scan ไว้ (backward compatible)
func TestParseOldConfigKeepsFilesystemDefaults(t *testing.T) {
	old := []byte("server_url: http://example\nscan_interval_hours: 6\n")
	cfg, err := parse(old)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.FilesystemScanEnabled {
		t.Error("FilesystemScanEnabled: want true for a config missing the key")
	}
	if cfg.FilesystemDeepMode {
		t.Error("FilesystemDeepMode: want false default")
	}
	if cfg.FirstScanTimeoutMinutes != 15 {
		t.Errorf("FirstScanTimeoutMinutes: want 15, got %d", cfg.FirstScanTimeoutMinutes)
	}
}

// TestParseHonorsExplicitDisable ตรวจสอบว่าเมื่อ config ปิด filesystem scan
// อย่างชัดเจน ค่าที่ parse ได้ต้องเป็น false (ไม่ถูก default ทับ)
func TestParseHonorsExplicitDisable(t *testing.T) {
	cfg, err := parse([]byte("filesystem_scan_enabled: false\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.FilesystemScanEnabled {
		t.Error("FilesystemScanEnabled: want false when explicitly set")
	}
}
