// ไฟล์นี้นิยาม "เนื้อหา consent" แบบ platform-neutral ที่ใช้ร่วมกันระหว่าง
// ตัว renderer สองแบบ: console (ข้อความล้วน บน macOS/Linux และเป็น fallback บน
// Windows) กับ GUI (Win32 TaskDialog บน Windows) — เพื่อให้ข้อความที่ผู้ใช้เห็น
// มาจากแหล่งเดียว ไม่ต้องดูแลซ้ำสองที่
//
// (EN) Platform-neutral consent content shared by the console renderer and the
// Windows TaskDialog GUI, so the disclosure facts live in exactly one place.
package installer

// ConsentInfo คือข้อเท็จจริงทั้งหมดที่ต้องบอกผู้ใช้ก่อนขอความยินยอมติดตั้ง
// — ติดตั้งที่ไหน, ทำงานแบบไหน, ต้องใช้สิทธิ์อะไร, รายงานไปที่ไหน, เก็บ/ไม่เก็บ
// ข้อมูลอะไร, และถอนการติดตั้งยังไง
//
// หมายเหตุความปลอดภัย: struct นี้ "จงใจ" ไม่มีฟิลด์ deployment token เพราะ token
// เป็นความลับและห้ามแสดงให้ผู้ใช้เห็น (ถ้าหลุดบนหน้าจอ ผู้ไม่หวังดีอาจเอาไป
// enroll เครื่องอื่น) — BuildConsentInfo ก็ไม่รับ token เป็น parameter ด้วยซ้ำ
type ConsentInfo struct {
	AppName       string   // ชื่อโปรแกรมที่แสดง เช่น "SoftSentry Agent"
	Purpose       string   // อธิบายสั้นๆ ว่าโปรแกรมนี้มีไว้ทำอะไร
	InstallDir    string   // โฟลเดอร์ปลายทางที่จะติดตั้ง
	RunsAs        string   // รูปแบบการทำงาน (เช่น background service เริ่มตอนบูต)
	Permission    string   // สิทธิ์ที่ต้องใช้ (Administrator)
	ServerURL     string   // server ที่ agent จะรายงานผลไป
	DataCollected []string // รายการข้อมูลที่ "เก็บ" และส่งให้ server
	DataNotKept   []string // รายการข้อมูลที่ "ไม่เก็บ" — สำคัญต่อความไว้วางใจ
	UninstallHint string   // วิธีถอนการติดตั้ง
}

// BuildConsentInfo ประกอบ ConsentInfo จาก server URL และ install directory ที่
// คำนวณไว้แล้ว — เนื้อหาคงที่ (purpose, รายการข้อมูล ฯลฯ) ถูกกำหนดไว้ที่เดียวตรงนี้
//
// ไม่รับ token โดยเจตนา (ดูหมายเหตุที่ ConsentInfo) ถ้า installDir ว่าง จะใช้
// คำอธิบายทั่วไปแทน เพื่อให้ disclosure ยังอ่านรู้เรื่องแม้หา path ไม่เจอ
func BuildConsentInfo(serverURL, installDir string) ConsentInfo {
	if installDir == "" {
		installDir = "โฟลเดอร์โปรแกรม SoftSentry"
	}
	return ConsentInfo{
		AppName: "SoftSentry Agent",
		Purpose: "SoftSentry ช่วยให้ทีม IT / Security ขององค์กรเก็บรายการซอฟต์แวร์" +
			"ในเครื่องคอมพิวเตอร์ และแจ้งเตือนช่องโหว่ความปลอดภัยที่รู้จัก",
		InstallDir: installDir,
		RunsAs:     "background service ที่เริ่มทำงานอัตโนมัติเมื่อเปิดเครื่อง",
		Permission: "Administrator (เพื่อลงทะเบียน service) — Windows จะถามสิทธิ์ต่อจากนี้",
		ServerURL:  serverURL,
		DataCollected: []string{
			"รายการซอฟต์แวร์ที่ติดตั้ง (ชื่อ, เวอร์ชัน, ผู้พัฒนา)",
			"สถานะลายเซ็นดิจิทัลของไฟล์โปรแกรม",
			"ข้อมูลเครื่องพื้นฐาน (ชื่อเครื่อง, เวอร์ชัน OS, สถาปัตยกรรม)",
		},
		DataNotKept: []string{
			"ไฟล์ส่วนตัว, เอกสาร, รูปภาพ, อีเมล",
			"การพิมพ์คีย์บอร์ด, ภาพหน้าจอ, หรือประวัติการเข้าเว็บ",
		},
		UninstallHint: "ถอนการติดตั้งได้ทุกเมื่อจาก \"Apps & features\" หรือสั่ง: softsentry-agent uninstall",
	}
}
