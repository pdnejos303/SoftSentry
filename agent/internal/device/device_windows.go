//go:build windows

// device_windows.go เก็บข้อมูลฮาร์ดแวร์จริงผ่าน WMI + registry และ orchestrate
// การตรวจ Windows Update แบบสองชั้น (lightweight registry + WUA online scan ที่
// throttle ไว้ 24 ชม. ดู windowsupdate.go) ทุกการอ่านเป็น best-effort: ส่วนที่
// อ่านไม่ได้จะถูกเว้นว่าง ไม่ทำให้ทั้ง snapshot ล้มเหลว
package device

import (
	"context" // ใช้ส่ง timeout/cancellation ลงไปยัง WUA scan
	"strings" // ใช้ normalize ค่า string จาก WMI
	"time"    // ใช้ stamp เวลาและคำนวณ throttle

	"github.com/softsentry/agent/internal/storage" // ใช้ cache ผล WUA online scan
	"github.com/yusufpapurcu/wmi"                   // pure-Go WMI client (CGO-free)
	"golang.org/x/sys/windows/registry"             // ใช้อ่าน Secure Boot / reboot-pending / last install
)

// WUAThrottle คือระยะเวลาขั้นต่ำระหว่างการยิง WUA online scan จริง (หนัก + ต้องต่อเน็ต)
// scan รอบที่ยังไม่ครบกำหนดจะใช้ผล cache แทน
const WUAThrottle = 24 * time.Hour

// wuaScanTimeout จำกัดเวลาของ WUA online scan หนึ่งครั้ง (COM Search บล็อกยาวได้)
const wuaScanTimeout = 3 * time.Minute

// collect เก็บข้อมูลเครื่องทั้งหมดบน Windows
func collect(ctx context.Context) Info {
	info := Info{Supported: true, CollectedAt: time.Now()}
	info.System = wmiSystem()
	info.CPU = wmiCPU()
	info.Memory = wmiMemory()
	info.Disks = wmiDisks()
	info.GPUs = wmiGPUs()
	info.Network = wmiNetwork()
	info.Firmware = wmiFirmware()
	info.Security = readSecurity()
	info.Battery = wmiBattery()
	info.Monitors = wmiMonitors()
	info.WindowsUpdate = collectWindowsUpdate(ctx)
	return info
}

// ── WMI query structs (ชื่อ field ต้องตรงกับ property ของ WMI class) ──────────────

type win32ComputerSystem struct {
	Manufacturer              string
	Model                     string
	SystemType                string
	Domain                    string
	TotalPhysicalMemory       uint64
	NumberOfLogicalProcessors uint32
}

type win32ComputerSystemProduct struct {
	IdentifyingNumber string
}

type win32Processor struct {
	Name                      string
	Manufacturer              string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
	MaxClockSpeed             uint32
	Architecture              uint16
}

type win32PhysicalMemory struct {
	Capacity      uint64
	Speed         uint32
	Manufacturer  string
	PartNumber    string
	DeviceLocator string
}

type win32DiskDrive struct {
	Model         string
	Size          uint64
	InterfaceType string
	SerialNumber  string
}

type win32VideoController struct {
	Name          string
	DriverVersion string
	AdapterRAM    uint32
}

type win32BIOS struct {
	Manufacturer      string
	SMBIOSBIOSVersion string
	ReleaseDate       string
}

type win32BaseBoard struct {
	Product      string
	SerialNumber string
}

type win32NetworkAdapter struct {
	Name            string
	MACAddress      string
	AdapterType     string
	PhysicalAdapter bool
	NetConnectionID string
}

type win32DesktopMonitor struct {
	Name         string
	ScreenWidth  uint32
	ScreenHeight uint32
}

type win32Battery struct {
	Name                     string
	EstimatedChargeRemaining uint16
	BatteryStatus            uint16
}

type win32Tpm struct {
	IsEnabled_InitialValue   bool   //nolint:revive,stylecheck // ชื่อ property ตาม WMI
	IsActivated_InitialValue bool   //nolint:revive,stylecheck // ชื่อ property ตาม WMI
	SpecVersion              string //nolint:revive,stylecheck // ชื่อ property ตาม WMI
}

// ── WMI collectors (แต่ละตัว best-effort: error → คืน zero value) ─────────────────

func wmiSystem() System {
	var rows []win32ComputerSystem
	s := System{}
	if err := wmi.Query("SELECT Manufacturer, Model, SystemType, Domain, TotalPhysicalMemory, NumberOfLogicalProcessors FROM Win32_ComputerSystem", &rows); err == nil && len(rows) > 0 {
		r := rows[0]
		s.Manufacturer = strings.TrimSpace(r.Manufacturer)
		s.Model = strings.TrimSpace(r.Model)
		s.SystemType = strings.TrimSpace(r.SystemType)
		s.Domain = strings.TrimSpace(r.Domain)
		s.TotalRAMMB = bytesToMB(r.TotalPhysicalMemory)
	}
	// serial มาจาก Win32_ComputerSystemProduct (IdentifyingNumber)
	var prod []win32ComputerSystemProduct
	if err := wmi.Query("SELECT IdentifyingNumber FROM Win32_ComputerSystemProduct", &prod); err == nil && len(prod) > 0 {
		s.SerialNumber = strings.TrimSpace(prod[0].IdentifyingNumber)
	}
	return s
}

func wmiCPU() CPU {
	var rows []win32Processor
	c := CPU{}
	if err := wmi.Query("SELECT Name, Manufacturer, NumberOfCores, NumberOfLogicalProcessors, MaxClockSpeed, Architecture FROM Win32_Processor", &rows); err == nil && len(rows) > 0 {
		r := rows[0]
		c.Model = strings.TrimSpace(r.Name)
		c.Manufacturer = strings.TrimSpace(r.Manufacturer)
		c.Cores = int(r.NumberOfCores)
		c.LogicalCount = int(r.NumberOfLogicalProcessors)
		c.ClockMHz = int(r.MaxClockSpeed)
		c.Architecture = cpuArch(r.Architecture)
	}
	return c
}

func wmiMemory() Memory {
	var rows []win32PhysicalMemory
	m := Memory{}
	if err := wmi.Query("SELECT Capacity, Speed, Manufacturer, PartNumber, DeviceLocator FROM Win32_PhysicalMemory", &rows); err == nil {
		for _, r := range rows {
			cap := bytesToMB(r.Capacity)
			m.TotalMB += cap
			m.Modules = append(m.Modules, MemoryModule{
				CapacityMB:   cap,
				SpeedMHz:     int(r.Speed),
				Manufacturer: strings.TrimSpace(r.Manufacturer),
				PartNumber:   strings.TrimSpace(r.PartNumber),
				Slot:         strings.TrimSpace(r.DeviceLocator),
			})
		}
	}
	return m
}

func wmiDisks() []Disk {
	var rows []win32DiskDrive
	var disks []Disk
	if err := wmi.Query("SELECT Model, Size, InterfaceType, SerialNumber FROM Win32_DiskDrive", &rows); err == nil {
		for _, r := range rows {
			disks = append(disks, Disk{
				Model:         strings.TrimSpace(r.Model),
				SizeGB:        bytesToGB(r.Size),
				InterfaceType: strings.TrimSpace(r.InterfaceType),
				Serial:        strings.TrimSpace(r.SerialNumber),
				MediaType:     "unknown", // Win32_DiskDrive แยก SSD/HDD ไม่ได้ — เติมจาก storage namespace ด้านล่าง
			})
		}
	}
	annotateDiskMedia(disks) // best-effort: เติม ssd/hdd จาก MSFT_PhysicalDisk
	return disks
}

func wmiGPUs() []GPU {
	var rows []win32VideoController
	var gpus []GPU
	if err := wmi.Query("SELECT Name, DriverVersion, AdapterRAM FROM Win32_VideoController", &rows); err == nil {
		for _, r := range rows {
			gpus = append(gpus, GPU{
				Name:      strings.TrimSpace(r.Name),
				DriverVer: strings.TrimSpace(r.DriverVersion),
				VRAMMB:    bytesToMB(uint64(r.AdapterRAM)),
			})
		}
	}
	return gpus
}

func wmiNetwork() []NetworkAdapter {
	var rows []win32NetworkAdapter
	var nics []NetworkAdapter
	if err := wmi.Query("SELECT Name, MACAddress, AdapterType, PhysicalAdapter, NetConnectionID FROM Win32_NetworkAdapter", &rows); err == nil {
		for _, r := range rows {
			// เก็บเฉพาะการ์ดจริงที่มี MAC (กรอง virtual/loopback/tunnel ออก)
			if !r.PhysicalAdapter || strings.TrimSpace(r.MACAddress) == "" {
				continue
			}
			name := strings.TrimSpace(r.NetConnectionID)
			if name == "" {
				name = strings.TrimSpace(r.Name)
			}
			nics = append(nics, NetworkAdapter{
				Name: name,
				MAC:  strings.TrimSpace(r.MACAddress),
				Type: strings.TrimSpace(r.AdapterType),
			})
		}
	}
	return nics
}

func wmiFirmware() Firmware {
	f := Firmware{}
	var bios []win32BIOS
	if err := wmi.Query("SELECT Manufacturer, SMBIOSBIOSVersion, ReleaseDate FROM Win32_BIOS", &bios); err == nil && len(bios) > 0 {
		f.BIOSVendor = strings.TrimSpace(bios[0].Manufacturer)
		f.BIOSVersion = strings.TrimSpace(bios[0].SMBIOSBIOSVersion)
		f.BIOSDate = cimDate(bios[0].ReleaseDate)
	}
	var board []win32BaseBoard
	if err := wmi.Query("SELECT Product, SerialNumber FROM Win32_BaseBoard", &board); err == nil && len(board) > 0 {
		f.Motherboard = strings.TrimSpace(board[0].Product)
		f.BoardSerial = strings.TrimSpace(board[0].SerialNumber)
	}
	return f
}

func wmiBattery() *Battery {
	var rows []win32Battery
	if err := wmi.Query("SELECT Name, EstimatedChargeRemaining, BatteryStatus FROM Win32_Battery", &rows); err != nil || len(rows) == 0 {
		return nil // desktop / ไม่มีแบต
	}
	return &Battery{
		Name:          strings.TrimSpace(rows[0].Name),
		ChargePercent: int(rows[0].EstimatedChargeRemaining),
		Status:        batteryStatus(rows[0].BatteryStatus),
	}
}

func wmiMonitors() []Monitor {
	var rows []win32DesktopMonitor
	var mons []Monitor
	if err := wmi.Query("SELECT Name, ScreenWidth, ScreenHeight FROM Win32_DesktopMonitor", &rows); err == nil {
		for _, r := range rows {
			mons = append(mons, Monitor{
				Name:   strings.TrimSpace(r.Name),
				Width:  int(r.ScreenWidth),
				Height: int(r.ScreenHeight),
			})
		}
	}
	return mons
}

// msftPhysicalDisk อยู่ใน namespace root\Microsoft\Windows\Storage และบอก SSD/HDD ได้
type msftPhysicalDisk struct {
	FriendlyName string
	MediaType    uint16 // 3=HDD, 4=SSD, 5=SCM
	BusType      uint16 // 17=NVMe, 11=SATA, 7=USB, ...
	SerialNumber string
}

// annotateDiskMedia เติม MediaType/InterfaceType ที่แม่นขึ้นจาก MSFT_PhysicalDisk
// จับคู่กับ disks เดิมแบบ best-effort (ตาม serial; ถ้าไม่ตรงใช้ลำดับ)
func annotateDiskMedia(disks []Disk) {
	if len(disks) == 0 {
		return
	}
	var rows []msftPhysicalDisk
	if err := wmi.QueryNamespace("SELECT FriendlyName, MediaType, BusType, SerialNumber FROM MSFT_PhysicalDisk", &rows, `root\Microsoft\Windows\Storage`); err != nil {
		return // namespace ไม่มี (Windows เก่า) → คง "unknown"
	}
	for i := range disks {
		var match *msftPhysicalDisk
		for j := range rows {
			if rows[j].SerialNumber != "" && strings.EqualFold(strings.TrimSpace(rows[j].SerialNumber), disks[i].Serial) {
				match = &rows[j]
				break
			}
		}
		if match == nil && i < len(rows) {
			match = &rows[i] // fallback ตามลำดับเมื่อ serial ไม่ตรง
		}
		if match != nil {
			disks[i].MediaType = mediaType(match.MediaType)
			if bt := busType(match.BusType); bt != "" {
				disks[i].InterfaceType = bt
			}
		}
	}
}

// readSecurity อ่าน Secure Boot (registry) + TPM (WMI namespace security)
func readSecurity() Security {
	s := Security{SecureBoot: "unknown"}
	// Secure Boot: HKLM\SYSTEM\CurrentControlSet\Control\SecureBoot\State\UEFISecureBootEnabled
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\SecureBoot\State`, registry.QUERY_VALUE); err == nil {
		if v, _, err := k.GetIntegerValue("UEFISecureBootEnabled"); err == nil {
			if v == 1 {
				s.SecureBoot = "enabled"
			} else {
				s.SecureBoot = "disabled"
			}
		}
		_ = k.Close()
	} else {
		// key ไม่มี = เครื่อง legacy BIOS ไม่รองรับ Secure Boot
		s.SecureBoot = "unsupported"
	}
	// TPM: namespace root\CIMV2\Security\MicrosoftTpm
	var tpm []win32Tpm
	if err := wmi.QueryNamespace("SELECT IsEnabled_InitialValue, IsActivated_InitialValue, SpecVersion FROM Win32_Tpm", &tpm, `root\CIMV2\Security\MicrosoftTpm`); err == nil && len(tpm) > 0 {
		s.TPMPresent = true
		s.TPMEnabled = tpm[0].IsEnabled_InitialValue
		// SpecVersion เช่น "2.0, 0, 1.38" — เอาเฉพาะตัวแรก
		if parts := strings.SplitN(tpm[0].SpecVersion, ",", 2); len(parts) > 0 {
			s.TPMVersion = strings.TrimSpace(parts[0])
		}
	}
	return s
}

// ── Windows Update orchestration ────────────────────────────────────────────────

// collectWindowsUpdate รวมข้อมูล WU สองชั้น: lightweight (registry, ทุกครั้ง) +
// WUA online scan (throttle 24 ชม., cache บนดิสก์)
func collectWindowsUpdate(ctx context.Context) *WindowsUpdate {
	wu := &WindowsUpdate{}
	wu.RebootPending = readRebootPending()
	wu.LastInstalledAt = readLastInstallTime()

	cache, found, _ := storage.LoadWUACache()
	fresh := found && time.Since(cache.CheckedAt) < WUAThrottle
	known := false

	switch {
	case fresh:
		// ยังไม่ครบกำหนดยิงใหม่ → ใช้ cache
		wu.Pending = fromCache(cache.Pending)
		wu.LastCheckedAt = cache.CheckedAt.UTC().Format(time.RFC3339)
		wu.Source = "cached"
		known = true
	default:
		// ครบกำหนด (หรือยังไม่เคย) → ยิง WUA online scan จริง
		res, err := runWUAScan(ctx)
		if err != nil {
			if found {
				// net ล่ม/timeout แต่มี cache เก่า → ใช้ของเก่าไปก่อน
				wu.Pending = fromCache(cache.Pending)
				wu.LastCheckedAt = cache.CheckedAt.UTC().Format(time.RFC3339)
				wu.Source = "cached"
				known = true
			} else {
				wu.Source = "lightweight" // ไม่มีอะไรเลย → รู้แค่ reboot-pending
			}
		} else {
			now := time.Now()
			wu.Pending = res.Pending
			if res.LastInstalledKB != "" {
				wu.LastInstalledKB = res.LastInstalledKB
			}
			if res.LastInstalledAt != "" {
				wu.LastInstalledAt = res.LastInstalledAt
			}
			wu.LastCheckedAt = now.UTC().Format(time.RFC3339)
			wu.Source = "online"
			known = true
			_ = storage.SaveWUACache(toCache(res.Pending, now)) // best-effort
		}
	}

	wu.PendingCount = len(wu.Pending)
	wu.SecurityPending = CountSecurityPending(wu.Pending)
	wu.Status = DeriveWUStatus(known, wu.Pending, wu.RebootPending)
	return wu
}

// readRebootPending คืน true ถ้ามี key บ่งชี้ว่าค้าง reboot จากอัปเดต
func readRebootPending() bool {
	keys := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`,
	}
	for _, key := range keys {
		if k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE); err == nil {
			_ = k.Close()
			return true
		}
	}
	return false
}

// readLastInstallTime อ่านเวลาที่ Windows Update ลงสำเร็จล่าสุดจาก registry (lightweight)
func readLastInstallTime() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\Results\Install`,
		registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("LastSuccessTime") // รูปแบบ "2026-06-20 03:14:55"
	return strings.TrimSpace(v)
}

// ── helper: แปลง cache ↔ device types + แปลงค่า WMI ─────────────────────────────

func fromCache(in []storage.WUAPending) []PendingUpdate {
	out := make([]PendingUpdate, 0, len(in))
	for _, p := range in {
		out = append(out, PendingUpdate{KB: p.KB, Title: p.Title, Security: p.Security, Severity: p.Severity})
	}
	return out
}

func toCache(in []PendingUpdate, checkedAt time.Time) storage.WUACache {
	out := make([]storage.WUAPending, 0, len(in))
	for _, p := range in {
		out = append(out, storage.WUAPending{KB: p.KB, Title: p.Title, Security: p.Security, Severity: p.Severity})
	}
	return storage.WUACache{CheckedAt: checkedAt, Pending: out}
}

func bytesToMB(b uint64) int64 { return int64(b / (1024 * 1024)) }
func bytesToGB(b uint64) int64 { return int64(b / (1024 * 1024 * 1024)) }

func cpuArch(a uint16) string {
	switch a {
	case 0:
		return "x86"
	case 5:
		return "arm"
	case 9:
		return "x64"
	case 12:
		return "arm64"
	default:
		return "unknown"
	}
}

func mediaType(m uint16) string {
	switch m {
	case 3:
		return "hdd"
	case 4:
		return "ssd"
	case 5:
		return "scm"
	default:
		return "unknown"
	}
}

func busType(b uint16) string {
	switch b {
	case 7:
		return "USB"
	case 11:
		return "SATA"
	case 17:
		return "NVMe"
	case 8:
		return "RAID"
	case 10:
		return "SAS"
	default:
		return ""
	}
}

func batteryStatus(s uint16) string {
	switch s {
	case 1:
		return "discharging"
	case 2:
		return "ac"
	case 3:
		return "fully_charged"
	case 4:
		return "low"
	case 5:
		return "critical"
	default:
		return "unknown"
	}
}

// cimDate แปลง CIM_DATETIME (เช่น "20260101000000.000000+000") เป็น "2026-01-01"
func cimDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return s
	}
	return s[0:4] + "-" + s[4:6] + "-" + s[6:8]
}
