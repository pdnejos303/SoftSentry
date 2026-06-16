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

// emptyDirFallback คือข้อความแทน install dir เมื่อหา path ไม่เจอ — แยกตามภาษา
var emptyDirFallback = map[Lang]string{
	LangTH: "โฟลเดอร์โปรแกรม SoftSentry",
	LangEN: "the SoftSentry program folder",
	LangJA: "SoftSentry プログラムフォルダー",
}

// BuildConsentInfo ประกอบ ConsentInfo เป็นภาษาไทย (ค่าเริ่มต้น) — คงไว้เพื่อ
// console fallback และ test เดิม ภายในเรียก BuildConsentInfoLang
func BuildConsentInfo(serverURL, installDir string) ConsentInfo {
	return BuildConsentInfoLang(LangTH, serverURL, installDir)
}

// BuildConsentInfoLang ประกอบ ConsentInfo จาก server URL และ install directory
// ในภาษาที่กำหนด — เนื้อหาประโยค (purpose, รายการข้อมูล ฯลฯ) มาจาก consentContents
// ที่เดียว ส่วน path/URL ใส่ตามค่าจริง ไม่รับ token โดยเจตนา (ดูหมายเหตุที่
// ConsentInfo) ถ้า installDir ว่าง จะใช้คำอธิบายทั่วไปแทนตามภาษานั้น
func BuildConsentInfoLang(lang Lang, serverURL, installDir string) ConsentInfo {
	c := consentContents[lang]
	if c.purpose == "" { // ภาษาไม่รู้จัก → fallback ไทย
		lang, c = LangTH, consentContents[LangTH]
	}
	if installDir == "" {
		installDir = emptyDirFallback[lang]
	}
	return ConsentInfo{
		AppName:       "SoftSentry Agent",
		Purpose:       c.purpose,
		InstallDir:    installDir,
		RunsAs:        c.runsAs,
		Permission:    c.permission,
		ServerURL:     serverURL,
		DataCollected: c.dataCollected,
		DataNotKept:   c.dataNotKept,
		UninstallHint: c.uninstallHint,
	}
}
