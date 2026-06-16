//go:build !windows

// บนแพลตฟอร์มที่ไม่ใช่ Windows ไม่มี wizard GUI (RunWizard อยู่ใน wizard_windows.go)
// จึงไม่ลงทะเบียนคำสั่ง preview-wizard — คืน nil ให้ root ข้ามไป
package main

import "github.com/spf13/cobra"

// previewWizardCmd คืน nil บน non-Windows — root จะไม่ลงทะเบียนคำสั่งนี้
func previewWizardCmd() *cobra.Command { return nil }
