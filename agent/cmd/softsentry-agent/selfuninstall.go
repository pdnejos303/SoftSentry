// ไฟล์นี้จัดการเส้นทาง "ผู้ใช้กด Uninstall ใน Settings → Apps" (Windows เรียก
// UninstallString = `softsentry-agent.exe --uninstall`) เป็นภาพสะท้อนของ
// maybeSelfInstall: ยืนยัน → ขอสิทธิ์ admin ผ่าน UAC → ลบทุกอย่างด้วย removeAgent
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/softsentry/agent/internal/installer"
)

const (
	uninstallFlag       = "--uninstall"        // จุดเริ่ม GUI uninstall (เห็นได้จากผู้ใช้/ARP)
	acceptUninstallFlag = "--accept-uninstall" // marker หลังผ่าน UAC (ยืนยันแล้ว ไม่ต้องถามซ้ำ)
	silentFlag          = "--silent"           // ถอนแบบเงียบ (QuietUninstallString / MDM)
)

// maybeSelfUninstall คืน handled=true เมื่อเป็นการถอนผ่าน --uninstall มิฉะนั้น false
// เพื่อให้ flow เดิม (self-install / Cobra) ทำงานต่อ
func maybeSelfUninstall(ctx context.Context) (handled bool, err error) {
	args := os.Args[1:]
	if !hasFlag(args, uninstallFlag) {
		return false, nil
	}
	preConsented := hasFlag(args, acceptUninstallFlag)
	silent := hasFlag(args, silentFlag)
	lang := installer.ParseLang(langFromArgs(args))
	return true, runSelfUninstall(ctx, preConsented, silent, lang)
}

// langFromArgs ดึงค่า --lang= จาก args (ใช้ default UI language ถ้าไม่ระบุ)
// ใช้ strings.CutPrefix (Go 1.20+) แทน helper แบบ hand-rolled
func langFromArgs(args []string) string {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--lang="); ok {
			return v
		}
	}
	return string(installer.DefaultLang())
}

// runSelfUninstall ทำขั้นตอนถอนจริง — ขอ consent (ถ้ายัง) → elevate → removeAgent
func runSelfUninstall(ctx context.Context, preConsented, silent bool, lang installer.Lang) error {
	_ = ctx
	installer.HideConsoleWindow()

	exe, err := resolveSelfPath()
	if err != nil {
		return err
	}

	if !preConsented {
		// ขอ consent (เว้นแต่ silent) ก่อนแตะต้องอะไร
		if !silent {
			proceed, shown := installer.RunUninstallConfirm(lang)
			if shown {
				if !proceed {
					return nil // ผู้ใช้ยกเลิก — ไม่มีการเปลี่ยนแปลง
				}
			} else {
				// non-Windows → console prompt
				installer.ShowConsoleWindow()
				if !confirmUninstallConsole(os.Stdout, os.Stdin) {
					return nil
				}
			}
		}
		// ขอสิทธิ์ admin ถ้ายังไม่มี (ลบ service + HKLM ต้องใช้ admin)
		if !installer.IsElevated() {
			relArgs := []string{uninstallFlag, acceptUninstallFlag, "--lang=" + string(lang)}
			if silent {
				relArgs = append(relArgs, silentFlag)
			}
			if err := installer.RelaunchElevated(exe, relArgs); err != nil {
				return showUninstallError(lang, silent, fmt.Errorf("ขอสิทธิ์ผู้ดูแลระบบไม่สำเร็จ: %w", err))
			}
			return nil // instance ที่ elevated ทำงานต่อ
		}
	}

	// มีสิทธิ์ admin แล้ว — ลบทุกอย่าง (Purge: ผู้ใช้สั่งถอนแบบสะอาด)
	if err := removeAgent(io.Discard, removeOptions{Purge: true}); err != nil {
		return showUninstallError(lang, silent, err)
	}
	showUninstallSuccess(lang, silent)
	return nil
}

// showUninstallSuccess แจ้งผลสำเร็จแบบ GUI หรือ console (silent = ไม่แสดง)
func showUninstallSuccess(lang installer.Lang, silent bool) {
	if silent {
		return
	}
	heading, msg := installer.UninstallResultText(lang)
	if installer.ShowResultGUI(true, heading, msg) {
		return
	}
	out := os.Stdout
	fmt.Fprintf(out, "\n  ✓ %s — %s\n", heading, msg)
	closeWithCountdown(out, 6)
}

// showUninstallError แจ้งผลล้มเหลวแบบ GUI หรือ console แล้วคืน err
func showUninstallError(lang installer.Lang, silent bool, err error) error {
	if silent {
		return err
	}
	heading, msg := installer.ErrorText(lang, err.Error())
	if installer.ShowResultGUI(false, heading, msg) {
		return err
	}
	installer.ShowConsoleWindow()
	fmt.Fprintln(os.Stderr, "error:", err)
	waitForExit(os.Stdout)
	return err
}

// confirmUninstallConsole ถาม y/N บน console (fallback นอก Windows)
func confirmUninstallConsole(w io.Writer, r io.Reader) bool {
	heading, body, _, _ := installer.UninstallConfirmText(installer.DefaultLang())
	fmt.Fprintf(w, "%s\n%s\n[y/N]: ", heading, body)
	var resp string
	_, _ = fmt.Fscanln(r, &resp)
	return resp == "y" || resp == "Y" || resp == "yes"
}
