// Package installer handles the self-installing download flow: reading the
// config trailer the backend appends to the agent binary, and (on Windows)
// relaunching elevated so the service can be registered. When a user
// double-clicks the downloaded SoftSentry-Setup.exe the agent reads its own
// embedded config and installs itself with no typing required.
//
// (TH) แพ็กเกจ installer จัดการ flow การดาวน์โหลดแบบติดตั้งตัวเอง: อ่าน config
// trailer ที่ backend แนบต่อท้ายไบนารี agent และ (บน Windows) relaunch แบบ
// elevated เพื่อให้ลงทะเบียน service ได้ เมื่อผู้ใช้ดับเบิลคลิก
// SoftSentry-Setup.exe ที่ดาวน์โหลดมา agent จะอ่าน config ที่ฝังมาในตัวเองและ
// ติดตั้งตัวเองโดยไม่ต้องพิมพ์อะไรเลย
package installer

import (
	"encoding/binary" // ใช้อ่าน/เขียน uint64 แบบ Little-Endian สำหรับฟิลด์ความยาวของ trailer
	"encoding/json"   // ใช้ marshal/unmarshal Config struct เป็น JSON ใน trailer
	"fmt"             // ใช้จัดรูปแบบ error message
	"io"              // ใช้ interface สำหรับการเขียนข้อมูล (io.Writer) และ CopyN
	"os"              // ใช้เปิด/สร้างไฟล์ และอ่าน metadata ไฟล์
)

// magic is the 8-byte footer identifying a SoftSentry config trailer. It must
// stay byte-identical to backend app/services/installer_service.py.
//
// (TH) magic คือ signature ขนาด 8 ไบต์ที่ใช้ระบุว่าเป็น SoftSentry config trailer
// ค่านี้ต้องตรงกับฝั่ง backend (app/services/installer_service.py) ทุกไบต์
// ใช้ตรวจสอบว่าไบนารีนี้มี trailer ต่อท้ายอยู่หรือไม่
var magic = []byte("SSCFGv1\x00")

// footerLen is the trailer footer: uint64 length (8) + magic (8).
// (TH) footerLen คือขนาดรวมของ footer ส่วนท้ายของ trailer
// ประกอบด้วย: ความยาวของ JSON payload แบบ uint64 (8 ไบต์) + magic bytes (8 ไบต์)
// รวมเป็น 16 ไบต์สุดท้ายของไฟล์ installer
const footerLen = 8 + 8

// Config is the embedded enrollment config carried in the trailer.
// (TH) Config คือโครงสร้างข้อมูล config สำหรับ enroll ที่ฝังอยู่ใน trailer ของ installer
// ประกอบด้วยข้อมูลที่จำเป็นสำหรับการลงทะเบียน agent กับ server
type Config struct {
	ServerURL string `json:"server_url"` // URL ของ backend server ที่ agent จะรายงานผล
	Token     string `json:"token"`      // deployment token สำหรับ enroll agent กับ server
}

// Embedded is the result of reading a binary's trailer.
// (TH) Embedded คือโครงสร้างที่เก็บผลลัพธ์จากการอ่าน trailer ของไบนารี installer
// ใช้ส่งข้อมูลกลับมาให้ผู้เรียก ReadEmbeddedConfig()
type Embedded struct {
	Config Config // Config คือค่า config ที่อ่านได้จาก trailer (ServerURL และ Token)
	// OriginalLen is the size of the agent binary without the trailer, so a
	// self-install can copy itself cleanly (the installed copy must not carry
	// the trailer, or it would try to self-install again).
	//
	// (TH) OriginalLen คือขนาดของไบนารี agent ต้นฉบับโดยไม่รวม trailer
	// ใช้ในการคัดลอกตัวเองระหว่าง self-install: ต้องคัดลอกเฉพาะส่วนนี้เท่านั้น
	// เพราะสำเนาที่ติดตั้งแล้วไม่ควรมี trailer ติดไปด้วย ไม่อย่างนั้นจะ self-install ซ้ำ
	OriginalLen int64
}

// ReadEmbeddedConfig reads the config trailer from the executable at exePath.
// ok is false (with nil error) when the file has no SoftSentry trailer — the
// normal case for a CLI-built binary. A non-nil error means the trailer was
// present but malformed.
//
// (TH) ReadEmbeddedConfig อ่าน config trailer จากไฟล์ executable ที่ระบุใน exePath
//
// ค่าที่คืนกลับ:
//   - emb: ข้อมูล Embedded ที่อ่านได้ (ใช้งานได้เมื่อ ok=true เท่านั้น)
//   - ok: true ถ้าพบ trailer ที่ถูกต้อง, false ถ้าไม่มี trailer (ไม่ใช่ error)
//   - err: ไม่เป็น nil เฉพาะเมื่อพบ trailer แต่รูปแบบผิดพลาด
func ReadEmbeddedConfig(exePath string) (emb Embedded, ok bool, err error) {
	// เปิดไฟล์ executable เพื่ออ่าน trailer
	// #nosec G304 — path คือ os.Executable() ของตัวเอง ไม่ได้รับมาจากผู้ใช้ จึงปลอดภัย
	f, err := os.Open(exePath) // #nosec G304 — path is os.Executable()
	if err != nil {
		// ถ้าเปิดไฟล์ไม่ได้ (เช่น ไม่มีสิทธิ์) ให้คืน error
		return Embedded{}, false, fmt.Errorf("open self: %w", err)
	}
	// ปิดไฟล์เมื่อฟังก์ชันจบ เพื่อป้องกัน resource leak
	defer f.Close()

	// ดึงข้อมูล metadata ของไฟล์ เช่น ขนาดไฟล์
	info, err := f.Stat()
	if err != nil {
		// ถ้าดึง metadata ไม่ได้ให้คืน error
		return Embedded{}, false, fmt.Errorf("stat self: %w", err)
	}
	// อ่านขนาดไฟล์ทั้งหมดเป็น bytes
	size := info.Size()
	// ถ้าไฟล์เล็กกว่า footer ขั้นต่ำ แสดงว่าไม่มี trailer อยู่ในนั้น
	if size < footerLen {
		return Embedded{}, false, nil
	}

	// อ่าน footer 16 ไบต์สุดท้ายของไฟล์เพื่อตรวจสอบ magic signature
	footer := make([]byte, footerLen)
	if _, err := f.ReadAt(footer, size-footerLen); err != nil {
		// ถ้าอ่าน footer ไม่ได้ให้คืน error
		return Embedded{}, false, fmt.Errorf("read footer: %w", err)
	}
	// ตรวจสอบ 8 ไบต์หลังของ footer ว่าตรงกับ magic signature หรือไม่
	if string(footer[8:]) != string(magic) {
		// magic ไม่ตรง — ไฟล์นี้ไม่มี trailer เป็นไบนารีปกติ (ไม่ใช่ error)
		return Embedded{}, false, nil // no trailer — ordinary binary
		// (TH) ไม่มี trailer — เป็นไบนารีปกติที่ build จาก CLI ไม่ใช่ installer
	}

	// อ่านความยาวของ JSON payload จาก 8 ไบต์แรกของ footer (Little-Endian uint64)
	jsonLen := int64(binary.LittleEndian.Uint64(footer[:8]))
	// คำนวณตำแหน่งเริ่มต้นของ JSON payload ในไฟล์
	start := size - footerLen - jsonLen
	// ตรวจสอบความถูกต้องของ jsonLen เพื่อป้องกัน out-of-bounds read
	if jsonLen <= 0 || start < 0 {
		// ถ้า jsonLen เป็น 0 หรือทำให้ start เป็นลบ แสดงว่า trailer เสียหาย
		return Embedded{}, false, fmt.Errorf("trailer length %d out of range", jsonLen)
	}

	// อ่าน JSON payload ตามความยาวที่ระบุใน footer
	buf := make([]byte, jsonLen)
	if _, err := f.ReadAt(buf, start); err != nil {
		// ถ้าอ่าน JSON payload ไม่ได้ (เช่น ไฟล์สั้นกว่าที่คิด)
		return Embedded{}, false, fmt.Errorf("read trailer config: %w", err)
	}
	// แปลง JSON bytes เป็น Config struct
	var cfg Config
	if err := json.Unmarshal(buf, &cfg); err != nil {
		// ถ้า JSON มีรูปแบบผิดพลาด เช่น ข้อมูลเสียหาย
		return Embedded{}, false, fmt.Errorf("parse trailer config: %w", err)
	}
	// ตรวจสอบว่า config มีข้อมูลครบถ้วน ทั้ง ServerURL และ Token ต้องไม่ว่าง
	if cfg.ServerURL == "" || cfg.Token == "" {
		// ถ้าขาดฟิลด์ที่จำเป็น agent จะไม่สามารถ enroll กับ server ได้
		return Embedded{}, false, fmt.Errorf("trailer config missing server_url or token")
	}
	// คืนค่า Embedded พร้อม Config และ OriginalLen (ตำแหน่งเริ่มต้นของ JSON คือขนาดไบนารีต้นฉบับ)
	return Embedded{Config: cfg, OriginalLen: start}, true, nil
}

// CopyStripped copies the first originalLen bytes of src to dst (0o755), i.e.
// the agent binary without its config trailer.
// (TH) CopyStripped คัดลอกเฉพาะ originalLen ไบต์แรกของ src ไปยัง dst
// เพื่อได้ไบนารี agent ที่ไม่มี config trailer ติดมาด้วย
// สิทธิ์ไฟล์ dst คือ 0o755 (executable โดย everyone)
//
// parameter:
//   - src: path ของไฟล์ installer ต้นทาง (มี trailer อยู่)
//   - dst: path ของไฟล์ปลายทางที่จะติดตั้ง (จะไม่มี trailer)
//   - originalLen: จำนวนไบต์ที่จะคัดลอก (ขนาดไบนารีต้นฉบับ ไม่รวม trailer)
func CopyStripped(src, dst string, originalLen int64) error {
	// เปิดไฟล์ installer ต้นทางเพื่ออ่าน
	// #nosec G304 — path คือ os.Executable() ของตัวเอง จึงปลอดภัย
	in, err := os.Open(src) // #nosec G304 — path is os.Executable()
	if err != nil {
		// ถ้าเปิดไฟล์ต้นทางไม่ได้ให้คืน error
		return fmt.Errorf("open source: %w", err)
	}
	// ปิดไฟล์ต้นทางเมื่อฟังก์ชันจบ
	defer in.Close()

	// สร้างหรือทับไฟล์ปลายทางด้วยสิทธิ์ 0o755 (executable)
	// O_CREATE: สร้างไฟล์ใหม่ถ้ายังไม่มี, O_TRUNC: ล้างเนื้อหาเดิมถ้ามีอยู่, O_WRONLY: เขียนอย่างเดียว
	// #nosec G302 — เป็นไบนารีของโปรแกรมเอง ต้องการสิทธิ์ executable จึงตั้งค่าตามนี้โดยตั้งใจ
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755) // #nosec G302
	if err != nil {
		// ถ้าสร้างไฟล์ปลายทางไม่ได้ (เช่น ไม่มีสิทธิ์เขียน)
		return fmt.Errorf("create dest: %w", err)
	}
	// ปิดไฟล์ปลายทางเมื่อฟังก์ชันจบ (เพื่อ flush buffer)
	defer out.Close()

	// คัดลอกเฉพาะ originalLen ไบต์แรก (ไบนารีต้นฉบับ ไม่รวม trailer)
	if _, err := io.CopyN(out, in, originalLen); err != nil {
		// ถ้าคัดลอกล้มเหลวกลางคัน (เช่น disk full)
		return fmt.Errorf("copy binary: %w", err)
	}
	// ปิดไฟล์ปลายทางอีกครั้งเพื่อ flush และตรวจสอบ error จากการปิดไฟล์
	// (defer out.Close() ข้างบนจะถูกเรียกอีกครั้ง แต่ duplicate close บน os.File ปลอดภัย)
	return out.Close()
}

// appendTrailer writes binary + config trailer to w. Used by tests to mirror
// the backend's build_installer; production trailers are produced server-side.
//
// (TH) appendTrailer เขียนไบนารีต้นฉบับ + config trailer ลงใน w
// ใช้ในเทสต์เพื่อจำลองการทำงานของ build_installer ฝั่ง backend
// ใน production จริง trailer จะถูกสร้างโดย backend server ไม่ใช่ agent
//
// โครงสร้าง trailer ที่เขียน:
//   [binaryBytes] [JSON payload] [uint64 jsonLen (LE)] [magic 8 bytes]
//
// parameter:
//   - w: io.Writer ที่จะเขียนข้อมูลลงไป
//   - binaryBytes: ข้อมูล binary ต้นฉบับ (จำลอง PE/Mach-O binary)
//   - cfg: Config ที่จะ embed ลงใน trailer
func appendTrailer(w io.Writer, binaryBytes []byte, cfg Config) error {
	// เขียนข้อมูล binary ต้นฉบับลงใน writer ก่อน
	if _, err := w.Write(binaryBytes); err != nil {
		// ถ้าเขียน binary ล้มเหลว
		return err
	}
	// แปลง Config struct เป็น JSON bytes
	payload, err := json.Marshal(cfg)
	if err != nil {
		// ถ้า marshal JSON ล้มเหลว (ไม่ควรเกิดขึ้นกับ struct ปกติ)
		return err
	}
	// เขียน JSON payload ต่อท้าย binary
	if _, err := w.Write(payload); err != nil {
		// ถ้าเขียน JSON payload ล้มเหลว
		return err
	}
	// สร้าง buffer 8 ไบต์สำหรับเก็บความยาวของ JSON payload
	lenBuf := make([]byte, 8)
	// เข้ารหัสความยาว JSON เป็น uint64 แบบ Little-Endian ใส่ใน lenBuf
	binary.LittleEndian.PutUint64(lenBuf, uint64(len(payload)))
	// เขียนความยาว JSON (8 ไบต์) ต่อท้าย JSON payload
	if _, err := w.Write(lenBuf); err != nil {
		// ถ้าเขียนความยาวล้มเหลว
		return err
	}
	// เขียน magic bytes (8 ไบต์) เป็นลำดับสุดท้าย เพื่อให้ตรวจจับ trailer ได้
	_, err = w.Write(magic)
	return err
}
