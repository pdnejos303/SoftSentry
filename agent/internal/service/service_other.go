//go:build !windows && !darwin
// ไฟล์นี้ compile เฉพาะบน platform ที่ไม่ใช่ Windows และไม่ใช่ macOS (เช่น Linux)
// เพื่อให้ package สามารถ build ได้บนทุก platform แต่ฟังก์ชันทั้งหมดจะคืน ErrUnsupported

// Package service ให้ stub implementation สำหรับ platform ที่ไม่รองรับ
package service

import "github.com/softsentry/agent/internal/runner" // ใช้ runner.Options สำหรับ signature ของ RunIfService

// Install ไม่รองรับบน platform อื่นนอกจาก Windows และ macOS
// คืน ErrUnsupported เสมอเพื่อแจ้งว่า platform นี้ไม่รองรับ
func Install(Config) error { return ErrUnsupported }

// Uninstall ไม่รองรับบน platform อื่นนอกจาก Windows และ macOS
// คืน ErrUnsupported เสมอเพื่อแจ้งว่า platform นี้ไม่รองรับ
func Uninstall() error { return ErrUnsupported }

// Status ไม่รองรับบน platform อื่นนอกจาก Windows และ macOS
// คืน ErrUnsupported เสมอเพื่อแจ้งว่า platform นี้ไม่รองรับ
func Status() (string, error) { return "", ErrUnsupported }

// RunIfService ไม่เคยรันภายใต้ service manager บน platform เหล่านี้
// คืน (false, nil) เสมอเพื่อให้ caller ใช้ loop ปกติแทน
func RunIfService(runner.Options) (bool, error) { return false, nil }
