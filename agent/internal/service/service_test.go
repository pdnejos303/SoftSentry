// Package service รวม unit test สำหรับ cross-platform helpers ของ service package
// โดยเฉพาะการสร้าง plist XML สำหรับ macOS LaunchDaemon
package service

import (
	"strings" // ใช้ strings.Contains สำหรับตรวจสอบว่า plist output มี string ที่คาดหวังหรือไม่
	"testing" // ใช้ testing.T สำหรับเขียน unit test
)

// TestLaunchdPlist ทดสอบว่าฟังก์ชัน launchdPlist สร้าง plist XML ที่ถูกต้องครบถ้วน
// โดยตรวจสอบว่ามี element สำคัญทุกส่วนอยู่ใน output
func TestLaunchdPlist(t *testing.T) {
	// เรียก launchdPlist ด้วยค่า argument จริงที่ใช้ในการติดตั้ง
	out := launchdPlist(
		LaunchdLabel,                              // label = "com.softsentry.agent"
		"/usr/local/bin/softsentry-agent",         // path ของ binary
		[]string{"run"},                           // argument ที่ส่งให้ binary
		"/Library/Logs/SoftSentry/agent.log",      // path ของ log file
	)

	// กำหนด map ของการตรวจสอบ: key คือคำอธิบาย, value คือ string ที่ต้องพบใน output
	checks := map[string]string{
		"declares the job label":      "<string>com.softsentry.agent</string>",                                                  // ต้องมี label ของ job
		"runs the binary as argv[0]":  "<string>/usr/local/bin/softsentry-agent</string>",                                      // ต้องมี path binary เป็น argv[0]
		"passes the run subcommand":   "<string>run</string>",                                                                   // ต้องมี argument "run"
		"starts the daemon at load":   "<key>RunAtLoad</key>\n\t<true/>",                                                        // ต้องมี RunAtLoad=true
		"keeps the daemon alive":      "<key>KeepAlive</key>\n\t<true/>",                                                        // ต้องมี KeepAlive=true
		"redirects stdout to the log": "<key>StandardOutPath</key>\n\t<string>/Library/Logs/SoftSentry/agent.log</string>",     // ต้อง redirect stdout ไป log
		"redirects stderr to the log": "<key>StandardErrorPath</key>\n\t<string>/Library/Logs/SoftSentry/agent.log</string>",   // ต้อง redirect stderr ไป log
		"is a valid plist document":   "<plist version=\"1.0\">",                                                                // ต้องมี plist declaration ที่ถูกต้อง
	}
	// วนลูปตรวจสอบทุก requirement
	for why, needle := range checks {
		// ตรวจสอบว่า output มี needle ที่คาดหวังหรือไม่
		if !strings.Contains(out, needle) {
			// ถ้าไม่พบ needle ให้รายงาน error พร้อมแสดง output ทั้งหมดเพื่อช่วย debug
			t.Errorf("%s: expected output to contain %q\n--- got ---\n%s", why, needle, out)
		}
	}
}

// TestLaunchdPlistEscapesXML ทดสอบว่าฟังก์ชัน launchdPlist escape XML character พิเศษได้ถูกต้อง
// เพื่อป้องกัน malformed plist ที่จะทำให้ launchd parse ไม่ได้
func TestLaunchdPlistEscapesXML(t *testing.T) {
	// A path containing & must be escaped or the plist is malformed.
	// ทดสอบด้วย path ที่มี & ซึ่งเป็น character พิเศษใน XML
	out := launchdPlist("x", "/Apps/A & B/agent", nil, "/tmp/log")
	// ตรวจสอบว่า & ที่ไม่ได้ escape ไม่ปรากฏใน output (ต้องถูก escape เป็น &amp;)
	if strings.Contains(out, "A & B") {
		t.Errorf("raw ampersand leaked into plist (must be &amp;):\n%s", out)
	}
	// ตรวจสอบว่า & ถูก escape เป็น &amp; ใน output
	if !strings.Contains(out, "A &amp; B") {
		t.Errorf("expected escaped ampersand A &amp; B in plist:\n%s", out)
	}
}
