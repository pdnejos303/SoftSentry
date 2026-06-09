// Package service จัดการการติดตั้ง, ถอนการติดตั้ง, และรัน agent ในฐานะ OS service:
// เป็น Windows SCM service บน Windows และ launchd daemon บน macOS
// entry point สำหรับ install/uninstall/status เป็น platform-specific (ดู service_windows.go / service_darwin.go)
// ส่วนที่ใช้ร่วมกันข้าม platform (ชื่อต่างๆ และ launchd plist template) อยู่ในไฟล์นี้
// เพื่อให้ unit test บน platform ใดก็ได้
package service

import (
	"fmt"     // ใช้สำหรับ format string ในการสร้าง error message และ XML content
	"strings" // ใช้สำหรับ strings.Builder ในการสร้าง plist XML และ strings.NewReplacer สำหรับ escape
)

// ค่าคงที่สำหรับชื่อ service บน Windows และ macOS
const (
	// Name คือชื่อ service ที่ลงทะเบียนใน Windows SCM (Service Control Manager)
	Name = "SoftSentryAgent"
	// DisplayName คือชื่อที่แสดงใน Windows Services console เพื่อให้ผู้ใช้เข้าใจง่าย
	DisplayName = "SoftSentry Agent"
	// Description คือข้อความอธิบาย service ที่แสดงในรายละเอียด service บน Windows
	Description = "Scans installed software + digital signatures and reports to the SoftSentry server."
	// LaunchdLabel คือ job label ของ macOS launchd และใช้เป็นชื่อไฟล์ plist (ไม่มีนามสกุล)
	LaunchdLabel = "com.softsentry.agent"
)

// Config อธิบายวิธีที่ service ที่ติดตั้งแล้วจะ invoke binary ของ agent
type Config struct {
	// ExePath คือ absolute path ไปยัง agent binary ที่ service จะรัน
	ExePath string
	// Args คือ argument ที่ส่งให้ binary ทุกครั้งที่ start (เช่น ["run"])
	Args []string
}

// ErrUnsupported คือ error ที่ส่งกลับจาก install/uninstall/status
// บน platform อื่นนอกจาก Windows และ macOS
// (agent รองรับเฉพาะ Win/Mac ตาม spec "Out of scope")
var ErrUnsupported = fmt.Errorf("service management is only supported on Windows and macOS")

// launchdPlist สร้างเนื้อหา macOS LaunchDaemon plist สำหรับ binary + args ที่กำหนด
// daemon จะรันทันทีเมื่อ load และ restart อัตโนมัติเมื่อ crash (spec 1.8)
// LaunchDaemons รันเป็น root จึงไม่ต้องระบุ UserName key
// logPath รับทั้ง stdout และ stderr ของ process
//
// Parameter:
//   - label: launchd job label เช่น "com.softsentry.agent"
//   - exePath: absolute path ของ binary ที่จะรัน
//   - args: argument list ที่ส่งให้ binary (ถ้ามี)
//   - logPath: path ของ log file ที่จะ redirect stdout/stderr ไป
//
// Return: string ที่เป็น plist XML content พร้อมใช้เขียนลงไฟล์
func launchdPlist(label, exePath string, args []string, logPath string) string {
	// สร้าง string builder สำหรับ build XML content ทีละส่วน
	var b strings.Builder
	// เขียน XML declaration บรรทัดแรก
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	// เขียน DOCTYPE declaration ระบุ DTD ของ Apple plist
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" ` +
		`"http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	// เปิด tag plist พร้อมระบุ version
	b.WriteString(`<plist version="1.0">` + "\n")
	// เปิด dict element หลักของ plist
	b.WriteString("<dict>\n")
	// เขียน key "Label" สำหรับระบุ job label ของ daemon
	b.WriteString("\t<key>Label</key>\n")
	// เขียนค่า label พร้อม XML escape เพื่อป้องกัน character พิเศษ
	fmt.Fprintf(&b, "\t<string>%s</string>\n", xmlEscape(label))
	// เขียน key "ProgramArguments" สำหรับกำหนด binary และ argument ที่จะรัน
	b.WriteString("\t<key>ProgramArguments</key>\n")
	// เปิด array สำหรับ argument list
	b.WriteString("\t<array>\n")
	// เขียน path ของ binary เป็น element แรกของ array (argv[0])
	fmt.Fprintf(&b, "\t\t<string>%s</string>\n", xmlEscape(exePath))
	// วนลูปเพิ่ม argument แต่ละตัวเป็น element ใน array
	for _, a := range args {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", xmlEscape(a))
	}
	// ปิด array ของ ProgramArguments
	b.WriteString("\t</array>\n")
	// เพิ่ม RunAtLoad=true เพื่อให้ daemon รันทันทีเมื่อ launchd load plist
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	// เพิ่ม KeepAlive=true เพื่อให้ launchd restart daemon อัตโนมัติเมื่อ crash
	b.WriteString("\t<key>KeepAlive</key>\n\t<true/>\n")
	// redirect stdout ไปยัง log file ที่กำหนด
	fmt.Fprintf(&b, "\t<key>StandardOutPath</key>\n\t<string>%s</string>\n", xmlEscape(logPath))
	// redirect stderr ไปยัง log file เดียวกันกับ stdout
	fmt.Fprintf(&b, "\t<key>StandardErrorPath</key>\n\t<string>%s</string>\n", xmlEscape(logPath))
	// ปิด dict element หลัก
	b.WriteString("</dict>\n")
	// ปิด plist element
	b.WriteString("</plist>\n")
	// คืนค่า string XML ทั้งหมดที่ build ได้
	return b.String()
}

// xmlEscape escape character ที่ไม่ปลอดภัยใน XML text
// เพื่อป้องกันไม่ให้ path หรือ arg ที่มี &, <, > ทำให้ plist เสียหาย
//
// Parameter:
//   - s: string ที่ต้องการ escape
//
// Return: string ที่ escaped แล้ว พร้อมใช้ใน XML attribute หรือ text node
func xmlEscape(s string) string {
	// ใช้ strings.NewReplacer แทนที่ character พิเศษ XML ด้วย entity reference
	return strings.NewReplacer(
		"&", "&amp;", // แทนที่ & ด้วย &amp; (ต้อง escape ก่อนเสมอ)
		"<", "&lt;",  // แทนที่ < ด้วย &lt; เพื่อไม่ให้ถูกตีความเป็น tag
		">", "&gt;",  // แทนที่ > ด้วย &gt; เพื่อให้ XML ถูกต้อง
	).Replace(s)
}
