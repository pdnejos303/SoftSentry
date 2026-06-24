//go:build windows

// ไฟล์นี้คือ "wizard GUI" หลายหน้าจอของ self-installer บน Windows ใช้ lxn/walk
// (pure-Go, ไม่มี cgo → cross-compile เป็น single .exe ได้) แทน TaskDialog หน้าจอ
// เดียวเดิม ให้ผู้ใช้: เลือกภาษา (ไทย/EN/JA) → อ่าน+ยินยอม → เลือกโฟลเดอร์ติดตั้ง →
// กด "ติดตั้ง" จากนั้น instance ที่ elevated จะแสดงหน้าความคืบหน้า (RunInstallProgress)
//
// โครงสร้าง: หน้าต่างเดียว แบ่งเป็น 3 แถบแนวตั้ง — header (กล่องเลือกภาษามุมขวาบน),
// content (พื้นที่เนื้อหาที่ "สร้างใหม่" ทุกครั้งที่เปลี่ยน step ตามภาษาปัจจุบัน),
// footer (ปุ่ม ย้อนกลับ/ยกเลิก/ถัดไป) เนื่องจาก walk ไม่จัด layout ให้ widget ที่
// invisible (shouldLayoutItem) เราจึง dispose+rebuild เฉพาะ panel ของ step ปัจจุบัน
package installer

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"github.com/softsentry/agent/internal/transport"
)

// lockFixedSize ทำให้หน้าต่างเป็น "ขนาดคงที่" — ปิดปุ่มขยายเต็มจอ (WS_MAXIMIZEBOX)
// และกรอบลากปรับขนาด (WS_THICKFRAME) ออกจาก window style เป็นมาตรฐานของตัวติดตั้ง
// (Inno/NSIS/MSI) และสำคัญกับ wizard นี้เพราะเราใช้วิธี dispose+rebuild panel ของ
// แต่ละ step — การ resize/maximize จะ relayout parent ทำให้ panel เก่าที่เพิ่ง
// dispose เกิดเป็น "เงาซ้อน" (ดูเหมือนมีสองหน้าต่าง/UI เพี้ยน) ตรึงขนาดไว้จึงตัด
// ปัญหานี้ทั้งหมด เรียกหลัง Create() เท่านั้น (ต้องมี HWND แล้ว)
func lockFixedSize(hwnd win.HWND) {
	style := win.GetWindowLong(hwnd, win.GWL_STYLE)
	style &^= win.WS_MAXIMIZEBOX | win.WS_THICKFRAME
	win.SetWindowLong(hwnd, win.GWL_STYLE, style)
	// บังคับวาดกรอบใหม่ทันทีหลังเปลี่ยน style (ไม่ย้าย/ปรับขนาด)
	win.SetWindowPos(hwnd, 0, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_FRAMECHANGED)
}

// redrawFull บังคับ erase + repaint ของ widget และลูกทั้งหมด "ทันที" (synchronous)
// walk.Invalidate() เรียกแค่ InvalidateRect(hwnd, nil, true) — ตั้งเวลาวาดทีหลังและ
// ไม่รวม child window จึงลบ "เงาซ้อน" ของ panel เก่าที่เพิ่ง dispose ไม่หมด (เห็นเป็น
// ตัวอักษรซ้อนกันเวลาสลับภาษา/เปลี่ยน step). RDW_ERASE ลบพื้นหลังเดิม, RDW_ALLCHILDREN
// วาด child ใหม่ด้วย, RDW_UPDATENOW สั่งวาดทันทีไม่รอ message loop.
func redrawFull(hwnd win.HWND) {
	win.RedrawWindow(hwnd, nil, 0,
		win.RDW_ERASE|win.RDW_INVALIDATE|win.RDW_ALLCHILDREN|win.RDW_UPDATENOW)
}

// forceEraseBg ลบพิกเซลของ client area ทิ้งด้วย GDI ตรงๆ (ไม่พึ่ง WM_ERASEBKGND ของ
// walk ที่คืน NullBrush แล้วไม่ยอมลบ = ต้นเหตุเงาซ้อนที่แก้ด้วย Background ไม่หาย) —
// เรียกตอน content "ว่าง" (หลัง dispose panel เก่า ก่อนสร้างใหม่) เพื่อทาทับด้วยสีพื้น
// dialog มาตรฐาน ลบตัวอักษร panel เก่าหายเกลี้ยงแน่นอน ไม่ว่า walk จะ erase ให้หรือไม่
func forceEraseBg(hwnd win.HWND) {
	hdc := win.GetDC(hwnd)
	if hdc == 0 {
		return
	}
	defer win.ReleaseDC(hwnd, hdc)
	var rc win.RECT
	win.GetClientRect(hwnd, &rc)
	// FillRect(hdc, *RECT, hBrush) — เรียกผ่าน proc ที่ประกาศเอง (lxn/win ไม่ bind ให้)
	// ใช้ solid brush สีพื้น dialog (COLOR_BTNFACE) ทาทับทั้ง client area ลบเงาเก่าเกลี้ยง
	procFillRect.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(&rc)),
		uintptr(win.GetSysColorBrush(win.COLOR_BTNFACE)),
	)
}

// procGetUserDefaultUILanguage ใช้ตรวจภาษา UI ของ Windows เพื่อตั้งภาษาเริ่มต้น
// ให้ตรงกับเครื่องผู้ใช้ (modkernel32 ประกาศไว้ใน dialog_windows.go แพ็กเกจเดียวกัน)
var procGetUserDefaultUILanguage = modkernel32.NewProc("GetUserDefaultUILanguage")

// DefaultLang เดาภาษาเริ่มต้นจากภาษา UI ของ Windows — ไทย/ญี่ปุ่น/อังกฤษ ที่เหลือ
// fallback เป็นไทย (กลุ่มผู้ใช้หลัก) primary language id = LANGID ตัดเหลือ 10 บิตล่าง
func DefaultLang() Lang {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	switch uint16(r) & 0x3ff {
	case 0x1e: // LANG_THAI
		return LangTH
	case 0x11: // LANG_JAPANESE
		return LangJA
	case 0x09: // LANG_ENGLISH
		return LangEN
	default:
		return LangTH
	}
}

// wizardStepCount คือจำนวนหน้าที่โต้ตอบก่อน elevation: 0=ต้อนรับ 1=ยินยอม 2=ตำแหน่ง
const wizardStepCount = 3

// wizardUI ถือ state + reference ของ widget ที่อยู่คงที่ (header/footer) ระหว่าง
// อายุของหน้าต่าง — widget ใน content panel เป็นของชั่วคราว (สร้างใหม่ทุก step)
type wizardUI struct {
	mw      *walk.MainWindow
	content *walk.Composite // พื้นที่เนื้อหา (ถูกเติม panel ของ step ปัจจุบัน)
	panel   *walk.Composite // panel ของ step ปัจจุบัน (dispose ก่อนสร้างใหม่)

	backBtn   *walk.PushButton
	nextBtn   *walk.PushButton
	cancelBtn *walk.PushButton
	stepLbl   *walk.Label // "ขั้นที่ N จาก M" ชิดซ้ายบน — บอกตำแหน่งในวิซาร์ด
	langLbl   *walk.Label
	langCombo *walk.ComboBox

	serverURL  string
	step       int
	lang       Lang
	installDir string
	agreed     bool // ติ๊ก checkbox ยินยอมในหน้า consent แล้วหรือยัง
	rebuilding bool // กัน showStep ทำงานซ้อน (re-entrant) ระหว่างสร้าง panel — ดู showStep

	result WizardResult
}

// RunWizard แสดง wizard GUI และคืน (ผลลัพธ์, shown) — shown=false เมื่อสร้างหน้าต่าง
// ไม่ได้ ให้ผู้เรียก fallback ไป console ผู้ใช้ที่ปิดหน้าต่าง/กดยกเลิก → Proceed=false
func RunWizard(serverURL string, defaultLang Lang, defaultDir string) (WizardResult, bool) {
	ui := &wizardUI{
		serverURL:  serverURL,
		lang:       defaultLang,
		installDir: defaultDir,
	}
	t := textFor(ui.lang)

	err := (MainWindow{
		AssignTo: &ui.mw,
		Title:    "SoftSentry Agent — " + t.setupSuffix,
		// ฟอนต์ฐานของทั้งหน้าต่าง — system default ของ walk เล็ก (~8pt) อ่านยาก
		// โดยเฉพาะไทย/ญี่ปุ่น จึงตั้ง 10pt ให้ทุก widget สืบทอด (header/footer/เนื้อหา)
		Font: Font{PointSize: 10},
		// พื้นหลังจริงที่ root (clientComposite) ด้วย — กัน backgroundEffective ของ
		// widget ใดๆ ไล่ขึ้นมาจนสุดแล้วคืน nil (ทำให้ WM_ERASEBKGND ไม่ลบเงา)
		Background: SystemColorBrush{Color: walk.SysColorBtnFace},
		MinSize:    Size{Width: 620, Height: 520},
		Size:       Size{Width: 620, Height: 520},
		Layout:     VBox{Margins: Margins{Left: 16, Top: 12, Right: 16, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			// header: ป้าย+กล่องเลือกภาษา ชิดขวาบน (จำกัดความสูงไม่ให้ยืด)
			//
			// ต้องตั้ง Background สีจริงเหมือน content ด้วย มิฉะนั้น langLbl (static
			// control) ที่ความกว้างเปลี่ยนตามภาษา ("Language:" → "言語:" → "ภาษา:")
			// จะ "ซ้อน" ข้อความเก่าทุกครั้งที่สลับภาษา เพราะ backgroundEffective ไล่ขึ้น
			// ไปเจอ NullBrush → WM_ERASEBKGND ไม่ลบพื้นหลังของ label เดิม (เดิมแก้แค่
			// content จึงเหลือเงาที่แถบ header — เป็นจุดที่ผู้ใช้ยังเห็นบั๊กอยู่)
			Composite{
				MaxSize:    Size{Height: 36},
				Background: SystemColorBrush{Color: walk.SysColorBtnFace},
				Layout:     HBox{MarginsZero: true, Spacing: 6},
				Children: []Widget{
					// ป้ายบอกขั้นชิดซ้าย — ถ่วงสมดุลกับกล่องเลือกภาษาทางขวา
					// (ข้อความตั้งจริงใน updateNav ตามขั้น/ภาษาปัจจุบัน)
					Label{AssignTo: &ui.stepLbl, Text: ""},
					HSpacer{},
					Label{AssignTo: &ui.langLbl, Text: t.languageLabel + ":"},
					ComboBox{
						// ไม่ตั้ง OnCurrentIndexChanged ที่นี่ — attach หลัง Create เพื่อ
						// กัน handler ยิงตอนกำหนด CurrentIndex เริ่มต้น
						Model:        langLabels,
						CurrentIndex: langIndex(ui.lang),
						MinSize:      Size{Width: 140},
						MaxSize:      Size{Width: 160},
						AssignTo:     &ui.langCombo,
					},
				},
			},
			// เส้นคั่นบางใต้ header ให้แยกแถบเลือกภาษาออกจากเนื้อหา
			HSeparator{},
			// content: พื้นที่ยืดได้ เติม panel ของ step ปัจจุบัน
			//
			// ต้องตั้ง Background เป็นสีจริง (ไม่ใช่ NullBrush ที่ walk ตั้งให้ทุก
			// Composite โดยปริยาย) มิฉะนั้น backgroundEffective() จะไล่หา parent
			// ขึ้นไปจนถึง clientComposite ของ MainWindow ที่ background=nil แล้วคืน nil
			// ทำให้ตัวจัดการ WM_ERASEBKGND ของ walk "break" ไม่ลบพื้นหลังเลย → ตัวอักษร
			// ของ panel เก่าที่ dispose ไปแล้วค้างเป็นเงาซ้อนทุกครั้งที่สลับภาษา/step.
			// ใส่สี BtnFace (เทาเดียวกับพื้นหลัง dialog มาตรฐาน) ให้ทั้ง content และ
			// panel (สืบทอดผ่าน backgroundEffective) มีพื้นหลังจริงที่ FillRectangle
			// ทับเงาเก่าได้จริง
			Composite{
				AssignTo:   &ui.content,
				Background: SystemColorBrush{Color: walk.SysColorBtnFace},
				Layout:     VBox{MarginsZero: true},
			},
			// เส้นคั่นเหนือปุ่มนำทาง
			HSeparator{},
			// footer: ปุ่มนำทาง (จำกัดความสูง) — ตั้ง Background สีจริงด้วยเหตุผลเดียวกับ
			// header/content (ปุ่มวาดพื้นหลังตัวเองอยู่แล้ว แต่พื้นที่ว่างรอบปุ่มต้องมี
			// brush จริงให้ WM_ERASEBKGND ลบเงา ไม่งั้น backgroundEffective คืน nil)
			Composite{
				MaxSize:    Size{Height: 48},
				Background: SystemColorBrush{Color: walk.SysColorBtnFace},
				Layout:     HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					PushButton{AssignTo: &ui.backBtn, Text: t.back, MinSize: Size{Width: 96, Height: 34}, OnClicked: ui.onBack},
					HSpacer{},
					PushButton{AssignTo: &ui.cancelBtn, Text: t.cancel, MinSize: Size{Width: 96, Height: 34}, OnClicked: ui.onCancel},
					PushButton{AssignTo: &ui.nextBtn, Text: t.next, MinSize: Size{Width: 110, Height: 34}, OnClicked: ui.onNext},
				},
			},
		},
	}).Create()
	if err != nil {
		return WizardResult{}, false
	}
	// ตรึงขนาดหน้าต่าง: ปิด maximize/resize กัน UI เพี้ยน/ซ้อนตอนเปลี่ยน step
	lockFixedSize(ui.mw.Handle())

	// ผูก handler เปลี่ยนภาษาหลังสร้างเสร็จ — กันไม่ให้ยิงตอนตั้ง CurrentIndex เริ่มต้น
	ui.langCombo.CurrentIndexChanged().Attach(func() {
		idx := ui.langCombo.CurrentIndex()
		if idx < 0 || idx >= len(wizardLangs) {
			return
		}
		ui.lang = wizardLangs[idx]
		ui.refreshChrome() // อัปเดตข้อความ header/footer
		ui.showStep(ui.step)
	})

	// แม้ lockFixedSize ปิดปุ่ม maximize/กรอบ resize แล้ว แต่ Aero Snap (Win+ลูกศร),
	// double-click title bar หรือการเปลี่ยน DPI ระหว่างจอ ยังปรับขนาดหน้าต่างได้ ซึ่ง
	// relayout เนื้อหาแล้วทิ้งเงาซ้อน → force redraw ทั้งพื้นที่เนื้อหาทุกครั้งที่ขนาดเปลี่ยน
	ui.content.SizeChanged().Attach(func() {
		redrawFull(ui.content.Handle())
	})

	ui.showStep(0)
	ui.mw.Show()
	ui.mw.Run()
	return ui.result, true
}

// refreshChrome อัปเดตข้อความของ widget คงที่ (ป้ายภาษา + ปุ่มนำทาง) ตามภาษาปัจจุบัน
func (ui *wizardUI) refreshChrome() {
	t := textFor(ui.lang)
	ui.mw.SetTitle("SoftSentry Agent — " + t.setupSuffix)
	ui.langLbl.SetText(t.languageLabel + ":")
	ui.cancelBtn.SetText(t.cancel)
	ui.updateNav()
}

// updateNav ปรับสถานะปุ่มตาม step: ปุ่มย้อนกลับเปิดเมื่อไม่ใช่หน้าแรก, ปุ่มถัดไป
// กลายเป็น "ติดตั้ง" ที่หน้าสุดท้าย และถูก disable ในหน้า consent จนกว่าจะติ๊กยินยอม
func (ui *wizardUI) updateNav() {
	t := textFor(ui.lang)
	// ป้าย "ขั้นที่ N จาก M" (อัปเดตทั้งตอนเปลี่ยน step และตอนสลับภาษา เพราะ
	// updateNav ถูกเรียกจากทั้ง showStep และ refreshChrome)
	ui.stepLbl.SetText(fmt.Sprintf(t.stepIndicator, ui.step+1, wizardStepCount))
	ui.backBtn.SetText(t.back)
	ui.backBtn.SetEnabled(ui.step > 0)
	if ui.step == wizardStepCount-1 {
		ui.nextBtn.SetText(t.install)
	} else {
		ui.nextBtn.SetText(t.next)
	}
	// หน้า consent (step 1) ต้องติ๊กยินยอมก่อนจึงกดถัดไปได้
	ui.nextBtn.SetEnabled(ui.step != 1 || ui.agreed)
}

// showStep สร้างเนื้อหาของ step ที่ระบุใหม่ในภาษาปัจจุบัน — dispose panel เดิมก่อน
// แล้วสร้าง panel ใหม่ลงใน content (SetSuspended เพื่อ relayout ครั้งเดียวตอนจบ)
//
// กัน re-entrant: การ build panel (Create/SetSuspended/redraw) มีการ pump message ของ
// Win32 ระหว่างทาง ทำให้ event เช่น CurrentIndexChanged ของ combo เลือกภาษา "ยิงซ้ำ"
// แล้วเรียก showStep ซ้อนเข้ามากลางคัน → panel ที่สร้างค้างกลายเป็น child ที่ "หลุดการ
// ติดตาม" (ui.panel ชี้ไปตัวใหม่ ตัวเก่าไม่ถูก dispose) สะสมเป็นพาเนลซ้อนเรียงลงมา
// (อาการนี้โผล่เฉพาะบางเครื่อง/DPI/timing — บน native build อาจไม่เห็น) flag rebuilding
// ตัด re-entry ทิ้ง ส่วน clearContent ด้านล่างเก็บกวาด child ที่เคยหลุดให้หมดทุกครั้ง
func (ui *wizardUI) showStep(i int) {
	if ui.rebuilding {
		return
	}
	ui.rebuilding = true
	defer func() { ui.rebuilding = false }()

	ui.step = i
	ui.content.SetSuspended(true)
	ui.clearContent()
	// content ว่างแล้ว (ไม่มี child) → ลบพิกเซลเงาเก่าด้วย GDI ตรงๆ ทันที ก่อนสร้าง panel
	// ใหม่ เป็นการรับประกันว่าเงาหายจริงโดยไม่พึ่งกลไก erase ของ walk (ที่พิสูจน์แล้วว่า
	// ไม่ยอมลบแม้ตั้ง Background ครบ) — fix รอบสุดท้ายที่ลงไปถึงระดับ GDI
	forceEraseBg(ui.content.Handle())
	switch i {
	case 0:
		ui.buildWelcome()
	case 1:
		ui.buildConsent()
	case 2:
		ui.buildLocation()
	}
	ui.content.SetSuspended(false)
	// redraw "ทั้งหน้าต่าง" (ไม่ใช่แค่ content) — การสลับภาษาเปลี่ยนข้อความที่ header
	// (langLbl) + footer (ปุ่ม) ด้วย จึงต้อง erase+repaint ทั้ง client area ครอบคลุม
	// header/separator/footer ไม่งั้นเงาข้อความเก่าที่ header ยังค้าง
	redrawFull(ui.mw.Handle())
	ui.updateNav()
}

// clearContent ทำลาย child "ทุกตัว" ของ content (ไม่ใช่แค่ ui.panel ตัวที่ติดตามอยู่)
// — กันพาเนลเก่าที่ "หลุดการติดตาม" จาก re-entrant showStep ค้างเป็น live HWND แล้ว
// render ซ้อนกัน (ต้นเหตุอาการ "เปลี่ยนภาษาแล้วเพิ่มตัวใหม่ไม่ลบตัวเก่า"). ต้อง snapshot
// reference ลูกทั้งหมดก่อนวน Dispose เพราะ walk แก้ children list ระหว่าง Dispose (ผ่าน
// SetParent) ทำให้ index เลื่อนถ้าวนอ่านสดๆ. Dispose แต่ละตัว = DestroyWindow + ถอดออกจาก
// layout ของ content ครบ (child พวกนี้ parent ถูกต้องเสมอ จึงเข้าเงื่อนไขถอดใน Dispose)
func (ui *wizardUI) clearContent() {
	kids := ui.content.Children()
	stale := make([]walk.Widget, kids.Len())
	for i := range stale {
		stale[i] = kids.At(i)
	}
	for _, w := range stale {
		w.Dispose()
	}
	ui.panel = nil
}

// onBack/onNext/onCancel คือ handler ของปุ่มนำทาง
func (ui *wizardUI) onBack() {
	if ui.step > 0 {
		ui.showStep(ui.step - 1)
	}
}

func (ui *wizardUI) onNext() {
	if ui.step < wizardStepCount-1 {
		ui.showStep(ui.step + 1)
		return
	}
	// หน้าสุดท้าย — บันทึกผลแล้วปิดหน้าต่างเพื่อไปขั้นขอสิทธิ์ admin
	ui.result = WizardResult{Proceed: true, Lang: ui.lang, InstallDir: ui.installDir}
	ui.mw.Close()
}

func (ui *wizardUI) onCancel() {
	ui.result = WizardResult{} // Proceed=false
	ui.mw.Close()
}

// buildWelcome สร้างหน้าต้อนรับ (หัวข้อใหญ่ + คำอธิบายสั้น)
func (ui *wizardUI) buildWelcome() {
	t := textFor(ui.lang)
	_ = (Composite{
		AssignTo: &ui.panel,
		// พื้นหลังจริงของ panel เอง (สำคัญสุด!): panel คือ widget ที่วาง "ทับ" พื้นที่ที่
		// panel เก่าเพิ่ง dispose ไป ถ้า panel ไม่มี brush จริง (default = NullBrush) มันจะ
		// ไม่ทาพื้นหลังของตัวเอง → ตัวอักษรของ panel เก่าทะลุขึ้นมาเห็นเป็นเงาซ้อน แม้ content
		// /header/footer จะตั้ง brush ครบแล้วก็ตาม (fix ก่อนหน้าตั้งให้ container แต่ลืม panel)
		Background: SystemColorBrush{Color: walk.SysColorBtnFace},
		Layout:     VBox{Margins: Margins{Left: 12, Top: 20, Right: 12, Bottom: 8}, Spacing: 14},
		Children: []Widget{
			Label{Text: t.welcomeHeading, Font: Font{PointSize: 15, Bold: true}},
			Label{Text: crlf(t.welcomeIntro), Font: Font{PointSize: 11}},
			// ดันเนื้อหาขึ้นชิดบน — VSpacer เดียวด้านล่าง (ไม่ใช่สองตัวคร่อมที่ทำให้
			// ข้อความลอยกลางจอเกิดช่องว่างเยอะ)
			VSpacer{},
			// build stamp = "ลายนิ้วมือของ build" (เปลี่ยนทุกครั้งที่ build) โชว์ค่าเดียว
			// กับที่หน้า deploy แสดง → ผู้ใช้/แอดมินเทียบได้ทันทีว่า "ตัวที่กำลังรันนี้ =
			// ตัวล่าสุดที่ build จาก source จริงไหม" (Version ถูก fix 0.1.0 เลยใช้เทียบไม่ได้)
			Label{
				Text:      fmt.Sprintf("v%s · build %s", transport.Version, transport.BuildStamp),
				Font:      Font{PointSize: 8, Family: "Consolas"},
				TextColor: walk.RGB(0x88, 0x88, 0x88),
			},
		},
	}).Create(NewBuilder(ui.content))
}

// buildConsent สร้างหน้าข้อมูล+ยินยอม: หัวข้อ + กล่องข้อความอ่านอย่างเดียว (เลื่อนได้)
// + checkbox ยินยอม ซึ่งคุม enable ของปุ่ม "ถัดไป"
func (ui *wizardUI) buildConsent() {
	t := textFor(ui.lang)
	ci := BuildConsentInfoLang(ui.lang, ui.serverURL, ui.installDir)
	body := localizedConsentBody(ui.lang, ci)
	var agreeCB *walk.CheckBox
	_ = (Composite{
		AssignTo: &ui.panel,
		// พื้นหลังจริงของ panel เอง — กันตัวอักษร panel เก่าทะลุเป็นเงาซ้อน (ดู buildWelcome)
		Background: SystemColorBrush{Color: walk.SysColorBtnFace},
		Layout:     VBox{Margins: Margins{Left: 12, Top: 16, Right: 12, Bottom: 8}, Spacing: 12},
		Children: []Widget{
			Label{Text: t.consentHeading, Font: Font{PointSize: 15, Bold: true}},
			// TextEdit เป็น widget ที่ยืดได้ → กินพื้นที่ที่เหลือทั้งหมดและ reflow
			// ตามขนาดหน้าต่าง (MinSize กันไม่ให้เตี้ยจนต้องเลื่อนทันที)
			TextEdit{
				Text:     crlf(body),
				ReadOnly: true,
				VScroll:  true,
				MinSize:  Size{Height: 230},
			},
			CheckBox{
				AssignTo: &agreeCB,
				Text:     t.agree,
				Checked:  ui.agreed,
				OnCheckedChanged: func() {
					ui.agreed = agreeCB.Checked() // อ่านสถานะจริงของ checkbox
					ui.updateNav()
				},
			},
		},
	}).Create(NewBuilder(ui.content))
}

// buildLocation สร้างหน้าเลือกโฟลเดอร์ติดตั้ง: ช่องกรอก path + ปุ่มเรียกดู (เปิด
// native folder picker) ค่า path sync เข้า ui.installDir ทุกครั้งที่เปลี่ยน
func (ui *wizardUI) buildLocation() {
	t := textFor(ui.lang)
	var edit *walk.LineEdit
	_ = (Composite{
		AssignTo: &ui.panel,
		// พื้นหลังจริงของ panel เอง — กันตัวอักษร panel เก่าทะลุเป็นเงาซ้อน (ดู buildWelcome)
		Background: SystemColorBrush{Color: walk.SysColorBtnFace},
		Layout:     VBox{Margins: Margins{Left: 12, Top: 20, Right: 12, Bottom: 8}, Spacing: 10},
		Children: []Widget{
			Label{Text: t.locationHeading, Font: Font{PointSize: 15, Bold: true}},
			Label{Text: t.locationLabel},
			Composite{
				MaxSize: Size{Height: 40},
				Layout:  HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					LineEdit{
						AssignTo: &edit,
						Text:     ui.installDir,
						OnTextChanged: func() {
							ui.installDir = edit.Text()
						},
					},
					PushButton{
						Text:    t.browse,
						MinSize: Size{Width: 130, Height: 34},
						MaxSize: Size{Width: 130},
						OnClicked: func() {
							dlg := &walk.FileDialog{
								Title:          t.browseTitle,
								InitialDirPath: ui.installDir,
							}
							if ok, _ := dlg.ShowBrowseFolder(ui.mw); ok && dlg.FilePath != "" {
								edit.SetText(dlg.FilePath)
							}
						},
					},
				},
			},
			VSpacer{},
		},
	}).Create(NewBuilder(ui.content))
}

// RunInstallProgress แสดงหน้าต่างความคืบหน้า (ใน instance ที่ elevated แล้ว) พร้อม
// รัน run() ใน goroutine แยก แล้วอัปเดตข้อความแต่ละขั้นผ่าน Synchronize เมื่อ run()
// จบ หน้าต่างจะปิดเอง คืน (error จาก run, shown) — shown=false ถ้าสร้างหน้าต่างไม่ได้
//
// run รับ callback setStep(i) เพื่อรายงานว่ากำลังอยู่ขั้นไหน (0..3 ตรงกับ progressSteps)
func RunInstallProgress(lang Lang, run func(setStep func(int)) error) (error, bool) {
	t := textFor(lang)
	var (
		mw     *walk.MainWindow
		status *walk.Label
		runErr error
	)
	err := (MainWindow{
		AssignTo: &mw,
		Title:    "SoftSentry Agent — " + t.setupSuffix,
		Font:     Font{PointSize: 10},
		MinSize:  Size{Width: 500, Height: 200},
		Size:     Size{Width: 500, Height: 200},
		Layout:   VBox{Margins: Margins{Left: 20, Top: 18, Right: 20, Bottom: 16}, Spacing: 12},
		Children: []Widget{
			Label{Text: t.progressHeading, Font: Font{PointSize: 14, Bold: true}},
			ProgressBar{MarqueeMode: true, MaxSize: Size{Height: 22}},
			Label{AssignTo: &status, Text: t.progressSteps[0]},
			VSpacer{},
		},
	}).Create()
	if err != nil {
		return nil, false
	}
	lockFixedSize(mw.Handle())

	setStep := func(i int) {
		if i < 0 || i >= len(t.progressSteps) {
			return
		}
		mw.Synchronize(func() { status.SetText(t.progressSteps[i]) })
	}

	go func() {
		e := run(setStep)
		// เขียน runErr + ปิดหน้าต่างบน UI thread (Synchronize) เพื่อ happens-before
		mw.Synchronize(func() {
			runErr = e
			mw.Close()
		})
	}()

	mw.Show()
	mw.Run()
	return runErr, true
}

// localizedConsentBody ประกอบเนื้อหา consent เป็นข้อความหลายบรรทัด (ตามภาษา) สำหรับ
// แสดงในกล่อง TextEdit ของหน้า consent — ใช้ป้ายกำกับจาก wizText + ค่าจาก ConsentInfo
func localizedConsentBody(lang Lang, ci ConsentInfo) string {
	t := textFor(lang)
	var b strings.Builder
	b.WriteString(ci.Purpose + "\n\n")
	b.WriteString(t.labelInstallAt + " : " + ci.InstallDir + "\n")
	b.WriteString(t.labelRunsAs + " : " + ci.RunsAs + "\n")
	b.WriteString(t.labelPermission + " : " + ci.Permission + "\n")
	b.WriteString(t.labelReportsTo + " : " + ci.ServerURL + "\n\n")
	b.WriteString(t.labelCollected + "\n")
	for _, d := range ci.DataCollected {
		b.WriteString("  • " + d + "\n")
	}
	b.WriteString("\n" + t.labelNotKept + "\n")
	for _, d := range ci.DataNotKept {
		b.WriteString("  • " + d + "\n")
	}
	b.WriteString("\n" + ci.UninstallHint)
	return b.String()
}

// crlf แปลง \n เป็น \r\n เพื่อให้ Windows static/edit control ขึ้นบรรทัดใหม่ถูกต้อง
func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
