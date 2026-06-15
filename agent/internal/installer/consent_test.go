package installer

import (
	"strings"
	"testing"
)

// BuildConsentInfo ต้องเติมข้อเท็จจริงสำคัญครบทุกฟิลด์ และต้องสะท้อนค่าที่ส่งเข้าไป
// (server URL + install dir) เพราะ renderer ทั้ง console และ GUI พึ่งฟิลด์เหล่านี้
func TestBuildConsentInfoPopulatesKeyFacts(t *testing.T) {
	ci := BuildConsentInfo("http://192.168.1.88:47800", `C:\Program Files\SoftSentry`)

	if ci.ServerURL != "http://192.168.1.88:47800" {
		t.Errorf("ServerURL not carried through: %q", ci.ServerURL)
	}
	if ci.InstallDir != `C:\Program Files\SoftSentry` {
		t.Errorf("InstallDir not carried through: %q", ci.InstallDir)
	}
	if ci.AppName == "" || ci.Purpose == "" || ci.RunsAs == "" ||
		ci.Permission == "" || ci.UninstallHint == "" {
		t.Errorf("a required text field is empty: %+v", ci)
	}
	if len(ci.DataCollected) == 0 || len(ci.DataNotKept) == 0 {
		t.Errorf("data lists must not be empty: %+v", ci)
	}
}

// install dir ว่าง ต้องไม่ทำให้ disclosure ว่างเปล่า — ใช้คำอธิบายทั่วไปแทน
func TestBuildConsentInfoFallsBackOnEmptyDir(t *testing.T) {
	ci := BuildConsentInfo("http://x", "")
	if strings.TrimSpace(ci.InstallDir) == "" {
		t.Error("empty install dir should fall back to a generic description")
	}
}
