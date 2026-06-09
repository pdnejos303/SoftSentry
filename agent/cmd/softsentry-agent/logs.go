// ไฟล์นี้กำหนด subcommand `logs` สำหรับแสดงเนื้อหาของ log file ของ agent
// log file จะถูกสร้างเฉพาะเมื่อ agent รันเป็น background service เท่านั้น
// รองรับ flag -n/--tail สำหรับจำกัดจำนวนบรรทัดที่แสดง
package main

import (
	"errors"        // ใช้ตรวจสอบว่า error เป็นประเภท ErrNotExist หรือไม่
	"os"            // ใช้อ่านไฟล์และตรวจสอบว่าไฟล์มีอยู่หรือไม่
	"path/filepath" // ใช้ต่อ path directory กับชื่อไฟล์ log
	"strings"       // ใช้แยก string เป็นบรรทัด, ตัด newline ท้าย, และต่อบรรทัดกลับ

	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/config" // ใช้หา config directory ที่เก็บ log file
)

// logsCmd สร้าง subcommand `logs` สำหรับแสดงเนื้อหา log file ของ agent service
// รองรับ flag -n/--tail เพื่อจำกัดจำนวนบรรทัดที่แสดง (default: 50 บรรทัดสุดท้าย)
func logsCmd() *cobra.Command {
	var tail int // จำนวนบรรทัดสุดท้ายที่จะแสดง (0 หมายถึงแสดงทั้งหมด)
	c := &cobra.Command{
		Use:   "logs",                                                            // ชื่อ subcommand
		Short: "Print the agent service log (written when running as a service)", // คำอธิบายสั้น
		RunE: func(cmd *cobra.Command, _ []string) error {
			// หา config directory ที่เก็บไฟล์ agent.log
			dir, err := config.Dir()
			if err != nil {
				// หา config directory ไม่ได้ — ไม่สามารถอ่าน log ได้
				return err
			}
			// ต่อ path ของ config directory กับชื่อไฟล์ log "agent.log"
			path := filepath.Join(dir, "agent.log")
			// (TH) path มาจากการคำนวณภายใน ไม่ได้รับมาจากผู้ใช้ จึงปลอดภัย
			data, err := os.ReadFile(path) // #nosec G304 — path derived internally
			if err != nil {
				// ตรวจสอบว่าไฟล์ log ยังไม่มีอยู่ (agent ยังไม่เคยรันเป็น service)
				if errors.Is(err, os.ErrNotExist) {
					// แสดงข้อความอธิบายว่า log จะมีก็ต่อเมื่อรันเป็น service เท่านั้น
					cmd.Println("(no log yet — the log is written only when running as a service)")
					return nil // ไม่ใช่ error — เป็นสถานการณ์ปกติสำหรับ fresh install
				}
				// error อื่นๆ เช่น permission denied
				return err
			}
			// แสดงเฉพาะ N บรรทัดสุดท้ายตามที่ --tail กำหนด (หรือทั้งหมดถ้า tail=0)
			cmd.Print(tailLines(string(data), tail))
			return nil
		},
	}
	// ลงทะเบียน flag -n/--tail สำหรับจำกัดจำนวนบรรทัดที่แสดง
	// default 50 บรรทัด, ค่า 0 หมายถึงแสดงทั้งหมด
	c.Flags().IntVarP(&tail, "tail", "n", 50, "show only the last N lines (0 = all)")
	return c
}

// tailLines returns the last n non-empty-trimmed lines of s (n<=0 returns all).
// (TH) คืนค่า n บรรทัดสุดท้ายของ string s
// ถ้า n<=0 จะคืนค่าทั้งหมด
// ถ้า n มากกว่าจำนวนบรรทัดทั้งหมด ก็คืนค่าทั้งหมดเช่นกัน
//
// Parameters:
//   - s: เนื้อหา log ทั้งหมดเป็น string
//   - n: จำนวนบรรทัดสุดท้ายที่ต้องการ (0 หรือน้อยกว่า = ทั้งหมด)
func tailLines(s string, n int) string {
	// ถ้า n<=0 แสดงทั้งหมดโดยไม่ตัด
	if n <= 0 {
		return s
	}
	// แยก string ออกเป็นบรรทัด โดยตัด newline ท้ายสุดออกก่อนเพื่อหลีกเลี่ยงบรรทัดว่างสุดท้าย
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	// ถ้าจำนวนบรรทัดทั้งหมดไม่เกิน n ให้คืนค่าทั้งหมดพร้อม newline ท้าย
	if len(lines) <= n {
		return strings.Join(lines, "\n") + "\n"
	}
	// คืนค่าเฉพาะ n บรรทัดสุดท้าย พร้อม newline ท้าย
	return strings.Join(lines[len(lines)-n:], "\n") + "\n"
}
