// Package device รวบรวมข้อมูลฮาร์ดแวร์/สเปคของเครื่อง (รุ่น, CPU, RAM, disk, GPU,
// firmware, security, แบต, จอ) และสถานะ Windows Update เพื่อแนบไปกับ scan payload
// ให้ backend เก็บและ dashboard แสดงผล
//
// การเลือก implementation ทำตอน compile time ผ่าน build tag เหมือน package scanner:
// device_windows.go เก็บข้อมูลจริงผ่าน WMI/registry/WUA ส่วน device_other.go และ
// device_darwin.go คืน stub Info{Supported:false} ไว้ก่อน (รองรับ macOS ภายหลังได้)
package device

import (
	"context" // ใช้ส่ง context สำหรับ timeout/cancellation ของการเก็บข้อมูล
	"time"    // ใช้บันทึกเวลาที่เก็บข้อมูล
)

// สถานะ Windows Update — ใช้ทั้งฝั่ง agent (คำนวณ) และเป็นค่าที่ส่งขึ้น backend
const (
	WUStatusUpToDate       = "up_to_date"       // ไม่มีอัปเดตค้าง
	WUStatusUpdatesPending = "updates_pending"  // มีอัปเดตค้างอย่างน้อยหนึ่งตัว
	WUStatusRebootPending  = "reboot_pending"   // ลงอัปเดตแล้วแต่ต้อง reboot
	WUStatusUnknown        = "unknown"          // ตรวจไม่ได้ (ยังไม่เคยสแกน / net ล่ม)
)

// Info คือ snapshot ข้อมูลเครื่องหนึ่งครั้ง — Supported=false หมายถึงแพลตฟอร์มนี้
// ยังไม่รองรับการเก็บ (เช่น macOS รอบนี้) field อื่นจะเป็น zero value
type Info struct {
	Supported     bool             // true เฉพาะแพลตฟอร์มที่เก็บข้อมูลจริง (Windows)
	CollectedAt   time.Time        // เวลาที่เก็บ snapshot นี้
	System        System           // ข้อมูลระบบ/รุ่นเครื่องโดยรวม
	CPU           CPU              // ข้อมูลหน่วยประมวลผล
	Memory        Memory           // RAM รวม + ราย DIMM
	Disks         []Disk           // ไดรฟ์เก็บข้อมูลทั้งหมด
	GPUs          []GPU            // การ์ดจอ/หน่วยประมวลผลกราฟิก
	Network       []NetworkAdapter // การ์ดเครือข่ายที่มี MAC จริง
	Firmware      Firmware         // BIOS/UEFI + motherboard
	Security      Security         // Secure Boot + TPM
	Battery       *Battery         // แบต (nil ถ้าเป็นเครื่อง desktop ไม่มีแบต)
	Monitors      []Monitor        // จอที่เชื่อมต่อ
	WindowsUpdate *WindowsUpdate   // สถานะ Windows Update (nil ถ้าตรวจไม่ได้เลย)
}

// System อธิบายรุ่นเครื่องและตัวระบบโดยรวม
type System struct {
	Manufacturer string // ผู้ผลิต เช่น "ASUSTeK COMPUTER INC."
	Model        string // รุ่น เช่น "ROG Strix G15"
	SerialNumber string // serial ของเครื่อง (อาจว่างถ้า OEM ไม่ใส่)
	SystemType   string // เช่น "x64-based PC"
	Domain       string // domain/workgroup ที่ join อยู่
	TotalRAMMB   int64  // RAM รวมที่ OS มองเห็น (MB)
}

// CPU อธิบายหน่วยประมวลผลหลัก (ตัวแรกถ้ามีหลายซ็อกเก็ต)
type CPU struct {
	Model        string // ชื่อรุ่น เช่น "AMD Ryzen 7 6800H"
	Manufacturer string // ผู้ผลิต เช่น "AuthenticAMD"
	Cores        int    // จำนวน physical cores
	LogicalCount int    // จำนวน logical processors (รวม hyper-threading)
	ClockMHz     int    // ความเร็วสูงสุดที่รายงาน (MHz)
	Architecture string // เช่น "x64", "arm64"
}

// Memory รวมขนาด RAM ทั้งหมดและรายละเอียดราย DIMM slot
type Memory struct {
	TotalMB int64          // ผลรวมความจุของทุกแถว (MB)
	Modules []MemoryModule // ราย DIMM ที่ติดตั้งอยู่
}

// MemoryModule คือ RAM หนึ่งแถวใน slot
type MemoryModule struct {
	CapacityMB   int64  // ความจุของแถวนี้ (MB)
	SpeedMHz     int    // ความเร็ว (MHz)
	Manufacturer string // ผู้ผลิตชิป
	PartNumber   string // part number
	Slot         string // ตำแหน่ง slot เช่น "DIMM 0"
}

// Disk คือไดรฟ์เก็บข้อมูลหนึ่งตัว
type Disk struct {
	Model         string // รุ่นไดรฟ์
	SizeGB        int64  // ขนาด (GB)
	MediaType     string // "ssd"/"hdd"/"unknown" (best-effort)
	InterfaceType string // เช่น "NVMe", "SATA", "USB"
	Serial        string // serial ของไดรฟ์ (อาจว่าง)
}

// GPU คือการ์ดจอหนึ่งตัว
type GPU struct {
	Name      string // ชื่อ เช่น "NVIDIA GeForce RTX 3060"
	DriverVer string // เวอร์ชัน driver
	VRAMMB    int64  // หน่วยความจำกราฟิก (MB, best-effort)
}

// NetworkAdapter คือการ์ดเครือข่ายที่มี MAC จริง (กรอง virtual/loopback ออกแล้ว)
type NetworkAdapter struct {
	Name string // ชื่ออุปกรณ์
	MAC  string // MAC address
	Type string // เช่น "Ethernet", "Wireless"
}

// Firmware รวม BIOS/UEFI และข้อมูล motherboard
type Firmware struct {
	BIOSVendor  string // ผู้ผลิต BIOS
	BIOSVersion string // เวอร์ชัน BIOS
	BIOSDate    string // วันที่ release BIOS (string ตามที่ WMI คืนมา)
	Motherboard string // รุ่น baseboard/mainboard
	BoardSerial string // serial ของ baseboard
}

// Security รวมสถานะ Secure Boot และ TPM ซึ่งเป็นสัญญาณ device posture
type Security struct {
	SecureBoot string // "enabled"/"disabled"/"unsupported"/"unknown"
	TPMPresent bool   // มีชิป TPM หรือไม่
	TPMEnabled bool   // TPM ถูกเปิดใช้งานหรือไม่
	TPMVersion string // เวอร์ชัน spec เช่น "2.0" (best-effort)
}

// Battery อธิบายแบตเตอรี่ (เฉพาะ laptop)
type Battery struct {
	Name          string // ชื่อ/รุ่นแบต
	ChargePercent int    // เปอร์เซ็นต์ประจุปัจจุบัน
	Status        string // สถานะที่อ่านได้ เช่น "discharging"
}

// Monitor คือจอที่เชื่อมต่ออยู่
type Monitor struct {
	Name   string // ชื่อรุ่นจอ
	Width  int    // ความกว้าง (px) ถ้าทราบ
	Height int    // ความสูง (px) ถ้าทราบ
}

// WindowsUpdate คือสถานะการอัปเดตของ Windows ณ เวลาที่เก็บ
type WindowsUpdate struct {
	Status          string          // ดูค่าคงที่ WUStatus* ด้านบน
	PendingCount    int             // จำนวนอัปเดตที่ค้างอยู่ทั้งหมด
	SecurityPending int             // จำนวนในนั้นที่เป็น security/critical
	RebootPending   bool            // ค้าง reboot จากอัปเดตที่ลงไปแล้ว
	LastInstalledKB string          // KB ของอัปเดตล่าสุดที่ลงสำเร็จ
	LastInstalledAt string          // เวลาที่ลงอัปเดตล่าสุด (string ตามแหล่งข้อมูล)
	LastCheckedAt   string          // เวลาที่ WUA online scan รันล่าสุด (RFC3339)
	Source          string          // "online"/"cached"/"lightweight" — ที่มาของข้อมูลค้าง
	Pending         []PendingUpdate // รายการอัปเดตที่ค้าง (จาก WUA scan)
}

// PendingUpdate คืออัปเดตที่ค้างหนึ่งตัวจากผล WUA search
type PendingUpdate struct {
	KB       string // เลข KB เช่น "KB5034123" (อาจว่างถ้า driver update)
	Title    string // ชื่อเต็มของอัปเดต
	Security bool   // เป็นอัปเดตด้าน security หรือไม่
	Severity string // ความรุนแรงจาก MSRC เช่น "Critical"/"Important"
}

// Collect คืนข้อมูลเครื่องของแพลตฟอร์มปัจจุบัน — เป็นจุดเรียกใช้สาธารณะจุดเดียว
// best-effort: field ที่อ่านไม่ได้จะถูกเว้นว่าง ไม่ทำให้ทั้ง snapshot ล้มเหลว
// platform implementation อยู่ในฟังก์ชัน collect() ที่เลือกด้วย build tag
func Collect(ctx context.Context) Info {
	info := collect(ctx)
	if info.CollectedAt.IsZero() {
		info.CollectedAt = time.Now()
	}
	return info
}

// DeriveWUStatus คำนวณสถานะ Windows Update จากข้อมูลที่เก็บได้ — แยกออกมาเป็น
// pure function (ไม่ผูกกับ COM/registry) เพื่อให้ test ได้ทุกแพลตฟอร์ม
//
// known = เคยได้ผล WUA scan (online หรือ cached) จึงเชื่อถือ "ไม่มีค้าง" ได้
// ลำดับความสำคัญ: reboot ค้าง > ไม่ทราบ > มีอัปเดตค้าง > เป็นปัจจุบัน
func DeriveWUStatus(known bool, pending []PendingUpdate, rebootPending bool) string {
	if rebootPending {
		return WUStatusRebootPending
	}
	if !known {
		return WUStatusUnknown
	}
	if len(pending) > 0 {
		return WUStatusUpdatesPending
	}
	return WUStatusUpToDate
}

// CountSecurityPending นับจำนวนอัปเดตค้างที่เป็น security
func CountSecurityPending(pending []PendingUpdate) int {
	n := 0
	for _, p := range pending {
		if p.Security {
			n++
		}
	}
	return n
}
