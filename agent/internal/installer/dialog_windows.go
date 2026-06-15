//go:build windows

// ไฟล์นี้ให้ "หน้าจอแบบ GUI" สำหรับ self-install บน Windows แทนการพ่นข้อความใน
// console สีดำ ใช้เฉพาะ Win32 API ที่มีติดเครื่องอยู่แล้ว — ไม่เพิ่ม dependency:
//
//   - TaskDialog (comctl32 v6)  : กล่อง dialog สมัยใหม่ หน้าตาเป็นทางการ
//   - MessageBoxW (user32)      : fallback คลาสสิก มีติดทุกเครื่องเสมอ
//   - GetConsoleWindow/ShowWindow: ซ่อน/แสดงหน้าต่าง console สีดำ
//
// กลยุทธ์แบบมีหลายชั้นเพื่อความทนทาน (เพราะ GUI โค้ดนี้ unit-test ไม่ได้):
// ลอง TaskDialog ก่อน → ถ้า comctl32 v6 ไม่ active (เช่น build แบบไม่มี manifest)
// proc จะหาไม่เจอ → ตกไปใช้ MessageBoxW (ยังเป็น GUI) → ถ้ายังไม่ได้อีกค่อยให้
// ผู้เรียกตกไปใช้ console (shown=false)
package installer

import (
	"strings" // ใช้ประกอบข้อความเนื้อหา dialog
	"unsafe"  // ใช้ส่ง pointer ของ UTF-16 string และ output button ให้ Win32

	"golang.org/x/sys/windows" // LazyDLL/LazyProc + UTF16PtrFromString
)

// โหลด DLL/proc แบบ lazy — proc จะถูก resolve ตอนเรียกครั้งแรก ทำให้ตรวจได้ว่า
// TaskDialog มีหรือไม่ (comctl32 v5 ที่โหลด default เมื่อไม่มี v6 manifest จะไม่
// export TaskDialog → Find() คืน error → เรารู้ว่าต้อง fallback)
var (
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")
	moduser32   = windows.NewLazySystemDLL("user32.dll")
	modcomctl32 = windows.NewLazySystemDLL("comctl32.dll")

	procGetConsoleWindow = modkernel32.NewProc("GetConsoleWindow")
	procShowWindow       = moduser32.NewProc("ShowWindow")
	procMessageBoxW      = moduser32.NewProc("MessageBoxW")
	procTaskDialog       = modcomctl32.NewProc("TaskDialog")
)

// ค่าคงที่ Win32 ที่ใช้ — ตั้งชื่อให้ตรงกับ SDK เพื่อตรวจสอบง่าย
const (
	swHide = 0 // SW_HIDE — ซ่อนหน้าต่าง
	swShow = 5 // SW_SHOW — แสดงหน้าต่างกลับ

	// flags ของ MessageBoxW
	mbOK              = 0x00000000 // MB_OK — ปุ่ม OK ปุ่มเดียว
	mbYesNo           = 0x00000004 // MB_YESNO — ปุ่ม Yes/No
	mbIconError       = 0x00000010 // MB_ICONERROR
	mbIconQuestion    = 0x00000020 // MB_ICONQUESTION
	mbIconInformation = 0x00000040 // MB_ICONINFORMATION
	mbSetForeground   = 0x00010000 // MB_SETFOREGROUND — ดึงมาหน้าสุด
	mbTopMost         = 0x00040000 // MB_TOPMOST — ลอยอยู่บนสุด

	// รหัสปุ่มที่ dialog คืนกลับ (เหมือนกันทั้ง MessageBox และ TaskDialog)
	idOK  = 1 // IDOK
	idYes = 6 // IDYES

	// common-button flags ของ TaskDialog
	tdcbfOKButton  = 0x0001 // TDCBF_OK_BUTTON
	tdcbfYesButton = 0x0002 // TDCBF_YES_BUTTON
	tdcbfNoButton  = 0x0004 // TDCBF_NO_BUTTON
)

// tdIcon แปลง standard-icon id (ค่าติดลบ) เป็นค่า pszIcon ตาม MAKEINTRESOURCEW
// — MAKEINTRESOURCEW(i) = (PWSTR)(WORD)i เช่น TD_SHIELD_ICON = -4 → 0xFFFC
func tdIcon(i int16) uintptr { return uintptr(uint16(i)) }

var (
	tdShieldIcon = tdIcon(-4) // TD_SHIELD_ICON — โล่ความปลอดภัย ใช้กับหน้า consent
	tdInfoIcon   = tdIcon(-3) // TD_INFORMATION_ICON — ใช้กับหน้าสำเร็จ
	tdErrorIcon  = tdIcon(-2) // TD_ERROR_ICON — ใช้กับหน้าล้มเหลว
)

// HideConsoleWindow ซ่อนหน้าต่าง console สีดำของโพรเซสนี้ (ถ้ามี) เรียกตอนเริ่ม
// self-install เพื่อให้ผู้ใช้เห็นเฉพาะ GUI dialog ไม่เห็นจอ CMD
func HideConsoleWindow() {
	if h, _, _ := procGetConsoleWindow.Call(); h != 0 {
		_, _, _ = procShowWindow.Call(h, swHide)
	}
}

// ShowConsoleWindow แสดงหน้าต่าง console กลับมา ใช้เมื่อจำเป็นต้องตกไปใช้ console
// (เช่น GUI แสดงไม่ได้) ผู้ใช้จะได้เห็น prompt/ข้อความ
func ShowConsoleWindow() {
	if h, _, _ := procGetConsoleWindow.Call(); h != 0 {
		_, _, _ = procShowWindow.Call(h, swShow)
	}
}

// messageBox แสดง MessageBoxW และคืนรหัสปุ่มที่กด (0 = แสดงไม่สำเร็จ) เพิ่ม
// SetForeground+TopMost เสมอเพื่อให้ dialog ไม่ไปโผล่หลังหน้าต่างอื่น
func messageBox(text, caption string, flags uintptr) int {
	t, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return 0
	}
	c, err := windows.UTF16PtrFromString(caption)
	if err != nil {
		return 0
	}
	r, _, _ := procMessageBoxW.Call(
		0, // ไม่มี parent window
		uintptr(unsafe.Pointer(t)),
		uintptr(unsafe.Pointer(c)),
		flags|mbSetForeground|mbTopMost,
	)
	return int(r)
}

// taskDialog แสดง TaskDialog (กล่องสมัยใหม่) คืน (รหัสปุ่ม, true) เมื่อแสดงสำเร็จ
// คืน (0, false) เมื่อ TaskDialog ใช้ไม่ได้ (comctl32 v6 ไม่ active) หรือ HRESULT
// ไม่ใช่ S_OK — ผู้เรียกต้อง fallback ไป messageBox ต่อ
func taskDialog(title, instruction, content string, commonButtons, icon uintptr) (int, bool) {
	// ถ้า proc หาไม่เจอ แปลว่า comctl32 v6 ไม่ active → ใช้ TaskDialog ไม่ได้
	if err := procTaskDialog.Find(); err != nil {
		return 0, false
	}
	tp, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return 0, false
	}
	ip, err := windows.UTF16PtrFromString(instruction)
	if err != nil {
		return 0, false
	}
	cp, err := windows.UTF16PtrFromString(content)
	if err != nil {
		return 0, false
	}
	var btn int32 // pnButton — รหัสปุ่มที่กด ถูกเขียนกลับโดย TaskDialog
	hr, _, _ := procTaskDialog.Call(
		0, // hwndParent — ไม่มี
		0, // hInstance — ไม่ใช้ resource จาก module
		uintptr(unsafe.Pointer(tp)),
		uintptr(unsafe.Pointer(ip)),
		uintptr(unsafe.Pointer(cp)),
		commonButtons,
		icon,
		uintptr(unsafe.Pointer(&btn)),
	)
	if int32(hr) != 0 { // S_OK == 0; อื่นๆ ถือว่าแสดงไม่สำเร็จ
		return 0, false
	}
	return int(btn), true
}

// consentBody ประกอบ "เนื้อหา" ของหน้า consent จาก ConsentInfo (ไม่รวมหัวข้อใหญ่
// ซึ่งเป็น main instruction แยกต่างหาก) ใช้ร่วมกันทั้ง TaskDialog และ MessageBox
func consentBody(ci ConsentInfo) string {
	var b strings.Builder
	b.WriteString(ci.Purpose)
	b.WriteString("\n\n")
	b.WriteString("ติดตั้งที่ : " + ci.InstallDir + "\n")
	b.WriteString("ทำงานเป็น : " + ci.RunsAs + "\n")
	b.WriteString("สิทธิ์ที่ใช้ : " + ci.Permission + "\n")
	b.WriteString("รายงานไป : " + ci.ServerURL + "\n\n")
	b.WriteString("ส่งให้เซิร์ฟเวอร์:\n")
	for _, d := range ci.DataCollected {
		b.WriteString("  • " + d + "\n")
	}
	b.WriteString("\nไม่เก็บ:\n")
	for _, d := range ci.DataNotKept {
		b.WriteString("  • " + d + "\n")
	}
	b.WriteString("\n" + ci.UninstallHint)
	return b.String()
}

// ConfirmInstallGUI แสดงหน้า consent แบบ GUI และคืน consent=true เมื่อผู้ใช้กด
// "ใช่/ติดตั้ง" shown=false เมื่อแสดง GUI ไม่ได้เลย (ผู้เรียกต้อง fallback console)
func ConfirmInstallGUI(ci ConsentInfo) (consented bool, shown bool) {
	title := ci.AppName + " — ตัวติดตั้ง"
	instruction := "ติดตั้ง " + ci.AppName + " บนเครื่องนี้?"
	body := consentBody(ci)

	// ชั้น 1: TaskDialog (สวยที่สุด)
	if btn, ok := taskDialog(title, instruction, body, tdcbfYesButton|tdcbfNoButton, tdShieldIcon); ok {
		return btn == idYes, true
	}
	// ชั้น 2: MessageBox (มีติดทุกเครื่อง) — รวมหัวข้อ+เนื้อหาเป็นข้อความเดียว
	if r := messageBox(instruction+"\n\n"+body, title, mbYesNo|mbIconQuestion); r != 0 {
		return r == idYes, true
	}
	// ชั้น 3: แสดง GUI ไม่ได้ → ให้ผู้เรียกใช้ console
	return false, false
}

// ShowResultGUI แสดงผลลัพธ์การติดตั้ง (สำเร็จ/ล้มเหลว) แบบ GUI คืน shown=false
// เมื่อแสดงไม่ได้ (ผู้เรียก fallback ไป console)
func ShowResultGUI(success bool, heading, message string) (shown bool) {
	const win = "SoftSentry Agent" // ชื่อบน title bar
	tdIco, mbIco := tdErrorIcon, uintptr(mbIconError)
	if success {
		tdIco, mbIco = tdInfoIcon, uintptr(mbIconInformation)
	}
	// ชั้น 1: TaskDialog
	if _, ok := taskDialog(win, heading, message, tdcbfOKButton, tdIco); ok {
		return true
	}
	// ชั้น 2: MessageBox
	if r := messageBox(heading+"\n\n"+message, win, mbOK|mbIco); r != 0 {
		return true
	}
	return false
}
