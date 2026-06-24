package runner

import (
	"github.com/softsentry/agent/internal/device"    // type ต้นทาง (device.Info)
	"github.com/softsentry/agent/internal/transport" // type ปลายทาง (wire payload)
)

// deviceToWire แปลง device.Info (เก็บจากเครื่อง) เป็น transport.DeviceInfo (wire
// payload ที่ backend คาดหวัง) แยกออกมาเป็นไฟล์ของตัวเองเพื่อให้ runner.go โฟกัส
// ที่ลูปการทำงานหลัก การแปลงเป็น field-by-field เพื่อ type safety
func deviceToWire(info device.Info) *transport.DeviceInfo {
	d := &transport.DeviceInfo{
		System: transport.DeviceSystem{
			Manufacturer: info.System.Manufacturer,
			Model:        info.System.Model,
			SerialNumber: info.System.SerialNumber,
			SystemType:   info.System.SystemType,
			Domain:       info.System.Domain,
			TotalRAMMB:   info.System.TotalRAMMB,
		},
		CPU: transport.DeviceCPU{
			Model:        info.CPU.Model,
			Manufacturer: info.CPU.Manufacturer,
			Cores:        info.CPU.Cores,
			LogicalCount: info.CPU.LogicalCount,
			ClockMHz:     info.CPU.ClockMHz,
			Architecture: info.CPU.Architecture,
		},
		Memory: transport.DeviceMemory{TotalMB: info.Memory.TotalMB},
		Firmware: transport.DeviceFirmware{
			BIOSVendor:  info.Firmware.BIOSVendor,
			BIOSVersion: info.Firmware.BIOSVersion,
			BIOSDate:    info.Firmware.BIOSDate,
			Motherboard: info.Firmware.Motherboard,
			BoardSerial: info.Firmware.BoardSerial,
		},
		Security: transport.DeviceSecurity{
			SecureBoot: info.Security.SecureBoot,
			TPMPresent: info.Security.TPMPresent,
			TPMEnabled: info.Security.TPMEnabled,
			TPMVersion: info.Security.TPMVersion,
		},
	}

	for _, m := range info.Memory.Modules {
		d.Memory.Modules = append(d.Memory.Modules, transport.DeviceMemoryModule{
			CapacityMB:   m.CapacityMB,
			SpeedMHz:     m.SpeedMHz,
			Manufacturer: m.Manufacturer,
			PartNumber:   m.PartNumber,
			Slot:         m.Slot,
		})
	}
	for _, disk := range info.Disks {
		d.Disks = append(d.Disks, transport.DeviceDisk{
			Model:         disk.Model,
			SizeGB:        disk.SizeGB,
			MediaType:     disk.MediaType,
			InterfaceType: disk.InterfaceType,
			Serial:        disk.Serial,
		})
	}
	for _, g := range info.GPUs {
		d.GPUs = append(d.GPUs, transport.DeviceGPU{Name: g.Name, DriverVer: g.DriverVer, VRAMMB: g.VRAMMB})
	}
	for _, n := range info.Network {
		d.Network = append(d.Network, transport.DeviceNIC{Name: n.Name, MAC: n.MAC, Type: n.Type})
	}
	for _, mon := range info.Monitors {
		d.Monitors = append(d.Monitors, transport.DeviceMonitor{Name: mon.Name, Width: mon.Width, Height: mon.Height})
	}
	if info.Battery != nil {
		d.Battery = &transport.DeviceBattery{
			Name:          info.Battery.Name,
			ChargePercent: info.Battery.ChargePercent,
			Status:        info.Battery.Status,
		}
	}
	if info.WindowsUpdate != nil {
		wu := info.WindowsUpdate
		d.WindowsUpdate = &transport.DeviceWU{
			Status:          wu.Status,
			PendingCount:    wu.PendingCount,
			SecurityPending: wu.SecurityPending,
			RebootPending:   wu.RebootPending,
			LastInstalledKB: wu.LastInstalledKB,
			LastInstalledAt: wu.LastInstalledAt,
			LastCheckedAt:   wu.LastCheckedAt,
			Source:          wu.Source,
		}
		for _, p := range wu.Pending {
			d.WindowsUpdate.Pending = append(d.WindowsUpdate.Pending, transport.DevicePendingKB{
				KB: p.KB, Title: p.Title, Security: p.Security, Severity: p.Severity,
			})
		}
	}
	return d
}
