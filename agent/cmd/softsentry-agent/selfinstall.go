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
	out := os.Stdout // ใช้ stdout สำหรับแสดงผลทั้งหมดในหน้าต่าง console

	// ถ้ายังไม่ได้รับ consent (instance แรกที่ไม่ใช่ elevated continuation)
	if !preConsented {
		// Informed consent up front, before requesting admin or touching disk.
		// (TH) ขอความยินยอมแบบรับทราบข้อมูลก่อนเริ่ม ก่อนจะขอสิทธิ์ผู้ดูแลระบบ
		// หรือแตะต้องดิสก์ — หลักการ privacy by design
		if !confirmInstall(out, os.Stdin, emb) {
			// ผู้ใช้ปฏิเสธการติดตั้ง — ไม่มีการเปลี่ยนแปลงใดๆ เกิดขึ้น
			fmt.Fprintln(out)
			fmt.Fprintln(out, "  Installation cancelled — nothing was installed or changed.")
			closeWithCountdown(out, 5) // รอ 5 วินาทีแล้วปิดหน้าต่างอัตโนมัติ
			return nil
		}
		// ตรวจสอบว่ากำลังรันด้วยสิทธิ์ admin อยู่แล้วหรือไม่
		if !installer.IsElevated() {
			// ยังไม่มีสิทธิ์ admin — ต้อง relaunch ผ่าน UAC (Windows)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Requesting administrator permission ...")
			// Relaunch ตัวเองพร้อม elevatedInstallFlag เพื่อบอกว่า consent ผ่านแล้ว
			if err := installer.RelaunchElevated(exe, []string{elevatedInstallFlag}); err != nil {
				// ขอสิทธิ์ admin ล้มเหลว (ผู้ใช้คลิก "No" ใน UAC หรือ error อื่น)
				return fmt.Errorf("could not get administrator permission: %w", err)
			}
			// The elevated instance continues in its own window; this one is done.
			// (TH) instance ที่ elevated แล้วจะทำงานต่อในหน้าต่างของมันเอง
			// ส่วนตัวนี้จบหน้าที่แล้ว — return nil เพื่อปิดหน้าต่างแรกอย่างสะอาด
			return nil
		}
	}

	// แสดง header การติดตั้ง — ถึงขั้นตอนนี้หมายความว่ามีสิทธิ์ admin แล้ว
	fmt.Fprintln(out, "============================================")
	fmt.Fprintln(out, "  SoftSentry Agent — installing")
	fmt.Fprintf(out, "  Server: %s\n", emb.Config.ServerURL) // แสดง server ที่จะรายงานไป
	fmt.Fprintln(out, "============================================")
	fmt.Fprintln(out)

	// หา install directory ตาม OS (Program Files บน Windows, /usr/local/softsentry บน macOS)
	installDir, err := installDir()
	if err != nil {
		// หา install directory ล้มเหลว
		return err
	}
	// สร้าง install directory ถ้ายังไม่มี (MkdirAll ไม่ error ถ้ามีอยู่แล้ว)
	if err := os.MkdirAll(installDir, 0o755); err != nil { // #nosec G301 — program dir
		// สร้าง directory ล้มเหลว (เช่น ไม่มีสิทธิ์เขียน)
		return fmt.Errorf("create install dir: %w", err)
	}
	// กำหนด path เต็มของ agent binary ที่จะติดตั้ง
	dstExe := filepath.Join(installDir, exeName()) // เช่น C:\Program Files\SoftSentry\softsentry-agent.exe

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
	// ตรวจสอบว่า service เดิมมีอยู่และ status ไม่ใช่ "not installed"
	if st, err := service.Status(); err == nil && st != "not installed" {
		// หยุดและลบ service เดิม เพื่อปลดล็อกไบนารีที่กำลังรัน
		if err := service.Uninstall(); err != nil {
			// ลบ service เก่าล้มเหลว — แจ้งเตือนแต่ดำเนินการต่อ (best-effort)
			fmt.Fprintf(out, "      (old service not fully removed: %v; continuing)\n", err)
		}
	}
	stepDone(out) // แสดง "done" ต่อท้ายบรรทัด step นี้

	// Step 2 — copy the agent into Program Files.
	// (TH) ขั้นที่ 2 — คัดลอก agent ไปไว้ใน Program Files
	step(out, 2, "Installing files")
	// คัดลอกไบนารีจาก src (Setup.exe ปัจจุบัน) ไปที่ dst (Program Files)
	// emb.OriginalLen บอกว่าไบนารีจริงขนาดเท่าไหร่ (ตัด trailer ออก)
	if err := replaceBinary(exe, dstExe, emb.OriginalLen); err != nil {
		// คัดลอกไบนารีล้มเหลว
		return fmt.Errorf("copy agent into place: %w", err)
	}
	stepDone(out)

	// Step 3 — enroll this machine with the embedded deployment token.
	// (TH) ขั้นที่ 3 — enroll เครื่องนี้ด้วย deployment token ที่ฝังมา
	step(out, 3, "Enrolling this machine")
	// ใช้ io.Discard เป็น output writer เพื่อซ่อน progress ของ enrollMachine
	// เพราะเราจัดการ progress display เองผ่าน step/stepDone แล้ว
	if err := enrollMachine(ctx, io.Discard, emb.Config.Token, emb.Config.ServerURL); err != nil {
		// enroll ล้มเหลว (เช่น token หมดอายุ หรือ server ไม่ตอบสนอง)
		return fmt.Errorf("enroll: %w", err)
	}
	stepDone(out)

	// Step 4 — register + start the background service.
	// (TH) ขั้นที่ 4 — ลงทะเบียน + เริ่ม background service
	step(out, 4, "Starting background service")
	// ติดตั้ง service โดยใช้ path ของไบนารีที่คัดลอกไปแล้ว และ argument "run"
	if err := service.Install(service.Config{ExePath: dstExe, Args: []string{"run"}}); err != nil {
		// ติดตั้ง service ล้มเหลว
		return fmt.Errorf("install service: %w", err)
	}
	stepDone(out)

	// แสดงข้อความสำเร็จและรอให้หน้าต่างปิดเองหลัง 8 วินาที
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  ✓ Done! SoftSentry Agent is installed and running.\n")
	fmt.Fprintln(out, "    This machine will appear in the dashboard within a minute.")
	closeWithCountdown(out, 8) // รอ 8 วินาทีแล้วปิดหน้าต่างอัตโนมัติ
	return nil
}

// installDisclosure returns the plain-language consent screen shown before any
// change is made: what gets installed, where, the permission required, the
// server it reports to, and exactly what data is (and is not) collected. Kept a
// pure function so it is unit-testable. The deployment token is intentionally
// never shown — it is a secret.
//
// (TH) คืนค่าหน้าจอขอความยินยอมแบบภาษาธรรมดา ที่แสดงก่อนจะมีการเปลี่ยนแปลงใดๆ:
// จะติดตั้งอะไร ที่ไหน ต้องใช้สิทธิ์อะไร รายงานไปยังเซิร์ฟเวอร์ไหน และข้อมูล
// อะไรบ้างที่ถูก (และไม่ถูก) เก็บ ทำเป็น pure function เพื่อให้ unit-test ได้
// ส่วน deployment token จะไม่ถูกแสดงโดยเจตนา — เพราะเป็นความลับ
func installDisclosure(emb installer.Embedded) string {
	// หา install directory เพื่อแสดงให้ผู้ใช้ทราบว่าจะติดตั้งที่ไหน
	dir, err := installDir()
	// ถ้าหา directory ล้มเหลวหรือว่าง ใช้คำอธิบายทั่วไปแทน
	if err != nil || dir == "" {
		dir = "the SoftSentry program folder"
	}

	// สร้าง disclosure text ด้วย strings.Builder เพื่อประสิทธิภาพ
	var b strings.Builder
	// แสดง header
	fmt.Fprintln(&b, "============================================================")
	fmt.Fprintln(&b, "  SoftSentry Agent — Setup")
	fmt.Fprintln(&b, "============================================================")
	fmt.Fprintln(&b)
	// อธิบายว่า SoftSentry คืออะไรและทำอะไร
	fmt.Fprintln(&b, "SoftSentry lets your IT / security team keep an inventory of")
	fmt.Fprintln(&b, "the software on company computers and flag known security")
	fmt.Fprintln(&b, "vulnerabilities. Before you agree, here is exactly what this")
	fmt.Fprintln(&b, "setup will do on THIS computer:")
	fmt.Fprintln(&b)
	// แสดงรายละเอียดที่จะเกิดขึ้น — ต้องครบถ้วนเพื่อ informed consent
	fmt.Fprintf(&b, "  Install to : %s\n", dir)                            // ติดตั้งที่ไหน
	fmt.Fprintln(&b, "  Runs as    : a background service that starts automatically") // รันแบบไหน
	fmt.Fprintln(&b, "               when the computer boots")
	fmt.Fprintln(&b, "  Permission : Administrator (to register the service) —") // ต้องใช้สิทธิ์อะไร
	fmt.Fprintln(&b, "               Windows will prompt you right after this")
	fmt.Fprintf(&b, "  Reports to : %s\n", emb.Config.ServerURL)           // รายงานไปที่ server ไหน
	fmt.Fprintln(&b)
	// แสดงข้อมูลที่จะส่งไปยัง server
	fmt.Fprintln(&b, "Sent to that server, on a schedule:")
	fmt.Fprintln(&b, "  - List of installed software (name, version, publisher)") // รายการซอฟต์แวร์
	fmt.Fprintln(&b, "  - Digital-signature status of program files")              // สถานะ digital signature
	fmt.Fprintln(&b, "  - Basic machine info (hostname, OS version, architecture)") // ข้อมูลเครื่องพื้นฐาน
	fmt.Fprintln(&b)
	// แสดงอย่างชัดเจนว่าข้อมูลใดที่ "ไม่" ถูกเก็บ — สำคัญมากสำหรับ consent
	fmt.Fprintln(&b, "NOT collected: your personal files, documents, photos,")
	fmt.Fprintln(&b, "emails, keystrokes, screen contents, or web browsing.")
	fmt.Fprintln(&b)
	// บอกวิธีถอดถอน — ผู้ใช้ต้องรู้ว่าถอดได้
	fmt.Fprintln(&b, "You can remove it any time from \"Apps & features\", or run:")
	fmt.Fprintln(&b, "  softsentry-agent uninstall")
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
	fmt.Fprint(w, "Install SoftSentry Agent on this computer? [y/N]: ")

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
