// ไฟล์นี้จัดการกระบวนการ "self-install" ทั้งหมด — เมื่อผู้ใช้ดับเบิลคลิก
// SoftSentry-Setup.exe ที่ดาวน์โหลดมาจาก dashboard (/deploy page)
// ไฟล์ Setup.exe คือไบนารี agent ปกติที่มี config trailer (server URL + deployment token)
// ฝังต่อท้ายโดย backend ตอนสร้าง installer ให้ผู้ใช้
//
// ขั้นตอนของ self-install:
//   1. แสดง disclosure และขอ consent จากผู้ใช้
//   2. ขอสิทธิ์ admin ผ่าน UAC (Windows) หรือ sudo (macOS)
//   3. คัดลอกตัวเองไปที่ Program Files / /usr/local/softsentry
//   4. Enroll เครื่องด้วย deployment token ที่ฝังมา
//   5. ลงทะเบียนและเริ่ม OS background service
package main

import (
	"bufio"        // ใช้อ่าน input จากผู้ใช้บรรทัดต่อบรรทัด และอ่าน Enter key
	"context"      // ใช้ส่ง context ให้ enrollMachine เพื่อรองรับการยกเลิก
	"fmt"          // ใช้พิมพ์ข้อความ progress, disclosure, และ error
	"io"           // ใช้รับ io.Writer และ io.Reader เพื่อให้ test inject ได้
	"os"           // ใช้ดู OS argument, อ่าน stdin, เขียนไฟล์, และหา env var
	"path/filepath" // ใช้ต่อ path และหาชื่อไฟล์
	"runtime"      // ใช้ตรวจสอบ OS ปัจจุบัน (GOOS) เพื่อเลือก path และชื่อไฟล์
	"strings"      // ใช้ build disclosure text และแปลง input เป็น lowercase
	"time"         // ใช้สร้าง timestamp สำหรับชื่อไฟล์ backup และ countdown timer

	"github.com/softsentry/agent/internal/installer" // ใช้อ่าน config trailer, ตรวจสิทธิ์ admin, relaunch elevated, คัดลอกไบนารี
	"github.com/softsentry/agent/internal/service"   // ใช้ตรวจสอบ/ถอดถอน/ติดตั้ง OS service
)

// elevatedInstallFlag is the internal argument we pass to ourselves when
// relaunching under UAC. Its presence means "the user already saw the
// disclosure and consented in the first window" — so the elevated instance
// skips the prompt and goes straight to installing.
//
// (TH) เป็น argument ภายในที่เราส่งให้ตัวเองตอน relaunch ภายใต้ UAC การมีอยู่ของ
// มันหมายความว่า "ผู้ใช้เห็น disclosure และยินยอมแล้วในหน้าต่างแรก" ดังนั้น
// instance ที่ elevated แล้วจะข้ามการถามและเริ่มติดตั้งไปเลย
// ใช้ค่านี้เป็น secret handshake ระหว่าง instance ก่อนและหลัง UAC elevation
const elevatedInstallFlag = "--accept-install"

// maybeSelfInstall handles the "user double-clicked the downloaded
// SoftSentry-Setup.exe" path. The downloaded binary carries an embedded config
// trailer (server URL + deployment token) appended by the backend. When the
// agent is launched with no subcommand AND that trailer is present, it installs
// itself with zero typing: elevate → copy into place → enroll → register the
// service.
//
// It returns handled=true when it took over (the caller must not run the normal
// CLI). A binary without a trailer (an ordinary CLI build) returns
// handled=false so Cobra runs as usual.
//
// (TH) จัดการกรณี "ผู้ใช้ดับเบิลคลิก SoftSentry-Setup.exe ที่ดาวน์โหลดมา" ไบนารี
// ที่ดาวน์โหลดมาจะมี config trailer ฝังอยู่ (server URL + deployment token) ที่
// backend แนบต่อท้ายไว้ เมื่อ agent ถูกรันโดยไม่มี subcommand และมี trailer นี้อยู่
// มันจะติดตั้งตัวเองโดยไม่ต้องพิมพ์อะไรเลย: ขอสิทธิ์ elevate → คัดลอกไปวาง →
// enroll → ลงทะเบียน service
//
// คืนค่า handled=true เมื่อมันรับช่วงต่อ (ผู้เรียกต้องไม่รัน CLI ปกติ) ส่วนไบนารี
// ที่ไม่มี trailer (CLI build ทั่วไป) จะคืนค่า handled=false เพื่อให้ Cobra ทำงาน
// ตามปกติ
func maybeSelfInstall(ctx context.Context) (handled bool, err error) {
	// Two entry shapes count as a self-install: the bare double-click (no args),
	// and our own elevated continuation (we relaunch with elevatedInstallFlag
	// after the user consents). Anything else is an ordinary CLI invocation.
	//
	// (TH) มีการเริ่มต้นสองรูปแบบที่นับเป็น self-install: การดับเบิลคลิกเฉยๆ
	// (ไม่มี arg) และการ relaunch ต่อเนื่องของตัวเราเอง (relaunch พร้อม
	// elevatedInstallFlag หลังผู้ใช้ยินยอมแล้ว) นอกเหนือจากนี้ถือเป็นการเรียกใช้
	// CLI แบบปกติ
	// ตรวจสอบว่านี่คือ elevated continuation (relaunch จาก UAC) หรือไม่
	continuation := len(os.Args) == 2 && os.Args[1] == elevatedInstallFlag
	// ถ้ามี argument อื่นๆ (ไม่ใช่ no-arg หรือ elevated continuation) ให้ข้ามไป
	if len(os.Args) != 1 && !continuation {
		return false, nil // เป็นการเรียก CLI ปกติ — ปล่อยให้ Cobra จัดการ
	}

	// หา path ของ executable ที่กำลังรัน เพื่ออ่าน config trailer จากท้ายไฟล์
	exe, err := os.Executable()
	if err != nil {
		// หา path ตัวเองไม่ได้ — ย้อนกลับไปใช้ CLI ปกติ
		// fall back to normal CLI
		// (TH) ย้อนกลับไปใช้ CLI ปกติ
		return false, nil
	}
	// อ่าน config trailer จากท้ายไฟล์ executable
	// emb จะมีข้อมูล ServerURL, Token และ OriginalLen (ขนาดไบนารีจริงไม่รวม trailer)
	emb, ok, err := installer.ReadEmbeddedConfig(exe)
	if err != nil {
		// อ่าน trailer ล้มเหลวด้วย error (ไม่ใช่แค่ไม่มี trailer)
		return false, fmt.Errorf("read embedded installer config: %w", err)
	}
	if !ok {
		return false, nil // ordinary binary — let Cobra handle it
		// (TH) ไบนารีปกติที่ไม่มี trailer — ปล่อยให้ Cobra จัดการ
	}

	// มี trailer อยู่ — รัน self-install flow
	return true, runSelfInstall(ctx, exe, emb, continuation)
}

// runSelfInstall performs the install. The first (non-elevated) launch shows a
// full disclosure and asks for explicit consent BEFORE anything is changed or
// admin is requested; if the user declines, nothing happens. After consent, on
// Windows it relaunches itself via UAC (passing elevatedInstallFlag so the
// elevated instance skips the prompt) and returns. Once elevated it copies
// itself into Program Files, enrolls with the embedded deployment token, and
// registers + starts the OS service.
//
// preConsented is true only for the elevated continuation we launched
// ourselves — the user already agreed, so we don't ask twice.
//
// (TH) ทำการติดตั้งจริง การรันครั้งแรก (ที่ยังไม่ elevated) จะแสดงข้อมูลเปิดเผย
// แบบเต็มและขอความยินยอมอย่างชัดเจน ก่อนที่จะมีการเปลี่ยนแปลงใดๆ หรือขอสิทธิ์
// ผู้ดูแลระบบ ถ้าผู้ใช้ปฏิเสธ จะไม่มีอะไรเกิดขึ้น หลังจากยินยอมแล้ว บน Windows
// มันจะ relaunch ตัวเองผ่าน UAC (โดยส่ง elevatedInstallFlag เพื่อให้ instance ที่
// elevated ข้ามการถาม) แล้ว return กลับ เมื่อ elevated แล้วมันจะคัดลอกตัวเองไปไว้
// ใน Program Files, enroll ด้วย deployment token ที่ฝังมา และลงทะเบียน + เริ่ม
// OS service
//
// preConsented จะเป็น true เฉพาะตอนที่เป็นการ relaunch ต่อเนื่องที่เราเปิดขึ้นมาเอง
// — ผู้ใช้ยินยอมไปแล้ว จึงไม่ต้องถามซ้ำ
func runSelfInstall(ctx context.Context, exe string, emb installer.Embedded, preConsented bool) error {
	// บน Windows ซ่อนหน้าต่าง console สีดำทันทีที่เริ่ม self-install — ทุกอย่างที่
	// ผู้ใช้เห็นจะเป็น GUI dialog แทน (บน non-Windows เป็น no-op, ใช้ console ตามเดิม)
	installer.HideConsoleWindow()

	// ถ้ายังไม่ได้รับ consent (instance แรกที่ไม่ใช่ elevated continuation)
	if !preConsented {
		// Informed consent up front, before requesting admin or touching disk.
		// (TH) ขอความยินยอมแบบรับทราบข้อมูลก่อนเริ่ม ก่อนจะขอสิทธิ์ผู้ดูแลระบบ
		// หรือแตะต้องดิสก์ — หลักการ privacy by design
		if !confirmConsent(emb) {
			// ผู้ใช้ปฏิเสธการติดตั้ง — ไม่มีการเปลี่ยนแปลงใดๆ เกิดขึ้น เงียบๆ
			return nil
		}
		// ตรวจสอบว่ากำลังรันด้วยสิทธิ์ admin อยู่แล้วหรือไม่
		if !installer.IsElevated() {
			// ยังไม่มีสิทธิ์ admin — relaunch ผ่าน UAC (consent ผ่านแล้ว ส่ง flag ไปด้วย)
			if err := installer.RelaunchElevated(exe, []string{elevatedInstallFlag}); err != nil {
				// ขอสิทธิ์ admin ล้มเหลว (ผู้ใช้คลิก "No" ใน UAC หรือ error อื่น)
				return showInstallError(fmt.Errorf("ขอสิทธิ์ผู้ดูแลระบบไม่สำเร็จ: %w", err))
			}
			// instance ที่ elevated แล้วจะทำงานต่อในหน้าต่างของมันเอง — ตัวนี้จบ
			return nil
		}
	}

	// ถึงตรงนี้ = มีสิทธิ์ admin แล้ว — ลงมือติดตั้งจริง
	if err := performInstall(ctx, exe, emb); err != nil {
		return showInstallError(err)
	}
	showInstallSuccess()
	return nil
}

// confirmConsent ขอความยินยอมจากผู้ใช้ ผ่าน GUI dialog ก่อน (บน Windows) แล้ว
// fallback เป็น console prompt ถ้าแสดง GUI ไม่ได้ (non-Windows หรือ dialog error)
// คืน true เมื่อผู้ใช้อนุญาตให้ติดตั้ง
func confirmConsent(emb installer.Embedded) bool {
	ci := buildConsentInfo(emb) // แหล่งข้อมูล disclosure กลาง (ไม่มี token)
	if consented, shown := installer.ConfirmInstallGUI(ci); shown {
		return consented
	}
	// GUI ใช้ไม่ได้ → ตกไปใช้ console (unhide console เผื่อถูกซ่อนไว้ก่อนหน้า)
	installer.ShowConsoleWindow()
	return confirmInstall(os.Stdout, os.Stdin, emb)
}

// performInstall ทำขั้นตอนติดตั้งจริงทั้ง 4 ขั้น (ต้องมีสิทธิ์ admin แล้ว):
// ลบเวอร์ชันเก่า → คัดลอกไฟล์ → enroll เครื่อง → ลงทะเบียน+เริ่ม service
// progress เขียนลง stdout (บน Windows console ถูกซ่อน จึงมองไม่เห็น — แต่ยังเก็บไว้
// เป็น log และเป็นช่องทางแสดงผลของ console fallback บน non-Windows)
func performInstall(ctx context.Context, exe string, emb installer.Embedded) error {
	out := os.Stdout

	// แสดง header การติดตั้ง
	fmt.Fprintln(out, "============================================")
	fmt.Fprintln(out, "  SoftSentry Agent — กำลังติดตั้ง")
	fmt.Fprintf(out, "  Server: %s\n", emb.Config.ServerURL) // server ที่จะรายงานไป
	fmt.Fprintln(out, "============================================")
	fmt.Fprintln(out)

	// หา install directory ตาม OS (Program Files บน Windows, /usr/local/softsentry บน macOS)
	dir, err := installDir()
	if err != nil {
		return err
	}
	// สร้าง install directory ถ้ายังไม่มี (MkdirAll ไม่ error ถ้ามีอยู่แล้ว)
	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 — program dir
		return fmt.Errorf("create install dir: %w", err)
	}
	// path เต็มของ agent binary ที่จะติดตั้ง เช่น C:\Program Files\SoftSentry\softsentry-agent.exe
	dstExe := filepath.Join(dir, exeName())

	// Step 1 — replace any previous install. If the agent is already installed, a
	// previous instance is almost certainly running as a service and holding
	// dstExe open — on Windows the OS locks a running .exe. Stop and remove the
	// old service first so the binary unlocks and the displaced-file cleanup in
	// replaceBinary can succeed cleanly. Best-effort: NOT fatal if it fails,
	// because replaceBinary tolerates a still-running old binary (rename-aside)
	// and service.Install re-creates the service even if one remains.
	//
	// (TH) ขั้นที่ 1 — แทนที่การติดตั้งเดิม (ถ้ามี) ถ้า agent ถูกติดตั้งอยู่แล้ว
	// instance เดิมแทบจะแน่นอนว่ากำลังรันเป็น service และถือ dstExe ค้างอยู่ — บน
	// Windows ระบบปฏิบัติการจะล็อกไฟล์ .exe ที่กำลังรัน จึงต้องหยุดและลบ service
	// เก่าก่อน เพื่อให้ไฟล์ปลดล็อกและขั้นตอน cleanup ใน replaceBinary ทำงานได้
	// สำเร็จ ทำแบบ best-effort: ไม่ถือเป็น fatal ถ้าล้มเหลว เพราะ replaceBinary
	// รองรับกรณีไบนารีเก่ายังรันอยู่ (ย้ายออกไปก่อน) และ service.Install จะสร้าง
	// service ใหม่ให้แม้จะยังมีของเก่าหลงเหลืออยู่
	step(out, 1, "Removing any previous version")
	if st, err := service.Status(); err == nil && st != "not installed" {
		if err := service.Uninstall(); err != nil {
			fmt.Fprintf(out, "      (old service not fully removed: %v; continuing)\n", err)
		}
	}
	stepDone(out)

	// Step 2 — copy the agent into Program Files.
	// (TH) ขั้นที่ 2 — คัดลอก agent ไปไว้ใน Program Files (ตัด trailer ออกด้วย OriginalLen)
	step(out, 2, "Installing files")
	if err := replaceBinary(exe, dstExe, emb.OriginalLen); err != nil {
		return fmt.Errorf("copy agent into place: %w", err)
	}
	stepDone(out)

	// Step 3 — enroll this machine with the embedded deployment token.
	// (TH) ขั้นที่ 3 — enroll เครื่องนี้ด้วย deployment token ที่ฝังมา (ซ่อน progress ของ enroll)
	step(out, 3, "Enrolling this machine")
	if err := enrollMachine(ctx, io.Discard, emb.Config.Token, emb.Config.ServerURL); err != nil {
		return fmt.Errorf("enroll: %w", err)
	}
	stepDone(out)

	// Step 4 — register + start the background service.
	// (TH) ขั้นที่ 4 — ลงทะเบียน + เริ่ม background service
	step(out, 4, "Starting background service")
	if err := service.Install(service.Config{ExePath: dstExe, Args: []string{"run"}}); err != nil {
		return fmt.Errorf("install service: %w", err)
	}
	stepDone(out)
	return nil
}

// showInstallSuccess แจ้งผลสำเร็จแบบ GUI (บน Windows) หรือ console (fallback)
func showInstallSuccess() {
	const heading = "ติดตั้งสำเร็จ"
	const msg = "SoftSentry Agent ติดตั้งและกำลังทำงานแล้ว\n" +
		"เครื่องนี้จะปรากฏใน dashboard ภายใน 1 นาที"
	if installer.ShowResultGUI(true, heading, msg) {
		return
	}
	// console fallback (non-Windows / GUI แสดงไม่ได้)
	out := os.Stdout
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  ✓ %s — %s\n", heading, msg)
	closeWithCountdown(out, 8) // รอ 8 วินาทีแล้วปิดหน้าต่างอัตโนมัติ
}

// showInstallError แจ้งผลล้มเหลวแบบ GUI (บน Windows) หรือ console (fallback)
// แล้วคืน err เดิมกลับไปให้ผู้เรียก (เพื่อให้ exit code สะท้อนความล้มเหลว) — ผู้เรียก
// ไม่ต้องแสดง error ซ้ำอีก เพราะฟังก์ชันนี้แสดงให้ผู้ใช้แล้ว
func showInstallError(err error) error {
	const heading = "การติดตั้งล้มเหลว"
	msg := err.Error() + "\n\nลองดาวน์โหลดและเรียกใช้ตัวติดตั้งอีกครั้ง " +
		"หากยังไม่สำเร็จ โปรดติดต่อทีม IT"
	if installer.ShowResultGUI(false, heading, msg) {
		return err
	}
	// console fallback (non-Windows / GUI แสดงไม่ได้)
	installer.ShowConsoleWindow()
	fmt.Fprintln(os.Stderr, "error:", err)
	waitForExit(os.Stdout)
	return err
}

// buildConsentInfo รวบรวมข้อเท็จจริงสำหรับ disclosure จาก embedded config +
// install directory ของเครื่องนี้ เป็นแหล่งข้อมูลกลางที่ทั้ง console renderer
// (installDisclosure) และ GUI renderer (Win32 TaskDialog) ใช้ร่วมกัน
//
// installDir() อาจคืน error — ในกรณีนั้นปล่อย dir ว่างให้ BuildConsentInfo ใส่
// คำอธิบายทั่วไปแทน (disclosure ยังอ่านรู้เรื่อง) token ไม่ถูกส่งต่อไป (ความลับ)
func buildConsentInfo(emb installer.Embedded) installer.ConsentInfo {
	dir, _ := installDir() // error → dir == "" → BuildConsentInfo fallback
	return installer.BuildConsentInfo(emb.Config.ServerURL, dir)
}

// installDisclosure returns the plain-language consent screen shown before any
// change is made: what gets installed, where, the permission required, the
// server it reports to, and exactly what data is (and is not) collected. Kept a
// pure function (rendered from ConsentInfo) so it is unit-testable. The
// deployment token is intentionally never shown — it is a secret.
//
// (TH) คืนค่าหน้าจอขอความยินยอมแบบภาษาธรรมดา ที่แสดงก่อนจะมีการเปลี่ยนแปลงใดๆ:
// จะติดตั้งอะไร ที่ไหน ต้องใช้สิทธิ์อะไร รายงานไปยังเซิร์ฟเวอร์ไหน และข้อมูล
// อะไรบ้างที่ถูก (และไม่ถูก) เก็บ render จาก ConsentInfo เพื่อให้ unit-test ได้
// ส่วน deployment token จะไม่ถูกแสดงโดยเจตนา — เพราะเป็นความลับ
func installDisclosure(emb installer.Embedded) string {
	ci := buildConsentInfo(emb) // แหล่งข้อมูลกลางเดียวกับ GUI

	// สร้าง disclosure text ด้วย strings.Builder เพื่อประสิทธิภาพ
	var b strings.Builder
	// แสดง header
	fmt.Fprintln(&b, "============================================================")
	fmt.Fprintf(&b, "  %s — ตัวติดตั้ง (Setup)\n", ci.AppName)
	fmt.Fprintln(&b, "============================================================")
	fmt.Fprintln(&b)
	// อธิบายว่า SoftSentry คืออะไรและทำอะไร แล้วตามด้วยสิ่งที่จะเกิดขึ้น
	fmt.Fprintln(&b, ci.Purpose)
	fmt.Fprintln(&b, "ก่อนกดยินยอม นี่คือสิ่งที่ตัวติดตั้งจะทำบนเครื่องนี้:")
	fmt.Fprintln(&b)
	// แสดงรายละเอียดที่จะเกิดขึ้น — ต้องครบถ้วนเพื่อ informed consent
	fmt.Fprintf(&b, "  ติดตั้งที่ : %s\n", ci.InstallDir) // ติดตั้งที่ไหน
	fmt.Fprintf(&b, "  ทำงานเป็น : %s\n", ci.RunsAs)      // รันแบบไหน
	fmt.Fprintf(&b, "  สิทธิ์ที่ใช้ : %s\n", ci.Permission) // ต้องใช้สิทธิ์อะไร
	fmt.Fprintf(&b, "  รายงานไป : %s\n", ci.ServerURL)    // รายงานไปที่ server ไหน
	fmt.Fprintln(&b)
	// แสดงข้อมูลที่จะส่งไปยัง server
	fmt.Fprintln(&b, "ส่งให้เซิร์ฟเวอร์เป็นระยะ:")
	for _, d := range ci.DataCollected {
		fmt.Fprintf(&b, "  - %s\n", d)
	}
	fmt.Fprintln(&b)
	// แสดงอย่างชัดเจนว่าข้อมูลใดที่ "ไม่" ถูกเก็บ — สำคัญมากสำหรับ consent
	fmt.Fprintln(&b, "ไม่เก็บ:")
	for _, d := range ci.DataNotKept {
		fmt.Fprintf(&b, "  - %s\n", d)
	}
	fmt.Fprintln(&b)
	// บอกวิธีถอดถอน — ผู้ใช้ต้องรู้ว่าถอดได้
	fmt.Fprintln(&b, ci.UninstallHint)
	return b.String()
}

// confirmInstall renders the disclosure and asks for an explicit yes. Consent
// must be affirmative: only "y"/"yes" (any case, surrounding space ignored)
// proceeds; a bare Enter or anything else cancels. Reading from an io.Reader /
// io.Writer keeps it testable.
//
// (TH) แสดงข้อมูลเปิดเผยและถามขอคำตอบ "ใช่" อย่างชัดเจน การยินยอมต้องเป็นคำตอบ
// เชิงบวกเท่านั้น: มีเพียง "y"/"yes" (ตัวพิมพ์ใหญ่เล็กไม่สำคัญ ตัดช่องว่างรอบๆ
// ออก) เท่านั้นที่ดำเนินการต่อ ส่วนการกด Enter เฉยๆ หรืออย่างอื่นจะยกเลิก การอ่าน
// ผ่าน io.Reader / io.Writer ทำให้ทดสอบได้ง่าย
//
// Parameters:
//   - w: writer สำหรับแสดง disclosure และ prompt (ใช้ os.Stdout หรือ test buffer)
//   - r: reader สำหรับอ่าน input ของผู้ใช้ (ใช้ os.Stdin หรือ test string reader)
//   - emb: embedded config ที่มีข้อมูลสำหรับแสดงใน disclosure
func confirmInstall(w io.Writer, r io.Reader, emb installer.Embedded) bool {
	// แสดง disclosure text ทั้งหมด
	fmt.Fprint(w, installDisclosure(emb))
	fmt.Fprintln(w)
	// แสดงเส้นแบ่ง separator
	fmt.Fprintln(w, "------------------------------------------------------------")
	// แสดง prompt — [y/N] บ่งชี้ว่า default คือ No (ต้องพิมพ์ y อย่างชัดเจน)
	fmt.Fprint(w, "ติดตั้ง SoftSentry Agent บนเครื่องนี้? [y/N]: ")

	// อ่าน input ของผู้ใช้จนถึง newline
	line, _ := bufio.NewReader(r).ReadString('\n')
	// แปลงเป็น lowercase และตัดช่องว่างรอบๆ เพื่อ normalize input
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes": // ตอบรับการติดตั้ง — ทั้ง "y" และ "yes" ยอมรับ (case-insensitive)
		return true
	default: // ทุก input อื่นๆ รวมถึง Enter เฉยๆ — ยกเลิก
		return false
	}
}

// step prints the start of a numbered install step (no newline) so stepDone can
// append "done" on the same line — giving simple, legible progress.
//
// (TH) พิมพ์จุดเริ่มต้นของขั้นตอนติดตั้งที่มีหมายเลขกำกับ (ไม่ขึ้นบรรทัดใหม่)
// เพื่อให้ stepDone ต่อคำว่า "done" ในบรรทัดเดียวกันได้ — ทำให้แสดงความคืบหน้า
// แบบเรียบง่ายและอ่านง่าย เช่น:  "[1/4] Removing any previous version ... done"
//
// Parameters:
//   - w: output writer
//   - n: หมายเลขขั้นตอน (1-4)
//   - label: ชื่อขั้นตอน
func step(w io.Writer, n int, label string) {
	// %-32s จัด align ชื่อขั้นตอนให้กว้าง 32 ตัวอักษร เพื่อให้ "done" อยู่แนวเดียวกัน
	fmt.Fprintf(w, "  [%d/4] %-32s", n, label+" ...")
}

// stepDone พิมพ์ "done" ต่อท้ายบรรทัด step ที่เปิดไว้ พร้อมขึ้นบรรทัดใหม่
func stepDone(w io.Writer) {
	fmt.Fprintln(w, "done")
}

// replaceBinary installs the new agent binary at dst even when a previous one
// is already there — possibly still locked by a running process. Windows refuses
// to overwrite or delete a running .exe, but it *does* allow renaming one, so we
// move any existing binary aside (`*.old-<ts>`) before writing the new one. This
// makes a re-run / in-place upgrade succeed in one click with no manual "stop
// the service" step. The displaced file is removed best-effort: a delete fails
// harmlessly if a live process still holds it, and the next install sweeps it.
//
// (TH) ติดตั้งไบนารี agent ตัวใหม่ที่ dst แม้ว่าจะมีตัวเก่าอยู่แล้ว — ซึ่งอาจยัง
// ถูกล็อกโดยโพรเซสที่กำลังรันอยู่ Windows ไม่ยอมให้เขียนทับหรือลบไฟล์ .exe ที่
// กำลังรัน แต่ *ยอมให้* เปลี่ยนชื่อได้ จึงย้ายไบนารีเดิมออกไปก่อน (`*.old-<ts>`)
// แล้วค่อยเขียนตัวใหม่ลงไป วิธีนี้ทำให้การรันซ้ำ / อัปเกรดแบบ in-place สำเร็จได้
// ในคลิกเดียว โดยไม่ต้อง "หยุด service" ด้วยมือ ไฟล์ที่ถูกย้ายออกจะถูกลบแบบ
// best-effort: การลบล้มเหลวอย่างไม่มีผลเสียถ้ายังมีโพรเซสที่ยังถืออยู่ และการ
// ติดตั้งครั้งถัดไปจะกวาดมันทิ้งให้
func replaceBinary(src, dst string, originalLen int64) error {
	// ลบไฟล์ *.old-* ที่หลงเหลือจากการอัปเกรดก่อนหน้า (best-effort cleanup)
	sweepStaleBinaries(dst)

	// ตรวจสอบว่ามีไบนารีเก่าอยู่แล้วหรือไม่
	if _, err := os.Stat(dst); err == nil {
		// มีไบนารีเก่าอยู่ — สร้างชื่อ backup โดยใช้ UnixNano timestamp เพื่อความเป็นเอกลักษณ์
		aside := fmt.Sprintf("%s.old-%d", dst, time.Now().UnixNano())
		// ย้ายไบนารีเก่าออกไปก่อน เพื่อปลดล็อก (Windows ยอมให้ rename ไฟล์ที่รันอยู่ได้)
		if err := os.Rename(dst, aside); err != nil {
			// Could not move it aside (rare) — fall back to overwriting in place,
			// retrying to absorb the brief delay before a just-stopped service
			// releases its file handle.
			//
			// (TH) ย้ายออกไม่ได้ (เกิดขึ้นไม่บ่อย) — ใช้วิธีเขียนทับในที่เดิมแทน
			// พร้อม retry เพื่อรอช่วงเวลาสั้นๆ ก่อนที่ service ที่เพิ่งหยุดจะ
			// ปล่อย file handle — กรณีนี้คือ fallback สุดท้าย
			return copyAgentWithRetry(src, dst, originalLen)
		}
		// ลบไฟล์ backup เมื่อฟังก์ชันกลับ (best-effort — ไม่ error ถ้าลบไม่ได้)
		defer func() { _ = os.Remove(aside) }()
	}
	// คัดลอกไบนารีใหม่ไปยัง dst โดยตัด trailer ออก (ติดตั้งเฉพาะส่วน agent จริงๆ)
	return installer.CopyStripped(src, dst, originalLen)
}

// sweepStaleBinaries best-effort removes `*.old-*` binaries left behind by a
// previous upgrade when the displaced file was still locked at the time. By the
// next install the old process is long gone, so the delete now succeeds and
// keeps the install directory tidy.
//
// (TH) ลบไบนารี `*.old-*` ที่หลงเหลือจากการอัปเกรดครั้งก่อนแบบ best-effort
// (ตอนนั้นไฟล์ที่ถูกย้ายออกยังถูกล็อกอยู่) พอถึงตอนติดตั้งครั้งถัดไป โพรเซสเก่า
// หายไปนานแล้ว การลบจึงสำเร็จและช่วยให้โฟลเดอร์ติดตั้งสะอาดเป็นระเบียบ
func sweepStaleBinaries(dst string) {
	// หาไฟล์ทั้งหมดที่ตรงกับ pattern "*.old-*" ในโฟลเดอร์เดียวกับ dst
	matches, err := filepath.Glob(dst + ".old-*")
	if err != nil {
		// Glob error (ไม่ปกติ) — ไม่ทำอะไร เพราะนี่เป็น best-effort cleanup
		return
	}
	// ลบทุกไฟล์ที่พบ (ผลการลบล้มเหลวจะถูกละเว้น)
	for _, m := range matches {
		_ = os.Remove(m) // ละเว้น error — ไฟล์อาจยังถูกล็อกอยู่โดยโพรเซสที่รันอยู่
	}
}

// copyAgentWithRetry writes the agent binary into place, retrying briefly on
// failure. Even after the old service reports Stopped, Windows can take a moment
// to release the lock on the previous .exe, so the first copy may still hit a
// sharing violation; a few short retries clear it without bothering the user.
//
// (TH) เขียนไบนารี agent ไปวางในตำแหน่งที่ต้องการ พร้อม retry สั้นๆ เมื่อล้มเหลว
// แม้ service เก่าจะรายงานว่า Stopped แล้ว Windows ก็อาจใช้เวลาสักครู่กว่าจะปล่อย
// ล็อกของ .exe เดิม ทำให้การคัดลอกครั้งแรกอาจเจอ sharing violation การ retry
// สั้นๆ ไม่กี่ครั้งจะแก้ปัญหานี้ได้โดยไม่ต้องรบกวนผู้ใช้
func copyAgentWithRetry(src, dst string, originalLen int64) error {
	const attempts = 10 // จำนวนครั้งสูงสุดที่จะ retry (รวมครั้งแรกด้วย)
	var err error
	// ลองคัดลอก attempts ครั้ง โดย sleep 500ms ระหว่างแต่ละครั้ง
	for range attempts {
		if err = installer.CopyStripped(src, dst, originalLen); err == nil {
			return nil // คัดลอกสำเร็จ — ออกทันที
		}
		// คัดลอกล้มเหลว — รอ 500ms ก่อน retry เพื่อให้ Windows ปล่อย file lock
		time.Sleep(500 * time.Millisecond)
	}
	// ล้มเหลวทุกครั้ง — คืน error ครั้งสุดท้าย
	return err
}

// installDir returns the per-OS directory the agent is installed into.
// (TH) คืนค่าไดเรกทอรีที่ใช้ติดตั้ง agent ของแต่ละ OS:
// - Windows: %ProgramFiles%\SoftSentry (default: C:\Program Files\SoftSentry)
// - macOS/Linux: /usr/local/softsentry
func installDir() (string, error) {
	// เลือก directory ตาม OS ปัจจุบัน
	if runtime.GOOS == "windows" {
		// อ่าน %ProgramFiles% env var สำหรับ path ที่ถูกต้องในทุก locale Windows
		base := os.Getenv("ProgramFiles")
		if base == "" {
			// fallback ถ้า env var ไม่มีค่า (ไม่ควรเกิดขึ้นบน Windows ปกติ)
			base = `C:\Program Files`
		}
		return filepath.Join(base, "SoftSentry"), nil
	}
	// macOS และ Linux ใช้ /usr/local/softsentry
	return "/usr/local/softsentry", nil
}

// exeName คืนชื่อไฟล์ executable ของ agent ตาม OS
// บน Windows มีนามสกุล .exe ตามมาตรฐาน Windows
func exeName() string {
	if runtime.GOOS == "windows" {
		return "softsentry-agent.exe" // Windows ต้องมีนามสกุล .exe
	}
	return "softsentry-agent" // macOS/Linux ไม่มีนามสกุล
}

// closeWithCountdown auto-closes the installer window after a short, visible
// countdown — so a successful one-click install needs zero interaction and the
// window never looks "hung" waiting for an Enter that the user must guess at.
// Pressing Enter at any point closes it immediately.
//
// (TH) ปิดหน้าต่างตัวติดตั้งให้อัตโนมัติหลังจากนับถอยหลังสั้นๆ ที่มองเห็นได้ —
// เพื่อให้การติดตั้งแบบคลิกเดียวที่สำเร็จไม่ต้องมีการโต้ตอบใดๆ เลย และหน้าต่าง
// จะไม่ดูเหมือน "ค้าง" รอการกด Enter ที่ผู้ใช้ต้องเดาเอง การกด Enter เมื่อไหร่ก็ได้
// จะปิดหน้าต่างทันที
func closeWithCountdown(w io.Writer, seconds int) {
	// Let an Enter keypress short-circuit the wait without blocking the countdown.
	// (TH) ให้การกด Enter ตัดจบการรอได้ทันทีโดยไม่บล็อกการนับถอยหลัง
	// ใช้ channel แบบ buffered size 1 เพื่อรับสัญญาณจาก goroutine อ่าน stdin
	enter := make(chan struct{}, 1)
	// รัน goroutine แยกเพื่ออ่าน stdin โดยไม่บล็อก main goroutine
	go func() {
		// รอผู้ใช้กด Enter (ReadString ค้างรอจนกว่าจะได้รับ newline)
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		// ส่งสัญญาณเข้า channel (non-blocking: ถ้า channel เต็มแล้ว ให้ข้ามไป)
		select {
		case enter <- struct{}{}:
		default:
		}
	}()
	// นับถอยหลังจาก seconds ถึง 1
	for s := seconds; s > 0; s-- {
		// แสดงนาฬิกานับถอยหลัง — \r ทำให้เขียนทับบรรทัดเดิม (ไม่เพิ่มบรรทัดใหม่)
		fmt.Fprintf(w, "\r  This window closes in %2ds (or press Enter) ...", s)
		select {
		case <-enter:
			// ผู้ใช้กด Enter — ปิดทันที
			fmt.Fprintln(w)
			return
		case <-time.After(time.Second):
			// รอ 1 วินาที แล้วนับถอยต่อ
		}
	}
	// นับถอยหลังครบแล้ว — ขึ้นบรรทัดใหม่และปิด
	fmt.Fprintln(w)
}

// waitForExit keeps the console window open after a *failed* install so the user
// can read the error, then waits for Enter. (Success uses closeWithCountdown.)
//
// (TH) เปิดหน้าต่างคอนโซลค้างไว้หลังการติดตั้ง *ล้มเหลว* เพื่อให้ผู้ใช้อ่าน
// ข้อความ error ได้ แล้วรอการกด Enter (กรณีสำเร็จใช้ closeWithCountdown แทน)
// ฟังก์ชันนี้บล็อกจนกว่าผู้ใช้จะกด Enter — ไม่มี timeout
func waitForExit(w io.Writer) {
	// แสดง prompt ขอให้กด Enter เพื่อปิดหน้าต่าง
	fmt.Fprint(w, "\nPress Enter to close ...")
	// รอผู้ใช้กด Enter (บล็อกจนกว่าจะได้รับ newline)
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
