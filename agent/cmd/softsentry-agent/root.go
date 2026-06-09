// Command hierarchy:
//
//	softsentry-agent
//	├── enroll     – one-time enrollment handshake (enrollment token → agent token)
//	├── install    – register OS service + optionally enroll in one step
//	├── uninstall  – deregister service, remove local state
//	├── run        – long-running loop (heartbeat + scan); entry point for the service
//	├── scan       – one-shot offline scan, prints JSON, no upload
//	├── status     – show service state + enrollment info
//	├── logs       – tail the service log file
//	└── version    – print version and exit
//
// (TH) ลำดับชั้นของคำสั่ง:
//
//	softsentry-agent
//	├── enroll     – ทำ handshake ลงทะเบียนครั้งแรก (enrollment token → agent token)
//	├── install    – ลงทะเบียน OS service และ enroll ในขั้นตอนเดียว (ถ้าต้องการ)
//	├── uninstall  – ถอนการลงทะเบียน service และลบ local state
//	├── run        – ลูปทำงานระยะยาว (heartbeat + scan); จุดเริ่มของ service
//	├── scan       – สแกนแบบครั้งเดียวออฟไลน์ พิมพ์ผลเป็น JSON ไม่อัปโหลด
//	├── status     – แสดงสถานะ service + ข้อมูลการ enroll
//	├── logs       – แสดงท้ายไฟล์ log ของ service
//	└── version    – พิมพ์เวอร์ชันแล้วจบการทำงาน
//
// ไฟล์นี้เป็น entry point สำหรับ Cobra CLI — กำหนด root command และลงทะเบียน
// subcommand ทั้งหมด รวมถึงจัดการตัวแปร agentVersion ที่แชร์ร่วมกันทั้งระบบ
package main

import (
	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI แบบมีโครงสร้างคำสั่ง

	"github.com/softsentry/agent/internal/transport" // ใช้อ่านค่า transport.Version ที่ถูก stamp ตอน build
)

// agentVersion is the single source of truth for this build's version, shared
// by the `version`/`--version` output, the enroll handshake, and the heartbeat.
// It lives in (and is stamped into) transport.Version via -ldflags at build
// time — see internal/transport/client.go. Keeping it to one var means the
// version reported at enroll can never drift from the one reported at heartbeat
// (a drift there is exactly what triggers a self-update restart loop).
//
// (TH) agentVersion คือแหล่งความจริงเดียวของเวอร์ชัน build นี้ ใช้ร่วมกันโดย
// คำสั่ง `version`/`--version`, การ handshake ตอน enroll และ heartbeat มันถูก
// เก็บไว้ใน (และถูก stamp ลงไปที่) transport.Version ผ่าน -ldflags ตอน build —
// ดู internal/transport/client.go การใช้ var ตัวเดียวทำให้เวอร์ชันที่รายงานตอน
// enroll ไม่มีทางเพี้ยนไปจากตอน heartbeat (ความเพี้ยนตรงนี้คือสาเหตุที่ทำให้เกิด
// self-update restart loop)
var agentVersion = transport.Version // ค่าจริงถูก stamp ผ่าน -ldflags ตอน build

// rootCmd สร้างและคืน root Cobra command พร้อม subcommand ทั้งหมดที่ลงทะเบียนไว้
// ถูกเรียกจาก main() เพื่อให้ Cobra แยกแยะ argument บรรทัดคำสั่ง
func rootCmd() *cobra.Command {
	// กำหนด root command หลัก — ชื่อ, คำอธิบายสั้น, คำอธิบายยาว, และตัวเลือกต่างๆ
	cmd := &cobra.Command{
		Use:   "softsentry-agent",                  // ชื่อคำสั่งที่แสดงใน help text
		Short: "SoftSentry endpoint agent",         // คำอธิบายสั้นสำหรับ help ระดับบน
		Long: `SoftSentry agent: scan installed software + signatures and report
to the backend.`, // คำอธิบายยาวที่แสดงเมื่อรัน `softsentry-agent --help`
		SilenceUsage: true,          // ไม่แสดง usage text เมื่อเกิด error (ทำให้ output สะอาดขึ้น)
		Version:      agentVersion,  // เปิดใช้งานคำสั่ง `softsentry-agent --version`
		// (TH) การตั้งค่า Version จะทำให้ Cobra จัดการ flag --version อัตโนมัติ
	}
	// ลงทะเบียน subcommand ทั้งหมดกับ root command
	cmd.AddCommand(enrollCmd())    // enroll — ลงทะเบียนเครื่องกับ server
	cmd.AddCommand(installCmd())   // install — ติดตั้งเป็น OS service
	cmd.AddCommand(uninstallCmd()) // uninstall — ถอดถอน service และลบ local state
	cmd.AddCommand(runCmd())       // run — ลูปหลักสำหรับ service (heartbeat + scan)
	cmd.AddCommand(scanCmd())      // scan — สแกนออฟไลน์ครั้งเดียว พิมพ์ JSON
	cmd.AddCommand(statusCmd())    // status — แสดงสถานะ service และการ enroll
	cmd.AddCommand(logsCmd())      // logs — แสดงบรรทัดสุดท้ายของ log file
	cmd.AddCommand(versionCmd())   // version — พิมพ์เวอร์ชันแล้วออก
	return cmd
}

// versionCmd สร้าง subcommand `version` ที่พิมพ์ชื่อโปรแกรมและเวอร์ชัน แล้วออก
// เป็นทางเลือกแบบ explicit แทนการใช้ `--version` flag
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",                              // ชื่อ subcommand ที่ใช้เรียก
		Short: "Print agent version and exit",         // คำอธิบายสั้นใน help text
		RunE: func(cmd *cobra.Command, _ []string) error {
			// พิมพ์ชื่อโปรแกรมตามด้วยเวอร์ชัน เช่น "softsentry-agent 0.1.0"
			cmd.Println("softsentry-agent", agentVersion)
			return nil // ไม่มี error — จบการทำงานปกติ
		},
	}
}
