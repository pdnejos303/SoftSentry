package installer

import (
	"strings"
	"testing"
)

func TestUninstallTextAllLangs(t *testing.T) {
	for _, l := range []Lang{LangTH, LangEN, LangJA} {
		h, b, yes, no := UninstallConfirmText(l)
		if h == "" || b == "" || yes == "" || no == "" {
			t.Fatalf("%s: empty confirm text", l)
		}
		sh, sm := UninstallResultText(l)
		if sh == "" || sm == "" {
			t.Fatalf("%s: empty result text", l)
		}
		// body ควรเอ่ยถึง SoftSentry เพื่อให้ผู้ใช้รู้ว่ากำลังถอนอะไร
		if !strings.Contains(b, "SoftSentry") {
			t.Fatalf("%s: confirm body should name SoftSentry", l)
		}
	}
}
