// Package queue is the agent's durable local retry buffer for scan uploads
// (spec 1.7). When a POST /agents/scans fails (network down, server 5xx) the
// scan is persisted to disk and retried with exponential backoff so results
// survive process restarts and transient outages.
//
// Storage is a directory of one-JSON-file-per-scan rather than SQLite: the spec
// names SQLite, but a CGO SQLite driver breaks the non-negotiable single-binary
// cross-compile rule, and for a 10-item FIFO buffer SQLite is overkill. Each
// item is written with a temp-file + atomic rename, which gives the same
// crash-safety the spec relies on (edge case 5) without any dependency.
//
// (TH) แพ็กเกจ queue คือบัฟเฟอร์ retry ในเครื่องแบบถาวรของ agent สำหรับการ
// อัปโหลดผลสแกน (ตาม spec 1.7) เมื่อ POST /agents/scans ล้มเหลว (เน็ตเวิร์กล่ม,
// เซิร์ฟเวอร์ตอบ 5xx) ผลสแกนจะถูกบันทึกลงดิสก์และ retry แบบ exponential backoff
// เพื่อให้ผลลัพธ์รอดแม้โพรเซสจะรีสตาร์ทหรือเกิดการขัดข้องชั่วคราว
//
// เก็บข้อมูลแบบไดเรกทอรีที่มีไฟล์ JSON หนึ่งไฟล์ต่อหนึ่งผลสแกน แทนที่จะใช้
// SQLite: แม้ spec จะระบุ SQLite ไว้ แต่ CGO SQLite driver จะขัดกับกฎที่ต่อรอง
// ไม่ได้เรื่อง single-binary cross-compile และสำหรับบัฟเฟอร์ FIFO ขนาด 10 รายการ
// SQLite ถือว่าเกินความจำเป็น แต่ละรายการถูกเขียนด้วยวิธี temp-file + atomic
// rename ซึ่งให้ความปลอดภัยจากการขัดข้องแบบเดียวกับที่ spec ต้องการ (edge case
// 5) โดยไม่ต้องพึ่ง dependency ใดๆ
package queue

import (
	"crypto/rand"   // ใช้สร้างตัวเลขสุ่มที่ปลอดภัยสำหรับ ID
	"encoding/hex"  // ใช้แปลง byte เป็นสตริง hex
	"encoding/json" // ใช้ encode/decode JSON สำหรับ serialize item ลงไฟล์
	"errors"        // ใช้สร้าง error พื้นฐาน เช่น "item not found"
	"fmt"           // ใช้จัดรูปแบบ error message และสตริง
	"os"            // ใช้อ่าน/เขียน/ลบไฟล์และไดเรกทอรี
	"path/filepath" // ใช้ต่อ path แบบ cross-platform
	"sort"          // ใช้เรียงลำดับชื่อไฟล์ตาม timestamp
	"strings"       // ใช้ตรวจสอบ suffix ของชื่อไฟล์
	"sync"          // ใช้ Mutex เพื่อป้องกันการเข้าถึงพร้อมกันจากหลาย goroutine
	"time"          // ใช้คำนวณ backoff และตั้งเวลา retry

	"github.com/softsentry/agent/internal/config"    // ใช้หา path ไดเรกทอรี config ของแต่ละ OS
	"github.com/softsentry/agent/internal/transport" // ใช้ ScanRequest ซึ่งเป็น payload ที่ต้องการ retry
)

const (
	maxItems    = 10              // spec 1.7: cap at 10 scans, FIFO-drop oldest — (TH) จำกัดไว้ที่ 10 รายการ ตัดอันเก่าสุดทิ้งแบบ FIFO เมื่อเต็ม
	baseBackoff = 1 * time.Second // เวลา backoff ขั้นต้น ใช้เป็นฐานคำนวณ: 1s, 2s, 4s ...
	maxBackoff  = 5 * time.Minute // เพดาน backoff สูงสุด ไม่ให้รอนานเกิน 5 นาที
	fileSuffix  = ".json"         // นามสกุลไฟล์ของแต่ละ item ในคิว
)

// Item is one queued scan awaiting upload.
// (TH) Item คือผลสแกนหนึ่งรายการที่รอการอัปโหลดอยู่ในคิว
type Item struct {
	ID             string                `json:"id"`              // ID ไม่ซ้ำกันของ item นี้ ใช้ค้นหาและลบออกจากคิว
	IdempotencyKey string                `json:"idempotency_key"` // คีย์ป้องกันการแทรกข้อมูลซ้ำเมื่อ retry หลายครั้ง
	Request        transport.ScanRequest `json:"request"`         // payload ผลสแกนที่ต้องการส่งไปยัง backend
	Attempts       int                   `json:"attempts"`        // จำนวนครั้งที่พยายามอัปโหลดแล้ว (รวมครั้งแรกที่ล้มเหลว)
	CreatedAt      time.Time             `json:"created_at"`      // เวลาที่ item นี้ถูกสร้างขึ้น ใช้เรียงลำดับ FIFO
	NextAttempt    time.Time             `json:"next_attempt"`    // เวลาที่กำหนดให้ retry ครั้งถัดไป

	filename string // ชื่อไฟล์บนดิสก์ ไม่ serialize เข้า JSON เพราะเก็บไว้ใช้ภายในเท่านั้น
}

// Queue is a directory-backed FIFO retry buffer. Safe for concurrent use.
// (TH) Queue คือบัฟเฟอร์ retry แบบ FIFO ที่อิงไดเรกทอรี ใช้งานพร้อมกันได้อย่างปลอดภัย
type Queue struct {
	dir string     // path ของไดเรกทอรีที่เก็บไฟล์ item ทั้งหมด
	mu  sync.Mutex // mutex ป้องกัน race condition เมื่อเข้าถึงคิวพร้อมกันจากหลาย goroutine
}

// Open returns a Queue backed by dir, creating the directory if needed.
// (TH) คืนค่า Queue ที่อิง dir โดยจะสร้างไดเรกทอรีให้ถ้ายังไม่มี
func Open(dir string) (*Queue, error) {
	// สร้างไดเรกทอรีพร้อม parent ถ้ายังไม่มี กำหนดสิทธิ์ 0700 (เจ้าของอ่าน/เขียน/execute เท่านั้น)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create queue dir %s: %w", dir, err)
	}
	return &Queue{dir: dir}, nil
}

// Default returns the Queue at the per-OS config location (queue/ subdir).
// (TH) คืนค่า Queue ที่ตำแหน่ง config ของแต่ละ OS (ไดเรกทอรีย่อย queue/)
func Default() (*Queue, error) {
	// หา base directory ของ config ตาม OS ที่กำลังรันอยู่
	base, err := config.Dir()
	if err != nil {
		return nil, err
	}
	// เปิดหรือสร้างไดเรกทอรี queue/ ใต้ base directory
	return Open(filepath.Join(base, "queue"))
}

// Enqueue persists a failed scan for later retry. The first retry is scheduled
// one backoff interval out (the live POST already counts as attempt 1). If the
// queue is full the oldest item is dropped to make room (FIFO).
//
// (TH) บันทึกผลสแกนที่อัปโหลดล้มเหลวไว้เพื่อ retry ภายหลัง การ retry ครั้งแรกจะ
// ถูกตั้งเวลาให้ห่างออกไปหนึ่งช่วง backoff (การ POST ที่เพิ่งล้มเหลวนับเป็น
// ความพยายามครั้งที่ 1 อยู่แล้ว) ถ้าคิวเต็ม รายการเก่าสุดจะถูกตัดทิ้งเพื่อเปิด
// พื้นที่ (แบบ FIFO)
func (q *Queue) Enqueue(req transport.ScanRequest, idempotencyKey string) error {
	// ล็อก mutex ก่อนแก้ไขคิว เพื่อป้องกัน race condition
	q.mu.Lock()
	defer q.mu.Unlock()

	// บันทึกเวลาปัจจุบันเพื่อใช้เป็น timestamp สำหรับ FIFO ordering
	now := time.Now()
	// สร้าง item ใหม่พร้อม ID สุ่ม, Idempotency-Key, จำนวน attempt เริ่มต้นที่ 1
	// และตั้งเวลา retry ครั้งแรกด้วย backoff ของ attempt ที่ 1
	item := Item{
		ID:             NewKey(),
		IdempotencyKey: idempotencyKey,
		Request:        req,
		Attempts:       1,             // POST ที่ล้มเหลวนับเป็น attempt ที่ 1 แล้ว
		CreatedAt:      now,
		NextAttempt:    now.Add(backoff(1)), // retry ครั้งแรกห่างออกไป 1 วินาที (backoff ของ attempt=1)
	}
	// กำหนดชื่อไฟล์โดยนำ unix nanoseconds นำหน้า เพื่อให้เรียงตามตัวอักษร = เรียงตามเวลาสร้าง (FIFO ordering)
	item.filename = fmt.Sprintf("%020d-%s%s", now.UnixNano(), item.ID, fileSuffix)
	// เขียนไฟล์ item ลงดิสก์แบบ atomic
	if err := q.write(item); err != nil {
		return err
	}
	// ตรวจสอบว่าคิวเกิน maxItems หรือไม่ ถ้าเกินให้ตัดรายการเก่าสุดทิ้ง
	return q.enforceCap()
}

// Due returns items whose next-attempt time has arrived, oldest first.
// (TH) คืนค่ารายการที่ถึงเวลา retry แล้ว เรียงจากเก่าสุดไปก่อน
func (q *Queue) Due(now time.Time) ([]Item, error) {
	// ล็อก mutex ก่อนอ่านคิว
	q.mu.Lock()
	defer q.mu.Unlock()

	// โหลด item ทั้งหมดจากดิสก์
	items, err := q.load()
	if err != nil {
		return nil, err
	}
	// ใช้ slice เดิมเป็น destination เพื่อประหยัด allocation
	out := items[:0]
	// กรองเฉพาะ item ที่ NextAttempt ไม่เลยเวลา now (ถึงเวลา retry แล้ว)
	for _, it := range items {
		if !it.NextAttempt.After(now) {
			out = append(out, it)
		}
	}
	return out, nil
}

// Remove deletes a successfully-uploaded item.
// (TH) ลบรายการที่อัปโหลดสำเร็จแล้วออกจากคิว
func (q *Queue) Remove(id string) error {
	// ล็อก mutex ก่อนลบไฟล์
	q.mu.Lock()
	defer q.mu.Unlock()

	// ค้นหาชื่อไฟล์จาก ID ของ item
	name, err := q.findByID(id)
	if err != nil {
		return err
	}
	// ลบไฟล์ item ออกจากดิสก์
	return os.Remove(filepath.Join(q.dir, name))
}

// Reschedule bumps the attempt count and pushes the next retry out by the
// exponential backoff for the new attempt count.
// (TH) เพิ่มจำนวนครั้งที่พยายามและเลื่อนเวลา retry ครั้งถัดไปออกไปตาม
// exponential backoff ของจำนวนครั้งใหม่นั้น
func (q *Queue) Reschedule(id string, now time.Time) error {
	// ล็อก mutex ก่อนแก้ไข item
	q.mu.Lock()
	defer q.mu.Unlock()

	// ค้นหาชื่อไฟล์จาก ID
	name, err := q.findByID(id)
	if err != nil {
		return err
	}
	// อ่าน item ปัจจุบันจากดิสก์
	item, err := q.read(name)
	if err != nil {
		return err
	}
	// เพิ่มจำนวนครั้งที่พยายาม แล้วคำนวณเวลา retry ถัดไปตาม backoff ใหม่
	item.Attempts++
	item.NextAttempt = now.Add(backoff(item.Attempts))
	// เก็บชื่อไฟล์เดิมไว้เพื่อ overwrite ไฟล์เดิม
	item.filename = name
	// เขียน item ที่อัปเดตแล้วกลับลงดิสก์แบบ atomic
	return q.write(item)
}

// Depth reports the number of queued items.
// (TH) รายงานจำนวนรายการที่อยู่ในคิวปัจจุบัน
func (q *Queue) Depth() (int, error) {
	// ล็อก mutex ก่อนนับไฟล์
	q.mu.Lock()
	defer q.mu.Unlock()
	// อ่านรายชื่อไฟล์ทั้งหมดในไดเรกทอรีคิว
	names, err := q.names()
	if err != nil {
		return 0, err
	}
	// คืนค่าจำนวนไฟล์เป็นความลึกของคิว
	return len(names), nil
}

// --- internals -------------------------------------------------------------
// (TH) --- ฟังก์ชันภายในที่ใช้งานเฉพาะภายแพ็กเกจนี้ ---

// names returns queue filenames sorted oldest-first (FIFO).
// (TH) คืนค่ารายชื่อไฟล์ทั้งหมดในคิว เรียงจากเก่าสุดไปก่อน (FIFO)
func (q *Queue) names() ([]string, error) {
	// อ่านรายการไฟล์และไดเรกทอรีทั้งหมดใน queue directory
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return nil, fmt.Errorf("read queue dir: %w", err)
	}
	var names []string
	// วนผ่านทุก entry กรองเฉพาะไฟล์ (ไม่ใช่ไดเรกทอรี) ที่มีนามสกุล .json
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), fileSuffix) {
			names = append(names, e.Name())
		}
	}
	// เรียงตามตัวอักษร (lexical sort) ซึ่งเท่ากับเรียงตามเวลา เพราะชื่อไฟล์ขึ้นต้นด้วย unix-nano
	sort.Strings(names) // unix-nano prefix makes lexical == chronological — (TH) ตัวนำหน้าที่เป็น unix-nano ทำให้เรียงตามตัวอักษร = เรียงตามเวลาจริง
	return names, nil
}

// load reads all items from disk, skipping corrupt files.
// (TH) อ่าน item ทั้งหมดจากดิสก์ โดยข้ามไฟล์ที่เสียหาย
func (q *Queue) load() ([]Item, error) {
	// ดึงรายชื่อไฟล์ทั้งหมดในคิว
	names, err := q.names()
	if err != nil {
		return nil, err
	}
	// จอง slice ด้วยความจุเท่ากับจำนวนไฟล์เพื่อลด allocation
	items := make([]Item, 0, len(names))
	// วนอ่านแต่ละไฟล์แล้ว deserialize เป็น Item
	for _, name := range names {
		it, err := q.read(name)
		if err != nil {
			// ไฟล์ที่เสียหาย/เขียนไม่สมบูรณ์ ไม่ควรทำให้ทั้งคิวค้าง ข้ามไฟล์นั้นต่อไป
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

// read deserializes one item file by name.
// (TH) deserialize ไฟล์ item หนึ่งไฟล์ตามชื่อที่ระบุ
func (q *Queue) read(name string) (Item, error) {
	// อ่านเนื้อหาไฟล์ทั้งหมด — #nosec G304 เพราะเป็นไดเรกทอรีภายในของระบบเอง จึงปลอดภัย
	data, err := os.ReadFile(filepath.Join(q.dir, name)) // #nosec G304 — internal dir
	if err != nil {
		// คืนค่า error เมื่ออ่านไฟล์ไม่ได้ เช่น ไฟล์ถูกลบไประหว่างอ่าน
		return Item{}, err
	}
	var it Item
	// แปลง JSON bytes เป็น Item struct
	if err := json.Unmarshal(data, &it); err != nil {
		// คืนค่า error เมื่อ JSON เสียหายหรือรูปแบบไม่ถูกต้อง
		return Item{}, err
	}
	// เซ็ตชื่อไฟล์กลับเข้า struct เพราะ field นี้ไม่ถูก serialize ลง JSON
	it.filename = name
	return it, nil
}

// write atomically persists an item (temp file + rename).
// (TH) บันทึก item ลงดิสก์แบบ atomic โดยใช้วิธีเขียนไฟล์ temp แล้ว rename
// เพื่อป้องกันไฟล์เสียหายหาก process ถูกบังคับหยุดระหว่างเขียน
func (q *Queue) write(it Item) error {
	// Serialize item เป็น JSON bytes
	data, err := json.Marshal(it)
	if err != nil {
		// error นี้เกิดเมื่อ item มี field ที่ marshal ไม่ได้ (ไม่น่าเกิดในทางปฏิบัติ)
		return fmt.Errorf("marshal queue item: %w", err)
	}
	// กำหนด path ของไฟล์ปลายทางและไฟล์ temp
	final := filepath.Join(q.dir, it.filename)
	tmp := final + ".tmp"
	// เขียนข้อมูลลงไฟล์ temp ก่อน ด้วยสิทธิ์ 0600 (เจ้าของอ่าน/เขียนเท่านั้น)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		// error นี้เกิดเมื่อดิสก์เต็มหรือไม่มีสิทธิ์เขียน
		return fmt.Errorf("write queue item: %w", err)
	}
	// rename ไฟล์ temp ไปเป็นไฟล์ปลายทาง (atomic บน POSIX, best-effort บน Windows)
	if err := os.Rename(tmp, final); err != nil {
		// ถ้า rename ล้มเหลว ให้ลบไฟล์ temp ทิ้งเพื่อไม่ให้ค้างอยู่ในไดเรกทอรี
		_ = os.Remove(tmp)
		return fmt.Errorf("commit queue item: %w", err)
	}
	return nil
}

// findByID searches the queue directory for the file matching the given ID.
// (TH) ค้นหาชื่อไฟล์ในไดเรกทอรีคิวที่ตรงกับ ID ที่ระบุ
func (q *Queue) findByID(id string) (string, error) {
	// อ่านรายชื่อไฟล์ทั้งหมด
	names, err := q.names()
	if err != nil {
		return "", err
	}
	// สร้าง suffix ที่คาดหวัง: "-<id>.json"
	suffix := "-" + id + fileSuffix
	// วนหาไฟล์ที่ลงท้ายด้วย suffix นี้
	for _, name := range names {
		if strings.HasSuffix(name, suffix) {
			return name, nil
		}
	}
	// ถ้าหาไม่เจอให้คืนค่า error พร้อม ID ที่ค้นหา
	return "", errors.New("queue item not found: " + id)
}

// enforceCap drops the oldest items until at most maxItems remain.
// (TH) ตัดรายการเก่าสุดทิ้งจนกว่าจะเหลือไม่เกิน maxItems รายการ
func (q *Queue) enforceCap() error {
	// อ่านรายชื่อไฟล์ทั้งหมด (เรียงจากเก่าสุด)
	names, err := q.names()
	if err != nil {
		return err
	}
	// วนลบรายการแรก (เก่าสุด) ไปเรื่อยๆ จนกว่าจำนวนจะไม่เกิน maxItems
	for len(names) > maxItems {
		if err := os.Remove(filepath.Join(q.dir, names[0])); err != nil {
			// error นี้เกิดเมื่อไฟล์ถูกลบไปแล้วก่อนหน้าหรือไม่มีสิทธิ์ลบ
			return fmt.Errorf("drop oldest queue item: %w", err)
		}
		// ตัดรายการแรกออกจาก slice เพื่อเลื่อนไปลบรายการถัดไป
		names = names[1:]
	}
	return nil
}

// backoff returns the retry delay for the given attempt count: base*2^(n-1),
// capped at maxBackoff. attempts < 1 is treated as 1.
// (TH) คืนค่าเวลาหน่วงก่อน retry ตามจำนวนครั้งที่พยายาม: base*2^(n-1)
// แต่ไม่เกิน maxBackoff ถ้า attempts < 1 จะถือว่าเป็น 1
// ตัวอย่าง: attempt=1→1s, attempt=2→2s, attempt=3→4s, attempt=10→5min
func backoff(attempts int) time.Duration {
	// ถ้า attempts น้อยกว่า 1 ให้ปรับเป็น 1 เพื่อหลีกเลี่ยงผลลัพธ์ที่ไม่คาดคิด
	if attempts < 1 {
		attempts = 1
	}
	// ถ้า attempts มากกว่า 30 ให้คืนค่า maxBackoff ทันที เพื่อป้องกัน bit shift overflow
	if attempts > 30 { // guard against shift overflow — (TH) ป้องกัน overflow จากการ left shift เกิน 30 บิต
		return maxBackoff
	}
	// คำนวณ delay ด้วยสูตร exponential: baseBackoff << (attempts-1) = baseBackoff * 2^(attempts-1)
	d := baseBackoff << (attempts - 1)
	// ถ้าผลลัพธ์เกิน maxBackoff หรือกลายเป็นลบ (overflow) ให้ใช้ maxBackoff แทน
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}

// NewKey returns a random hex identifier used both as the item ID and, when the
// caller has none, as an Idempotency-Key so retries never double-insert.
// (TH) คืนค่า identifier แบบสุ่มในรูป hex ใช้เป็นทั้ง item ID และ (เมื่อผู้เรียก
// ไม่มีให้) Idempotency-Key เพื่อไม่ให้การ retry แทรกข้อมูลซ้ำ
func NewKey() string {
	// สุ่ม 8 bytes (64 บิต) จาก crypto/rand เพื่อสร้าง ID ที่ไม่ซ้ำกัน
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand ล้มเหลวถือเป็นสิ่งที่กู้คืนไม่ได้ในทางปฏิบัติ จึงใช้ unix nanosecond แทน
		return fmt.Sprintf("%020d", time.Now().UnixNano())
	}
	// แปลง bytes เป็นสตริง hex 16 ตัวอักษร
	return hex.EncodeToString(b[:])
}
