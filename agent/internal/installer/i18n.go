// ไฟล์นี้เก็บ "ข้อความทั้งหมด" ของ self-installer ใน 3 ภาษา (ไทย/อังกฤษ/ญี่ปุ่น)
// ไว้ที่เดียว เพื่อให้ทั้ง wizard GUI (Windows) และเนื้อหา consent ดึงจากแหล่งเดียวกัน
// — เพิ่ม/แก้ภาษาได้โดยไม่ต้องไล่แก้หลายไฟล์
//
// (EN) Single source of truth for every user-facing installer string in Thai,
// English and Japanese — consumed by the Windows wizard GUI and the consent
// content builder so a translation lives in exactly one place.
package installer

// WizardResult คือผลลัพธ์ที่ผู้ใช้กรอกใน wizard (เก็บก่อนขอสิทธิ์ admin) แล้วส่งต่อ
// ให้ instance ที่ elevated ผ่าน command-line flags — ประกาศที่นี่ (ไม่มี build tag)
// เพราะ selfinstall.go ใช้ทุกแพลตฟอร์ม
type WizardResult struct {
	Proceed    bool   // true เมื่อผู้ใช้กด "ติดตั้ง" จนจบ wizard (ไม่ใช่กดยกเลิก/ปิด)
	Lang       Lang   // ภาษาที่เลือก
	InstallDir string // โฟลเดอร์ปลายทางที่เลือก
}

// Lang คือรหัสภาษาที่ installer รองรับ (ตรงกับ locale ของ dashboard: th/en/ja)
type Lang string

const (
	LangTH Lang = "th" // ไทย — ภาษาเริ่มต้น (กลุ่มผู้ใช้หลัก)
	LangEN Lang = "en" // English
	LangJA Lang = "ja" // 日本語
)

// wizardLangs กำหนด "ลำดับ" ภาษาในกล่องเลือกภาษาของ wizard (index ต้องตรงกับ
// langLabels) — ไทยมาก่อนเพราะเป็น default
var wizardLangs = []Lang{LangTH, LangEN, LangJA}

// langLabels คือชื่อภาษาที่แสดงในกล่องเลือก เขียนด้วยอักษรของภาษานั้นๆ เอง
// (ผู้ใช้อ่านออกแม้ยังไม่รู้ว่าตอนนี้ UI เป็นภาษาอะไร)
var langLabels = []string{"ไทย", "English", "日本語"}

// ParseLang แปลงสตริง (เช่นจาก --lang flag) เป็น Lang โดย fallback เป็นไทยเมื่อ
// ค่าไม่ถูกต้อง — ใช้กับ argument ที่ส่งข้าม UAC จาก instance ที่ยังไม่ elevated
func ParseLang(s string) Lang {
	switch Lang(s) {
	case LangEN:
		return LangEN
	case LangJA:
		return LangJA
	case LangTH:
		return LangTH
	default:
		return LangTH
	}
}

// langIndex คืน index ของ lang ใน wizardLangs (ใช้ตั้งค่าเริ่มต้นให้ combobox)
// ถ้าไม่พบคืน 0 (ไทย)
func langIndex(l Lang) int {
	for i, w := range wizardLangs {
		if w == l {
			return i
		}
	}
	return 0
}

// wizText คือชุดข้อความทั้งหมดของหน้าจอ wizard สำหรับ "หนึ่งภาษา"
// ทุก field คือข้อความที่ผู้ใช้เห็นโดยตรง — แยกออกจาก logic เพื่อแปลได้สะดวก
type wizText struct {
	setupSuffix string // ต่อท้ายชื่อแอปบน title bar เช่น "ตัวติดตั้ง"

	// ปุ่มนำทางท้ายหน้าต่าง
	back     string
	next     string
	install  string
	cancel   string
	closeBtn string

	languageLabel string // ป้ายกำกับกล่องเลือกภาษา
	stepIndicator string // รูปแบบป้ายบอกขั้น เช่น "ขั้นที่ %d จาก %d" (2 อาร์กิวเมนต์: ขั้นปัจจุบัน, ทั้งหมด)

	// หน้า 1 — ยินดีต้อนรับ
	welcomeHeading string
	welcomeIntro   string

	// หน้า 2 — สิ่งที่จะติดตั้ง + ยินยอม
	consentHeading  string
	labelInstallAt  string
	labelRunsAs     string
	labelPermission string
	labelReportsTo  string
	labelCollected  string
	labelNotKept    string
	agree           string // ข้อความ checkbox ยินยอม

	// หน้า 3 — เลือกตำแหน่งติดตั้ง
	locationHeading string
	locationLabel   string
	browse          string
	browseTitle     string // title ของ native folder picker

	// หน้า 4 — กำลังติดตั้ง
	progressHeading string
	progressSteps   [4]string // ป้ายของแต่ละขั้น 1..4 (ตรงกับ performInstall)

	// หน้า 5 — ผลลัพธ์
	successHeading string
	successMsg     string
	errorHeading   string
	errorHint      string // คำแนะนำต่อท้าย error message
}

// wizTexts คือตารางข้อความครบทั้ง 3 ภาษา — เพิ่มภาษาใหม่ = เพิ่ม entry ที่นี่
// (และเพิ่มใน wizardLangs/langLabels ให้ index ตรงกัน)
var wizTexts = map[Lang]wizText{
	LangTH: {
		setupSuffix:     "ตัวติดตั้ง",
		back:            "ย้อนกลับ",
		next:            "ถัดไป",
		install:         "ติดตั้ง",
		cancel:          "ยกเลิก",
		closeBtn:        "ปิด",
		languageLabel:   "ภาษา",
		stepIndicator:   "ขั้นที่ %d จาก %d",
		welcomeHeading:  "ยินดีต้อนรับสู่ SoftSentry Agent",
		welcomeIntro:    "ตัวช่วยนี้จะติดตั้ง SoftSentry Agent บนเครื่องนี้\nกด \"ถัดไป\" เพื่อดูรายละเอียดก่อนเริ่มติดตั้ง",
		consentHeading:  "สิ่งที่จะเกิดขึ้นบนเครื่องนี้",
		labelInstallAt:  "ติดตั้งที่",
		labelRunsAs:     "ทำงานเป็น",
		labelPermission: "สิทธิ์ที่ใช้",
		labelReportsTo:  "รายงานไป",
		labelCollected:  "ส่งให้เซิร์ฟเวอร์เป็นระยะ:",
		labelNotKept:    "ไม่เก็บ:",
		agree:           "ฉันเข้าใจและยินยอมให้ติดตั้ง SoftSentry Agent",
		locationHeading: "เลือกตำแหน่งติดตั้ง",
		locationLabel:   "โฟลเดอร์ปลายทาง:",
		browse:          "เรียกดู…",
		browseTitle:     "เลือกโฟลเดอร์สำหรับติดตั้ง SoftSentry Agent",
		progressHeading: "กำลังติดตั้ง SoftSentry Agent…",
		progressSteps: [4]string{
			"กำลังลบเวอร์ชันเก่า",
			"กำลังคัดลอกไฟล์",
			"กำลังลงทะเบียนเครื่อง",
			"กำลังเริ่มบริการเบื้องหลัง",
		},
		successHeading: "ติดตั้งสำเร็จ",
		successMsg:     "SoftSentry Agent ติดตั้งและกำลังทำงานแล้ว\nเครื่องนี้จะปรากฏใน dashboard ภายใน 1 นาที",
		errorHeading:   "การติดตั้งล้มเหลว",
		errorHint:      "ลองดาวน์โหลดและเรียกใช้ตัวติดตั้งอีกครั้ง หากยังไม่สำเร็จ โปรดติดต่อทีม IT",
	},
	LangEN: {
		setupSuffix:     "Setup",
		back:            "Back",
		next:            "Next",
		install:         "Install",
		cancel:          "Cancel",
		closeBtn:        "Close",
		languageLabel:   "Language",
		stepIndicator:   "Step %d of %d",
		welcomeHeading:  "Welcome to SoftSentry Agent",
		welcomeIntro:    "This wizard will install SoftSentry Agent on this computer.\nClick \"Next\" to review the details before installing.",
		consentHeading:  "What will happen on this computer",
		labelInstallAt:  "Install to",
		labelRunsAs:     "Runs as",
		labelPermission: "Permission",
		labelReportsTo:  "Reports to",
		labelCollected:  "Sent to the server periodically:",
		labelNotKept:    "Not collected:",
		agree:           "I understand and agree to install SoftSentry Agent",
		locationHeading: "Choose install location",
		locationLabel:   "Destination folder:",
		browse:          "Browse…",
		browseTitle:     "Choose a folder to install SoftSentry Agent",
		progressHeading: "Installing SoftSentry Agent…",
		progressSteps: [4]string{
			"Removing any previous version",
			"Copying files",
			"Enrolling this machine",
			"Starting the background service",
		},
		successHeading: "Installation complete",
		successMsg:     "SoftSentry Agent is installed and running.\nThis machine will appear in the dashboard within 1 minute.",
		errorHeading:   "Installation failed",
		errorHint:      "Please download and run the installer again. If it still fails, contact your IT team.",
	},
	LangJA: {
		setupSuffix:     "セットアップ",
		back:            "戻る",
		next:            "次へ",
		install:         "インストール",
		cancel:          "キャンセル",
		closeBtn:        "閉じる",
		languageLabel:   "言語",
		stepIndicator:   "ステップ %d/%d",
		welcomeHeading:  "SoftSentry Agent へようこそ",
		welcomeIntro:    "このウィザードは SoftSentry Agent をこのコンピューターにインストールします。\n「次へ」をクリックして、インストール前に詳細を確認してください。",
		consentHeading:  "このコンピューターで行われる処理",
		labelInstallAt:  "インストール先",
		labelRunsAs:     "実行形態",
		labelPermission: "必要な権限",
		labelReportsTo:  "送信先サーバー",
		labelCollected:  "サーバーへ定期的に送信:",
		labelNotKept:    "収集しないもの:",
		agree:           "内容を理解し、SoftSentry Agent のインストールに同意します",
		locationHeading: "インストール先の選択",
		locationLabel:   "インストール先フォルダー:",
		browse:          "参照…",
		browseTitle:     "SoftSentry Agent のインストール先フォルダーを選択",
		progressHeading: "SoftSentry Agent をインストールしています…",
		progressSteps: [4]string{
			"以前のバージョンを削除しています",
			"ファイルをコピーしています",
			"このマシンを登録しています",
			"バックグラウンドサービスを開始しています",
		},
		successHeading: "インストールが完了しました",
		successMsg:     "SoftSentry Agent がインストールされ、実行中です。\nこのマシンは1分以内にダッシュボードに表示されます。",
		errorHeading:   "インストールに失敗しました",
		errorHint:      "インストーラーを再度ダウンロードして実行してください。それでも失敗する場合は、IT 担当者にお問い合わせください。",
	},
}

// textFor คืนชุดข้อความของภาษาที่ขอ (fallback เป็นไทยถ้าไม่พบ — ไม่ควรเกิด)
func textFor(l Lang) wizText {
	if t, ok := wizTexts[l]; ok {
		return t
	}
	return wizTexts[LangTH]
}

// SuccessText / ErrorText เปิดข้อความผลลัพธ์ที่แปลแล้วให้แพ็กเกจ main (cmd) ใช้แสดง
// หน้าสำเร็จ/ล้มเหลว โดยไม่ต้องเข้าถึง wizText ที่เป็น unexported
func SuccessText(lang Lang) (heading, msg string) {
	t := textFor(lang)
	return t.successHeading, t.successMsg
}

// ErrorText คืนหัวข้อ + ข้อความ error (ต่อท้ายด้วยคำแนะนำของภาษานั้น)
func ErrorText(lang Lang, errMsg string) (heading, msg string) {
	t := textFor(lang)
	return t.errorHeading, errMsg + "\n\n" + t.errorHint
}

// consentContent คือเนื้อหา consent ที่ "ขึ้นกับภาษา" (ส่วนที่เป็นประโยค ไม่ใช่
// ค่าจากระบบอย่าง path/URL) แยกจาก wizText เพราะ BuildConsentInfo เป็นแหล่งข้อมูล
// กลางที่ทั้ง console และ GUI ใช้
type consentContent struct {
	purpose       string
	runsAs        string
	permission    string
	dataCollected []string
	dataNotKept   []string
	uninstallHint string
}

// consentContents เก็บเนื้อหา consent ทั้ง 3 ภาษา
var consentContents = map[Lang]consentContent{
	LangTH: {
		purpose: "SoftSentry ช่วยให้ทีม IT / Security ขององค์กรเก็บรายการซอฟต์แวร์" +
			"ในเครื่องคอมพิวเตอร์ และแจ้งเตือนช่องโหว่ความปลอดภัยที่รู้จัก",
		runsAs:     "background service ที่เริ่มทำงานอัตโนมัติเมื่อเปิดเครื่อง",
		permission: "Administrator (เพื่อลงทะเบียน service) — Windows จะถามสิทธิ์ต่อจากนี้",
		dataCollected: []string{
			"รายการซอฟต์แวร์ที่ติดตั้ง (ชื่อ, เวอร์ชัน, ผู้พัฒนา)",
			"สถานะลายเซ็นดิจิทัลของไฟล์โปรแกรม",
			"ข้อมูลเครื่องพื้นฐาน (ชื่อเครื่อง, เวอร์ชัน OS, สถาปัตยกรรม)",
		},
		dataNotKept: []string{
			"ไฟล์ส่วนตัว, เอกสาร, รูปภาพ, อีเมล",
			"การพิมพ์คีย์บอร์ด, ภาพหน้าจอ, หรือประวัติการเข้าเว็บ",
		},
		uninstallHint: "ถอนการติดตั้งได้ทุกเมื่อจาก \"Apps & features\" หรือสั่ง: softsentry-agent uninstall",
	},
	LangEN: {
		purpose: "SoftSentry helps your organization's IT / Security team keep an " +
			"inventory of installed software and alerts on known security vulnerabilities.",
		runsAs:     "a background service that starts automatically at boot",
		permission: "Administrator (to register the service) — Windows will ask next",
		dataCollected: []string{
			"Installed software (name, version, publisher)",
			"Digital-signature status of program files",
			"Basic machine info (hostname, OS version, architecture)",
		},
		dataNotKept: []string{
			"Personal files, documents, photos, email",
			"Keystrokes, screenshots, or browsing history",
		},
		uninstallHint: "You can uninstall any time from \"Apps & features\" or run: softsentry-agent uninstall",
	},
	LangJA: {
		purpose: "SoftSentry は、組織の IT / セキュリティチームがインストール済み" +
			"ソフトウェアの一覧を管理し、既知のセキュリティ脆弱性を通知するのに役立ちます。",
		runsAs:     "起動時に自動的に開始するバックグラウンドサービス",
		permission: "管理者権限（サービス登録のため）— この後 Windows が確認します",
		dataCollected: []string{
			"インストール済みソフトウェア（名前・バージョン・発行元）",
			"プログラムファイルのデジタル署名の状態",
			"基本的なマシン情報（ホスト名・OS バージョン・アーキテクチャ）",
		},
		dataNotKept: []string{
			"個人ファイル・ドキュメント・写真・メール",
			"キー入力・スクリーンショット・閲覧履歴",
		},
		uninstallHint: "「アプリと機能」からいつでもアンインストールできます。または: softsentry-agent uninstall",
	},
}

// uninstallText เก็บข้อความหน้าจอถอนการติดตั้งของ "หนึ่งภาษา"
type uninstallText struct {
	confirmHeading string
	confirmBody    string
	yes            string
	no             string
	successHeading string
	successMsg     string
}

// uninstallTexts ครบทั้ง 3 ภาษา (เพิ่มภาษา = เพิ่ม entry ที่นี่)
var uninstallTexts = map[Lang]uninstallText{
	LangTH: {
		confirmHeading: "ถอนการติดตั้ง SoftSentry Agent?",
		confirmBody: "การถอนการติดตั้งจะหยุดบริการเบื้องหลัง และลบ SoftSentry Agent " +
			"ออกจากเครื่องนี้ทั้งหมด (โปรแกรม, การตั้งค่า และข้อมูลที่เก็บไว้)\n" +
			"เครื่องนี้จะหยุดรายงานไปยังเซิร์ฟเวอร์",
		yes:            "ถอนการติดตั้ง",
		no:             "ยกเลิก",
		successHeading: "ถอนการติดตั้งสำเร็จ",
		successMsg:     "SoftSentry Agent ถูกถอนออกจากเครื่องนี้แล้ว",
	},
	LangEN: {
		confirmHeading: "Uninstall SoftSentry Agent?",
		confirmBody: "Uninstalling stops the background service and removes SoftSentry " +
			"Agent from this computer entirely (program, settings, and stored data).\n" +
			"This machine will stop reporting to the server.",
		yes:            "Uninstall",
		no:             "Cancel",
		successHeading: "Uninstall complete",
		successMsg:     "SoftSentry Agent has been removed from this computer.",
	},
	LangJA: {
		confirmHeading: "SoftSentry Agent をアンインストールしますか？",
		confirmBody: "アンインストールするとバックグラウンドサービスが停止し、SoftSentry " +
			"Agent がこのコンピューターから完全に削除されます（プログラム・設定・保存データ）。\n" +
			"このマシンはサーバーへの報告を停止します。",
		yes:            "アンインストール",
		no:             "キャンセル",
		successHeading: "アンインストールが完了しました",
		successMsg:     "SoftSentry Agent はこのコンピューターから削除されました。",
	},
}

// uninstallTextFor คืนข้อความถอนการติดตั้งของภาษาที่ขอ (fallback ไทย)
func uninstallTextFor(l Lang) uninstallText {
	if t, ok := uninstallTexts[l]; ok {
		return t
	}
	return uninstallTexts[LangTH]
}

// UninstallConfirmText เปิดข้อความหน้ายืนยันถอนให้แพ็กเกจ cmd ใช้
func UninstallConfirmText(lang Lang) (heading, body, yes, no string) {
	t := uninstallTextFor(lang)
	return t.confirmHeading, t.confirmBody, t.yes, t.no
}

// UninstallResultText เปิดข้อความหน้าผลสำเร็จของการถอน
func UninstallResultText(lang Lang) (successHeading, successMsg string) {
	t := uninstallTextFor(lang)
	return t.successHeading, t.successMsg
}
