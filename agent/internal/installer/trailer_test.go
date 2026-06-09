// แพ็กเกจ installer_test ทดสอบฟังก์ชันใน package installer
// ใช้ white-box testing (อยู่ใน package เดียวกัน) เพื่อเข้าถึง unexported function appendTrailer
package installer

import (
	"bytes"        // ใช้ bytes.Buffer เป็น in-memory writer และเปรียบเทียบ byte slices
	"os"           // ใช้อ่านไฟล์ที่ติดตั้งแล้วเพื่อตรวจสอบผลลัพธ์
	"path/filepath" // ใช้สร้าง path ของไฟล์ทดสอบใน temp directory
	"testing"      // framework สำหรับเขียน unit test ใน Go
)

// writeInstaller เป็น helper function สำหรับเทสต์
// สร้างไฟล์ installer จำลองโดยเขียน binaryBytes + trailer ลงไฟล์ใน dir
// คืน path ของไฟล์ที่สร้าง
// parameter:
//   - t: testing.T สำหรับรายงาน error
//   - dir: ไดเรกทอรีที่จะสร้างไฟล์ (ควรเป็น t.TempDir())
//   - binaryBytes: ข้อมูลที่จำลองเป็น binary ต้นฉบับ
//   - cfg: Config ที่จะ embed ลงใน trailer
func writeInstaller(t *testing.T, dir string, binaryBytes []byte, cfg Config) string {
	// บอก testing framework ว่านี่เป็น helper function
	// เมื่อ test fail บรรทัดที่รายงานจะเป็นบรรทัดที่เรียก writeInstaller ไม่ใช่ภายในนี้
	t.Helper()
	// สร้าง in-memory buffer สำหรับเขียน installer ชั่วคราว
	var buf bytes.Buffer
	// เขียน binary + trailer ลง buffer โดยใช้ appendTrailer
	if err := appendTrailer(&buf, binaryBytes, cfg); err != nil {
		// ถ้าสร้าง trailer ล้มเหลว ให้หยุดเทสต์ทันที
		t.Fatalf("appendTrailer: %v", err)
	}
	// กำหนด path ของไฟล์ installer จำลอง
	path := filepath.Join(dir, "setup.exe")
	// เขียนข้อมูลจาก buffer ลงไฟล์จริงด้วยสิทธิ์ executable
	if err := os.WriteFile(path, buf.Bytes(), 0o755); err != nil {
		// ถ้าเขียนไฟล์ไม่ได้ ให้หยุดเทสต์ทันที
		t.Fatalf("write file: %v", err)
	}
	// คืน path ของไฟล์ installer ที่สร้าง
	return path
}

// TestReadEmbeddedConfigRoundTrip ทดสอบ round-trip ของ trailer:
// สร้าง installer พร้อม config → อ่าน config กลับมา → ตรวจสอบว่าตรงกัน
// รวมถึงตรวจสอบว่า OriginalLen บอกขนาด binary ต้นฉบับได้ถูกต้อง
func TestReadEmbeddedConfigRoundTrip(t *testing.T) {
	// สร้าง binary ต้นฉบับจำลอง: ขึ้นต้นด้วย "MZ" (PE header) ตามด้วยข้อมูลซ้ำ 500 ครั้ง
	bin := append([]byte("MZ"), bytes.Repeat([]byte("pe"), 500)...)
	// กำหนด config ที่จะ embed ลงใน trailer
	cfg := Config{ServerURL: "http://192.168.1.50:8001", Token: "tok-xyz"}
	// สร้างไฟล์ installer จำลองใน temp directory
	path := writeInstaller(t, t.TempDir(), bin, cfg)

	// อ่าน config จาก installer ที่สร้างไว้
	emb, ok, err := ReadEmbeddedConfig(path)
	// ต้องไม่มี error และ ok ต้องเป็น true (พบ trailer ที่ถูกต้อง)
	if err != nil || !ok {
		t.Fatalf("ReadEmbeddedConfig ok=%v err=%v", ok, err)
	}
	// ตรวจสอบว่า Config ที่อ่านได้ตรงกับ Config ที่ embed ไว้
	if emb.Config != cfg {
		t.Errorf("config = %+v, want %+v", emb.Config, cfg)
	}
	// ตรวจสอบว่า OriginalLen ตรงกับขนาดของ binary ต้นฉบับ
	if emb.OriginalLen != int64(len(bin)) {
		t.Errorf("OriginalLen = %d, want %d", emb.OriginalLen, len(bin))
	}
}

// TestReadEmbeddedConfigNoTrailer ทดสอบกรณีที่ไฟล์ไม่มี trailer
// ReadEmbeddedConfig ต้องคืน ok=false โดยไม่มี error
func TestReadEmbeddedConfigNoTrailer(t *testing.T) {
	// สร้างไฟล์ binary ธรรมดาที่ไม่มี trailer
	path := filepath.Join(t.TempDir(), "plain.exe")
	// เขียนไฟล์ binary ธรรมดาที่ไม่มี SoftSentry trailer
	if err := os.WriteFile(path, []byte("just a normal binary"), 0o755); err != nil {
		// ถ้าสร้างไฟล์ทดสอบไม่ได้ ให้หยุดเทสต์ทันที
		t.Fatal(err)
	}
	// อ่านไฟล์ที่ไม่มี trailer ต้องได้ ok=false, err=nil
	_, ok, err := ReadEmbeddedConfig(path)
	// ต้องไม่มี error — ไม่มี trailer ไม่ถือเป็นข้อผิดพลาด
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ok ต้องเป็น false เพราะไฟล์นี้ไม่มี trailer
	if ok {
		t.Error("expected ok=false for binary without trailer")
	}
}

// TestCopyStrippedRemovesTrailer ทดสอบว่า CopyStripped ลบ trailer ออกได้ถูกต้อง:
// ไฟล์ที่คัดลอกมาต้องมีเนื้อหาเหมือน binary ต้นฉบับพอดี ไม่มี trailer เหลืออยู่
func TestCopyStrippedRemovesTrailer(t *testing.T) {
	// สร้าง temp directory สำหรับเทสต์นี้
	dir := t.TempDir()
	// สร้าง binary ต้นฉบับจำลองขนาด 2048 ไบต์ เต็มไปด้วย 'X'
	bin := bytes.Repeat([]byte("X"), 2048)
	// กำหนด config ที่จะ embed
	cfg := Config{ServerURL: "http://s", Token: "t"}
	// สร้างไฟล์ installer จำลองใน temp directory
	src := writeInstaller(t, dir, bin, cfg)

	// อ่าน config และ OriginalLen จากไฟล์ installer
	emb, ok, err := ReadEmbeddedConfig(src)
	// ต้องอ่านสำเร็จก่อนจึงจะทดสอบ CopyStripped ได้
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}

	// กำหนด path ของไฟล์ที่จะติดตั้ง (ผลลัพธ์หลัง strip trailer)
	dst := filepath.Join(dir, "installed.exe")
	// คัดลอกเฉพาะส่วน binary ต้นฉบับ (ไม่รวม trailer) ไปยัง dst
	if err := CopyStripped(src, dst, emb.OriginalLen); err != nil {
		// ถ้า CopyStripped ล้มเหลว ให้หยุดเทสต์ทันที
		t.Fatalf("CopyStripped: %v", err)
	}

	// อ่านเนื้อหาของไฟล์ที่ติดตั้งแล้วเพื่อตรวจสอบ
	got, err := os.ReadFile(dst)
	if err != nil {
		// ถ้าอ่านไฟล์ผลลัพธ์ไม่ได้ ให้หยุดเทสต์
		t.Fatal(err)
	}
	// เปรียบเทียบเนื้อหาของไฟล์ที่ติดตั้งกับ binary ต้นฉบับ ต้องเหมือนกันทุกไบต์
	if !bytes.Equal(got, bin) {
		t.Errorf("stripped copy length %d, want %d (trailer not removed)", len(got), len(bin))
	}
	// The stripped copy must not look like an installer anymore.
	// (TH) สำเนาที่ตัด trailer ออกแล้วต้องไม่ถูกระบุว่าเป็น installer อีกต่อไป
	// ถ้ายังมี trailer อยู่ agent ที่ติดตั้งแล้วจะพยายาม self-install ซ้ำอีกครั้ง
	if _, ok, _ := ReadEmbeddedConfig(dst); ok {
		t.Error("stripped copy still has a trailer")
	}
}
