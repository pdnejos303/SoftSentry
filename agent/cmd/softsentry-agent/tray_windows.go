//go:build windows

// ไฟล์นี้คือ subcommand `tray` — ตัวแสดงไอคอนใน system tray (user session) ที่
// อ่าน progress.json ที่ service (LocalSystem) เขียนไว้ แล้วโชว์ความคืบหน้าการสแกน
// ให้ end-user เห็น เป็น "โหมดที่สอง" ของ binary เดียวกัน (อีกโหมดคือ service ที่สแกน)
// คุยกันผ่าน status file เพราะ Session 0 Isolation ห้าม service แสดง UI เอง
package main

import (
	"time"

	"github.com/lxn/walk"
	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/installer"
	"github.com/softsentry/agent/internal/scanner"
	"github.com/softsentry/agent/internal/storage"
)

// trayStaleAfter: ถ้า progress.json ไม่ถูกอัปเดตนานเกินนี้ทั้งที่ phase ยัง active
// ให้ถือว่า service หยุด/ค้าง → tray แสดงสถานะ idle แทนที่จะค้างเลขเดิมตลอดไป
const trayStaleAfter = 30 * time.Second

// trayPollInterval: ความถี่ที่ tray อ่าน status file (เบามาก เป็นแค่อ่านไฟล์เล็ก)
const trayPollInterval = time.Second

// trayCmd คืน subcommand `tray` (เฉพาะ build Windows) — รันใน user session ตอน login
func trayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tray",
		Short: "Show a system-tray icon with live scan progress (runs in the user session)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTray(trayTextFor(installer.DefaultLang()))
		},
	}
}

// runTray สร้างหน้าต่างซ่อน + NotifyIcon แล้ว poll progress.json ทุก 1 วินาที
// อัปเดต tooltip และเด้ง balloon เมื่อสแกนเสร็จ (active → idle)
func runTray(txt trayText) error {
	// หน้าต่างหลักแบบซ่อน — จำเป็นต้องมี form ให้ NotifyIcon ผูก และให้ message loop รัน
	mw, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	defer mw.Dispose()
	mw.SetVisible(false) // tray-only: ไม่โชว์หน้าต่าง

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		return err
	}
	defer func() { _ = ni.Dispose() }()
	_ = ni.SetIcon(walk.IconShield()) // ใช้ stock icon เพื่อไม่ต้องผูก resource
	_ = ni.SetToolTip(txt.appName)
	if err := ni.SetVisible(true); err != nil {
		return err
	}

	// เมนูคลิกขวา: ออกจาก tray (ไม่กระทบ service ที่สแกนอยู่เบื้องหลัง)
	quit := walk.NewAction()
	_ = quit.SetText(txt.quit)
	quit.Triggered().Attach(func() { walk.App().Exit(0) })
	_ = ni.ContextMenu().Actions().Add(quit)

	// goroutine อ่าน status file แล้วสั่งอัปเดต UI ผ่าน mw.Synchronize (thread-safe)
	go func() {
		ticker := time.NewTicker(trayPollInterval)
		defer ticker.Stop()
		lastActive := false
		for range ticker.C {
			p, _ := storage.ReadProgress() // best-effort: อ่านไม่ได้ → ถือว่า idle
			now := time.Now()
			active := trayActive(p, now)
			tip := trayTooltip(p, now, txt)
			showDone := lastActive && !active // เพิ่งสแกนเสร็จ → เด้งแจ้งเตือน
			lastActive = active
			mw.Synchronize(func() {
				_ = ni.SetToolTip(tip)
				if showDone {
					_ = ni.ShowInfo(txt.doneTitle, txt.doneBody)
				}
			})
		}
	}()

	mw.Run() // message loop — คืนเมื่อ quit.Triggered เรียก App().Exit
	return nil
}

// trayActive บอกว่ามีการสแกนกำลังดำเนินอยู่จริงไหม: phase ต้องไม่ใช่ idle/ว่าง และ
// status file ต้องไม่เก่าเกิน trayStaleAfter (กัน tray ค้างเมื่อ service ตายกลางคัน)
func trayActive(p scanner.Progress, now time.Time) bool {
	ph := scanner.Phase(p.Phase)
	if ph == "" || ph == scanner.PhaseIdle {
		return false
	}
	if !p.UpdatedAt.IsZero() && now.Sub(p.UpdatedAt) > trayStaleAfter {
		return false // ข้อมูลเก่าเกินไป — น่าจะ service หยุดแล้ว
	}
	return true
}

// trayTooltip สร้างข้อความ tooltip ตาม phase ปัจจุบัน (แสดง % เฉพาะ phase scanning
// ที่รู้ Total แล้ว) — เป็น pure function เพื่อให้ทดสอบได้โดยไม่ต้องมี GUI
func trayTooltip(p scanner.Progress, now time.Time, txt trayText) string {
	if !trayActive(p, now) {
		return txt.appName + " — " + txt.idle
	}
	switch scanner.Phase(p.Phase) {
	case scanner.PhaseCounting:
		return txt.appName + " — " + txt.counting
	case scanner.PhaseScanning:
		return txt.appName + " — " + txt.scanning + " " + percentText(p.Done, p.Total)
	case scanner.PhaseVerifying:
		return txt.appName + " — " + txt.verifying
	case scanner.PhaseUploading:
		return txt.appName + " — " + txt.uploading
	default:
		return txt.appName
	}
}

// percentText คืน "NN%" จาก done/total โดย clamp 0..100; total<=0 → "…" (ยังไม่รู้)
func percentText(done, total int) string {
	if total <= 0 {
		return "…"
	}
	pct := done * 100 / total
	pct = min(pct, 100)
	if pct < 0 {
		pct = 0
	}
	return itoa(pct) + "%"
}

// itoa เล็กๆ เลี่ยง import strconv เพื่อ tooltip สั้นๆ (ค่าอยู่ในช่วง 0..100)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// trayText เก็บสตริงที่ผู้ใช้เห็น ตามภาษาที่เลือกตอนติดตั้ง (ไทย/EN/JA)
type trayText struct {
	appName   string
	idle      string
	counting  string
	scanning  string
	verifying string
	uploading string
	doneTitle string
	doneBody  string
	quit      string
}

// trayTextFor เลือกชุดข้อความตามภาษา — ใช้ภาษา UI ของ Windows เป็น default
// (installer.DefaultLang) เพราะ config ไม่ได้เก็บภาษาที่ผู้ใช้เลือกไว้
func trayTextFor(lang installer.Lang) trayText {
	switch lang {
	case installer.LangEN:
		return trayText{
			appName: "SoftSentry", idle: "no scan running",
			counting: "counting files", scanning: "scanning",
			verifying: "verifying signatures", uploading: "uploading",
			doneTitle: "SoftSentry", doneBody: "Security scan complete", quit: "Quit",
		}
	case installer.LangJA:
		return trayText{
			appName: "SoftSentry", idle: "スキャンなし",
			counting: "ファイルを集計中", scanning: "スキャン中",
			verifying: "署名を検証中", uploading: "アップロード中",
			doneTitle: "SoftSentry", doneBody: "セキュリティスキャンが完了しました", quit: "終了",
		}
	default: // LangTH
		return trayText{
			appName: "SoftSentry", idle: "ไม่มีการสแกน",
			counting: "กำลังนับไฟล์", scanning: "กำลังสแกน",
			verifying: "กำลังตรวจลายเซ็น", uploading: "กำลังอัปโหลด",
			doneTitle: "SoftSentry", doneBody: "ตรวจสอบความปลอดภัยเสร็จแล้ว", quit: "ออก",
		}
	}
}
