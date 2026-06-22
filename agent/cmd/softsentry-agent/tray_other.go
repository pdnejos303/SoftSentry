//go:build !windows

// บนแพลตฟอร์มที่ไม่ใช่ Windows ยังไม่มี tray GUI (NotifyIcon ผ่าน lxn/walk เป็น
// Windows-only) จึงไม่ลงทะเบียนคำสั่ง tray — คืน nil ให้ root ข้ามไป
// (macOS menu-bar เป็นงาน follow-up ตาม design doc)
package main

import "github.com/spf13/cobra"

// trayCmd คืน nil บน non-Windows — root จะไม่ลงทะเบียนคำสั่งนี้
func trayCmd() *cobra.Command { return nil }
