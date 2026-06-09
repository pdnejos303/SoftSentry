// Package updater ใช้งาน agent self-update (spec 1.6):
// ดาวน์โหลด binary เวอร์ชันใหม่ที่ server เสนอให้, ยืนยัน SHA-256,
// สลับ binary แบบ atomic (เก็บ backup ไว้เป็น .old สำหรับ rollback),
// และปล่อยให้ service manager restart process บน binary ใหม่
package updater

import (
	"context"       // ใช้ส่ง context สำหรับ timeout/cancellation ของ download
	"crypto/sha256" // ใช้คำนวณ checksum SHA-256 ของ binary ที่ดาวน์โหลดมา
	"encoding/hex"  // ใช้แปลง byte array ของ hash เป็น hex string
	"fmt"           // ใช้ฟอร์แมต error message พร้อม context
	"io"            // ใช้ Reader/Writer interface สำหรับ streaming และ hash
	"os"            // ใช้สร้าง/อ่านไฟล์และ rename สำหรับ atomic swap
	"strings"       // ใช้ EqualFold สำหรับ case-insensitive checksum comparison

	"github.com/softsentry/agent/internal/transport" // ใช้ AgentUpdate struct สำหรับข้อมูล update
)

// binaryDownloader เป็น interface ที่ตัด transport.Client เหลือแค่ method ที่ Apply ต้องการ
// ทำให้ test ง่ายขึ้น: ไม่ต้องสร้าง transport.Client จริง สามารถ mock ได้
type binaryDownloader interface {
	// GetBinary ดาวน์โหลด binary จาก path ที่กำหนดและเขียนลงใน w
	GetBinary(ctx context.Context, path string, w io.Writer) error
}

// Apply ดาวน์โหลด update ไปยัง currentPath+".new", ยืนยัน checksum,
// และสลับแทน binary ที่กำลังทำงานแบบ atomic
// หาก step ใดล้มเหลวก่อนการสลับ ไฟล์ download บางส่วนจะถูกลบ และ binary ที่ทำงานอยู่จะไม่ถูกแตะ
// ผู้เรียกควร exit หลังจาก Apply สำเร็จ เพื่อให้ service manager restart บน binary ใหม่
func Apply(
	ctx context.Context,          // context สำหรับ timeout/cancellation ของ download
	client binaryDownloader,      // HTTP client สำหรับดาวน์โหลด binary
	currentPath string,           // path ของ binary ปัจจุบันที่กำลังทำงาน
	up transport.AgentUpdate,     // ข้อมูล update ที่ได้รับจาก server (version, URL, SHA-256)
) error {
	// สร้าง path ชั่วคราวสำหรับไฟล์ที่ดาวน์โหลดมาก่อน verify
	newPath := currentPath + ".new"

	// สร้างไฟล์ .new พร้อม permission executable (0755) สำหรับรับ binary
	f, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // replacing an executable
	if err != nil {
		// คืน error ถ้าสร้างไฟล์ล้มเหลว (เช่น disk เต็ม, permission ขาด)
		return fmt.Errorf("create download file: %w", err)
	}
	// ดาวน์โหลด binary จาก server ไปยังไฟล์ .new
	dlErr := client.GetBinary(ctx, up.DownloadURL, f)
	// ปิดไฟล์เสมอ (ต้องปิดก่อน verify และ rename)
	closeErr := f.Close()
	// ถ้า download ล้มเหลว ลบไฟล์ .new บางส่วนออก
	if dlErr != nil {
		// ลบ .new ที่ download ไม่สมบูรณ์ ไม่ต้องสนใจ error จากการลบ
		_ = os.Remove(newPath)
		// คืน error พร้อม version ที่พยายาม download
		return fmt.Errorf("download %s: %w", up.Version, dlErr)
	}
	// ถ้าปิดไฟล์ล้มเหลว (เช่น flush error) ลบ .new ออก
	if closeErr != nil {
		// ลบ .new ที่อาจ corrupt เพราะ flush ล้มเหลว
		_ = os.Remove(newPath)
		// คืน error สำหรับปัญหาการปิดไฟล์
		return fmt.Errorf("finalize download: %w", closeErr)
	}

	// ยืนยัน SHA-256 ของ binary ที่ดาวน์โหลดมาตรงกับที่ server บอกไว้
	if err := VerifyChecksum(newPath, up.SHA256); err != nil {
		// ลบ .new ที่ checksum ไม่ตรง (อาจถูก tamper หรือ download เสียหาย)
		_ = os.Remove(newPath)
		// คืน error พร้อมระบุ version ที่ verify ล้มเหลว
		return fmt.Errorf("verify %s: %w", up.Version, err)
	}

	// TODO(phase-3 hardening): ตรวจสอบ Authenticode (Windows) / codesign (macOS)
	// signature ของ binary ที่ดาวน์โหลดด้วย per spec 1.6 โดยใช้ signature verifier
	// จาก scanner SHA-256 ปกป้อง integrity แล้ว แต่การตรวจสอบ signature
	// เพิ่ม defense-in-depth กรณี binary store ถูก compromise

	// สลับ binary ใหม่เข้ามาแทนที่ binary เก่าแบบ atomic
	if err := replaceBinary(currentPath, newPath); err != nil {
		// ถ้าสลับล้มเหลว ลบ .new ออก (binary เก่ายังอยู่ที่ currentPath)
		_ = os.Remove(newPath)
		// คืน error พร้อมระบุ version ที่ install ล้มเหลว
		return fmt.Errorf("install %s: %w", up.Version, err)
	}
	return nil
}

// VerifyChecksum คืน nil ถ้า SHA-256 ของไฟล์ที่ path ตรงกับ wantHex
// (เปรียบเทียบแบบ case-insensitive เพื่อรองรับ uppercase/lowercase hex)
func VerifyChecksum(path, wantHex string) error {
	// เปิดไฟล์สำหรับอ่าน (path มาจากภายใน ไม่ใช่ user input จึงปลอดภัย)
	f, err := os.Open(path) //nolint:gosec // path supplied by caller
	if err != nil {
		// คืน error ถ้าเปิดไฟล์ล้มเหลว
		return err
	}
	// ปิดไฟล์เสมอเมื่อ function คืนค่า
	defer f.Close()

	// สร้าง SHA-256 hasher
	h := sha256.New()
	// stream ไฟล์ผ่าน hasher เพื่อคำนวณ checksum โดยไม่โหลดทั้งหมดในหน่วยความจำ
	if _, err := io.Copy(h, f); err != nil {
		// คืน error ถ้าอ่านไฟล์ระหว่าง hashing ล้มเหลว
		return fmt.Errorf("hash file: %w", err)
	}
	// แปลง hash bytes เป็น lowercase hex string
	got := hex.EncodeToString(h.Sum(nil))
	// เปรียบเทียบ checksum แบบ case-insensitive และ trim whitespace
	if !strings.EqualFold(got, strings.TrimSpace(wantHex)) {
		// checksum ไม่ตรง: binary อาจถูก tamper หรือ download เสียหาย
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, wantHex)
	}
	return nil
}

// replaceBinary สลับ newPath เข้ามาแทน currentPath
// โดยเก็บ binary เก่าไว้เป็น currentPath+".old" เพื่อให้ rollback ได้ถ้า restart ล้มเหลว
// การ rename binary ที่กำลังทำงานออกก่อนทำงานได้บน Windows (executable ที่กำลังรันสามารถ rename ได้
// แต่ลบไม่ได้) และ POSIX เหมือนกัน ถ้าสลับ binary ใหม่ล้มเหลว จะ restore binary เก่ากลับมา
func replaceBinary(currentPath, newPath string) error {
	// กำหนด path ของ backup file
	oldPath := currentPath + ".old"
	// ลบ backup เก่าถ้ามีอยู่ เพื่อเตรียมพื้นที่ (ไม่สนใจ error ถ้าไม่มี)
	_ = os.Remove(oldPath) // clear any stale backup

	// rename binary ปัจจุบันไปยัง .old เพื่อสำรองไว้ก่อนสลับ
	if err := os.Rename(currentPath, oldPath); err != nil {
		// คืน error ถ้า backup ล้มเหลว (binary ปัจจุบันยังอยู่ที่เดิม)
		return fmt.Errorf("back up current binary: %w", err)
	}
	// rename binary ใหม่ไปยัง currentPath (atomic swap บน filesystem เดียวกัน)
	if err := os.Rename(newPath, currentPath); err != nil {
		// สลับล้มเหลว: พยายาม restore binary เก่ากลับมาจาก .old
		if rbErr := os.Rename(oldPath, currentPath); rbErr != nil {
			// rollback ก็ล้มเหลวด้วย: สถานการณ์ร้ายแรง ไม่มี binary ที่ใช้งานได้
			return fmt.Errorf("install new binary failed (%v) AND rollback failed: %w", err, rbErr)
		}
		// rollback สำเร็จ: binary เก่ากลับมาที่ currentPath แล้ว
		return fmt.Errorf("install new binary: %w", err)
	}
	return nil
}
