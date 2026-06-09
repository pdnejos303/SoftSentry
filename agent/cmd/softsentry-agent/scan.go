// ไฟล์นี้กำหนด subcommand `scan` สำหรับสแกน software inventory แบบออฟไลน์
// ผลลัพธ์จะถูกพิมพ์เป็น JSON ไปยัง stdout หรือเขียนลงไฟล์
// ไม่มีการส่งข้อมูลใดๆ ไปยัง server (ใช้สำหรับ debug และ audit เท่านั้น)
package main

import (
	"context"      // ใช้สร้าง context ที่มี timeout สำหรับการสแกน
	"encoding/json" // ใช้แปลงผลสแกนเป็น JSON แบบ pretty-print
	"fmt"          // ใช้สร้างข้อความ error และ format ข้อความแสดงผล
	"os"           // ใช้เขียนผลลัพธ์ลงไฟล์เมื่อระบุ --output
	"time"         // ใช้กำหนด timeout ให้การสแกน และ format timestamp

	"github.com/spf13/cobra" // framework สำหรับสร้าง CLI subcommand

	"github.com/softsentry/agent/internal/scanner" // ใช้รันการสแกน software inventory จริง
)

// scanCmd runs a one-shot local inventory scan and prints/writes the result.
// It does NOT send anything to the server (spec 1.9: `scan` is offline/debug).
//
// (TH) รันการสแกน inventory ในเครื่องแบบครั้งเดียวแล้วพิมพ์/เขียนผลลัพธ์
// จะ "ไม่" ส่งอะไรไปยังเซิร์ฟเวอร์ (ตาม spec 1.9: `scan` เป็นโหมดออฟไลน์/debug)
// เหมาะสำหรับ IT admin ที่ต้องการดูผลสแกนก่อน deploy จริง
func scanCmd() *cobra.Command {
	var output string // path ของไฟล์ที่จะเขียนผลลัพธ์ ("stdout"/"-" หมายถึงพิมพ์หน้าจอ)
	c := &cobra.Command{
		Use:   "scan",                                                       // ชื่อ subcommand
		Short: "Run a one-shot software inventory scan (local only, no upload)", // คำอธิบายสั้น
		Long: `Enumerate installed software and verify digital signatures, then print
the result as JSON. Nothing is sent to the server — use 'run' for that.

  Windows: HKLM/HKCU "Uninstall" registry keys + Authenticode (WinVerifyTrust)
  macOS:   /Applications + Info.plist + codesign --verify`,
		// (TH) ระบุที่มาของข้อมูลตาม OS:
		// Windows: อ่านจาก registry key "Uninstall" + ยืนยัน Authenticode signature
		// macOS: อ่านจากโฟลเดอร์ /Applications + Info.plist + ยืนยัน codesign
		RunE: func(cmd *cobra.Command, _ []string) error {
			// สร้าง context ที่มี timeout 2 นาที — การสแกนใช้เวลาสูงสุดเท่านี้
			// ถ้าเกิน 2 นาที context จะ cancel อัตโนมัติ
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel() // คืน resource ของ context เมื่อฟังก์ชันกลับ

			// แสดงข้อความบอกว่ากำลังสแกน — ส่งไป stderr เพื่อแยกจาก JSON output ใน stdout
			cmd.PrintErrln("Scanning installed software ...")
			// รันการสแกนจริง — enumerate software และยืนยัน signature
			res, err := scanner.Run(ctx)
			if err != nil {
				// การสแกนล้มเหลว (เช่น ไม่มีสิทธิ์อ่าน registry หรือ context ถูกยกเลิก)
				return fmt.Errorf("scan: %w", err)
			}

			// สร้าง payload สำหรับ JSON output โดยรวม metadata และรายการ software
			payload := map[string]any{
				"scanned_at":  res.FinishedAt.UTC().Format(time.RFC3339), // เวลาสแกนเสร็จ (UTC, RFC3339 format)
				"duration_ms": res.Duration().Milliseconds(),             // ระยะเวลาสแกน (มิลลิวินาที)
				"count":       len(res.Software),                         // จำนวน software ที่พบ
				"software":    res.Software,                              // รายการ software ทั้งหมด
			}
			// แปลง payload เป็น JSON แบบ pretty-print (indent 2 ช่อง)
			buf, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				// แปลงเป็น JSON ล้มเหลว — ไม่ควรเกิดขึ้นถ้า payload มีประเภทข้อมูลปกติ
				return fmt.Errorf("encode result: %w", err)
			}

			// ตรวจสอบว่าต้องพิมพ์ไป stdout หรือเขียนลงไฟล์
			if output == "" || output == "stdout" || output == "-" {
				// พิมพ์ JSON ไปยัง stdout ตามปกติ
				cmd.Println(string(buf))
			} else {
				// เขียน JSON ลงไฟล์ที่ระบุ ด้วย permission 0600 (เจ้าของอ่าน/เขียนได้เท่านั้น)
				if err := os.WriteFile(output, buf, 0o600); err != nil {
					// เขียนไฟล์ล้มเหลว (เช่น ไม่มีสิทธิ์ หรือ disk เต็ม)
					return fmt.Errorf("write %s: %w", output, err)
				}
				// แสดงผลสำเร็จไปยัง stderr พร้อมจำนวน software และ path ของไฟล์
				cmd.PrintErrf("✓ wrote %d software entries to %s\n", len(res.Software), output)
			}
			// แสดงสรุปผลการสแกนไปยัง stderr (จำนวน software และระยะเวลา)
			cmd.PrintErrf("✓ scanned %d entries in %dms\n",
				len(res.Software), res.Duration().Milliseconds())
			return nil
		},
	}
	// ลงทะเบียน flag --output สำหรับกำหนดปลายทางของ JSON output
	// "stdout"/"-"/"" หมายถึงพิมพ์ไป stdout, ค่าอื่นๆ คือ path ของไฟล์
	c.Flags().StringVar(&output, "output", "stdout", "where to write scan result (stdout|<file path>)")
	return c
}
