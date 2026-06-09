// Package runner holds the long-running agent loop (initial scan + periodic
// heartbeat + triggered scan/update). It lives outside cmd/ so both the
// foreground `run` command and the OS service handlers (Windows SCM / macOS
// launchd) can drive the same logic.
//
// (TH) แพ็กเกจ runner เก็บลูปการทำงานระยะยาวของ agent (สแกนเริ่มต้น +
// heartbeat ตามรอบ + สแกน/อัปเดตเมื่อถูกสั่ง) มันอยู่นอก cmd/ เพื่อให้ทั้งคำสั่ง
// `run` แบบ foreground และตัวจัดการ OS service (Windows SCM / macOS launchd)
// ใช้ logic เดียวกันได้
package runner

import (
	"context"  // ใช้จัดการ cancellation และ timeout ของแต่ละ operation
	"errors"   // ใช้สร้าง sentinel error เช่น ErrRestartRequired
	"fmt"      // ใช้จัดรูปแบบ log message และ error string
	"io"       // ใช้ io.Writer สำหรับ output และ io.Discard เพื่อทิ้ง output
	"os"       // ใช้หา executable path ของตัวเอง (os.Executable)
	"strings"  // ใช้ EqualFold เพื่อเปรียบเทียบ SHA-256 แบบ case-insensitive
	"sync"     // ใช้ WaitGroup เพื่อรอให้ background worker หยุดก่อน return
	"time"     // ใช้คำนวณ interval, timeout และ uptime ของ agent

	"github.com/softsentry/agent/internal/config"    // ใช้โหลด config ที่ enroll ไว้ (server URL, auto-update flag)
	"github.com/softsentry/agent/internal/queue"     // ใช้บัฟเฟอร์ retry สำหรับการอัปโหลดที่ล้มเหลว
	"github.com/softsentry/agent/internal/scanner"   // ใช้สแกน software inventory บนเครื่อง
	"github.com/softsentry/agent/internal/schedule"  // ใช้คำนวณว่าถึงเวลาสแกนครั้งถัดไปหรือยัง
	"github.com/softsentry/agent/internal/storage"   // ใช้อ่าน/เขียน token, last_scan_at, last_update marker
	"github.com/softsentry/agent/internal/transport" // ใช้ HTTP client ส่ง heartbeat และ scan payload
	"github.com/softsentry/agent/internal/updater"   // ใช้ดาวน์โหลดและแทนที่ binary ตัวใหม่
)

// DefaultHeartbeatInterval คือความถี่มาตรฐานของ heartbeat ที่ agent ส่งให้ backend
// ตาม spec 1.5 กำหนดไว้ที่ 60 วินาที
const DefaultHeartbeatInterval = 60 * time.Second

// ErrRestartRequired is returned by Run after a successful self-update: the
// new binary is in place and the process must exit so the service manager
// (Windows SCM recovery / macOS launchd KeepAlive) restarts onto it.
//
// (TH) Run จะคืนค่านี้หลัง self-update สำเร็จ: ไบนารีตัวใหม่อยู่ในตำแหน่งแล้ว
// และโพรเซสต้องออกจากการทำงาน เพื่อให้ service manager (Windows SCM recovery /
// macOS launchd KeepAlive) รันมันขึ้นมาใหม่ด้วยไบนารีตัวนั้น
var ErrRestartRequired = errors.New("agent updated; restart required to apply")

// Options tunes a Run invocation.
// (TH) Options ใช้ปรับแต่งพฤติกรรมการเรียก Run โดยผู้เรียกสามารถ override ค่าต่างๆ ได้
type Options struct {
	// OneShot scans + heartbeats once, then returns (used by tests/debug).
	// (TH) ถ้าเป็น true: สแกน + heartbeat เพียงครั้งเดียวแล้วคืนค่า ใช้สำหรับ test/debug
	OneShot bool
	// HeartbeatInterval overrides DefaultHeartbeatInterval when > 0.
	// (TH) ถ้าค่ามากกว่า 0 จะใช้แทนค่า DefaultHeartbeatInterval
	HeartbeatInterval time.Duration
	// Out/Err receive human-readable progress; either may be nil (discarded).
	// (TH) Out รับข้อความความคืบหน้าปกติ, Err รับ warning/error ที่อ่านได้
	// ค่าใดค่าหนึ่งเป็น nil ได้ (จะถูก discard อัตโนมัติ)
	Out io.Writer
	Err io.Writer
}

// Run loads config + token, then drives the agent loop until ctx is cancelled
// (or once when OneShot). It returns ctx.Err() on cancellation, or a setup
// error if the agent is not enrolled.
//
// (TH) โหลด config + token แล้วขับเคลื่อนลูปของ agent จนกว่า ctx จะถูกยกเลิก
// (หรือทำครั้งเดียวถ้าเป็น OneShot) จะคืนค่า ctx.Err() เมื่อถูกยกเลิก หรือ
// error การตั้งค่าถ้า agent ยังไม่ได้ enroll
func Run(ctx context.Context, opts Options) error {
	// แปลง nil writer เป็น io.Discard เพื่อให้เขียน log ได้โดยไม่ต้อง nil-check ทุกครั้ง
	out := writerOrDiscard(opts.Out)
	errw := writerOrDiscard(opts.Err)
	// ใช้ heartbeat interval ที่กำหนดมา ถ้าไม่ได้กำหนดให้ใช้ค่า default
	interval := opts.HeartbeatInterval
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}

	// โหลด config ที่บันทึกไว้ตอน enroll (server URL, scan interval, auto-update)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// ถ้า ServerURL ว่าง แสดงว่ายังไม่ได้ enroll ให้แจ้ง error พร้อม hint คำสั่ง enroll
	if cfg.ServerURL == "" {
		return fmt.Errorf("not enrolled — run `softsentry-agent enroll` first")
	}
	// โหลด agent token ที่บันทึกไว้ตอน enroll เพื่อใช้ authenticate กับ backend
	tok, err := storage.LoadToken()
	if err != nil {
		return err
	}
	// สร้าง HTTP client พร้อม server URL และ token สำหรับส่ง heartbeat และ scan
	client := transport.New(cfg.ServerURL, tok)
	// เปิดหรือสร้าง retry queue ที่ตำแหน่ง config ของ OS
	q, err := queue.Default()
	if err != nil {
		// error นี้เกิดเมื่อสร้างไดเรกทอรี queue ไม่ได้ (เช่น ไม่มีสิทธิ์)
		return fmt.Errorf("open retry queue: %w", err)
	}
	// บันทึกเวลาเริ่ม process เพื่อคำนวณ uptime ที่ส่งใน heartbeat
	started := time.Now()

	// หา path ของ executable ตัวเองสำหรับ self-update ถ้าหาไม่เจอจะปิด auto-update
	exePath, err := os.Executable()
	if err != nil {
		// ถ้าหา path ไม่ได้ (เกิดได้ในบางสภาพแวดล้อม) ให้ตั้งเป็นสตริงว่างและปิด auto-update
		exePath = ""
	}

	// คำนวณ scan interval จาก config (ชั่วโมง → Duration)
	scanInterval := schedule.Interval(cfg.ScanIntervalHours)
	// สร้าง loop struct ที่รวบรวม dependency ทั้งหมดไว้ด้วยกัน
	r := &loop{
		client:       client,
		queue:        q,
		out:          out,
		errw:         errw,
		started:      started,
		autoUpdate:   cfg.AutoUpdateEnabled,
		exePath:      exePath,
		version:      transport.Version, // เวอร์ชันที่ฝังอยู่ใน binary ณ ตอน build
		scanInterval: scanInterval,
	}

	// ตัดสินใจว่าถึงเวลาสแกนตอนเริ่มต้นหรือยัง (ตาม spec 1.1):
	// - การ enroll ใหม่ → สแกนทันที (lastScan เป็น zero value)
	// - การรีสตาร์ท → คำนวณจาก last_scan_at เพื่อไม่ให้สแกนซ้ำเร็วเกินไป
	lastScan, err := storage.LoadLastScan()
	if err != nil {
		// error อ่าน last scan time ไม่ใช่ fatal เพียงแต่ log ไว้
		fmt.Fprintf(errw, "read last scan time: %v\n", err)
	}
	// คำนวณว่าต้องรออีกนานแค่ไหนก่อนสแกนครั้งถัดไป (0 = ถึงเวลาแล้ว)
	dueIn := schedule.DueIn(lastScan, scanInterval, time.Now())
	initialTrigger := ""
	if dueIn == 0 {
		// ถึงเวลาสแกนทันที: กำหนด trigger เป็น "auto" หรือ "enroll" ถ้าเป็น enrollment ใหม่
		initialTrigger = "auto"
		if lastScan.IsZero() {
			initialTrigger = "enroll" // enrollment ใหม่: ไม่เคยสแกนมาก่อน
		}
		// ตั้ง dueIn เพื่อใช้เป็น interval ของ scan timer
		dueIn = scanInterval
	} else {
		// ยังไม่ถึงเวลา: แสดงข้อความว่าสแกนครั้งถัดไปอีกนานแค่ไหน
		fmt.Fprintf(out, "Next auto-scan in %s (last scan %s ago).\n",
			dueIn.Round(time.Minute), time.Since(lastScan).Round(time.Minute))
	}

	if opts.OneShot {
		// โหมด one-shot ทำงานแบบ synchronous ทั้งหมด (ใช้สำหรับ test/debug):
		// สแกนถ้าถึงเวลา แล้วทำ heartbeat แบบครบวงจรหนึ่งรอบ
		if initialTrigger != "" {
			// มี trigger หมายความว่าถึงเวลาสแกนแล้ว
			r.runScan(ctx, initialTrigger)
		}
		// ทำ heartbeat หนึ่งรอบ (รวมถึง flush queue และตอบสนองต่อ server request)
		restart := r.handleBeat(ctx)
		fmt.Fprintln(out, "✓ one-shot run complete")
		// ถ้า self-update เสร็จสิ้น ต้องคืนค่า ErrRestartRequired
		if restart {
			return ErrRestartRequired
		}
		return nil
	}

	// เริ่ม long-running loop พร้อม heartbeat ticker และ scan timer
	return r.serve(ctx, interval, dueIn, initialTrigger)
}

// serve runs the long-lived agent. The heartbeat ticker runs on its own
// goroutine (this one) and never performs slow work: every potentially
// long-running operation — scan, queue flush, self-update — is handed to a
// single background worker. Decoupling them is the whole point: a multi-minute
// scan can no longer starve the heartbeat, so the backend keeps seeing the
// agent as online (it ages to stale→offline after 5 min of silence) and a
// server-requested manual scan is picked up on the very next beat instead of
// waiting for an in-flight scan to finish.
//
// (TH) รัน agent แบบทำงานต่อเนื่องระยะยาว ตัว heartbeat ticker จะรันบน
// goroutine ของตัวเอง (ตัวนี้) และไม่ทำงานหนักใดๆ เลย: งานที่อาจใช้เวลานาน
// ทุกอย่าง — สแกน, flush คิว, self-update — จะถูกส่งไปให้ background worker
// เพียงตัวเดียว การแยกส่วนกันแบบนี้คือหัวใจสำคัญ: การสแกนที่ใช้เวลาหลายนาที
// จะไม่ทำให้ heartbeat ขาดช่วงอีกต่อไป ทำให้ backend ยังเห็นว่า agent ออนไลน์
// อยู่ (จะกลายเป็น stale→offline หลังจากเงียบไป 5 นาที) และคำขอสแกนแบบ manual
// จากเซิร์ฟเวอร์จะถูกรับในรอบ heartbeat ถัดไปทันที โดยไม่ต้องรอให้การสแกนที่
// กำลังทำอยู่เสร็จก่อน
func (r *loop) serve(ctx context.Context, interval, dueIn time.Duration, initialTrigger string) error {
	// แสดงข้อความว่า agent กำลังรัน พร้อมบอก interval และ scan schedule
	fmt.Fprintf(r.out, "Agent running. Heartbeat every %s, auto-scan every %s. Press Ctrl+C to stop.\n",
		interval, r.scanInterval)

	// channel งาน: บัฟเฟอร์ขนาด 1 + ไม่บล็อก (coalesced): มีงานค้างได้ไม่เกินหนึ่งรายการ
	// คำขอที่เข้ามาเป็นชุดขณะ worker กำลังยุ่งจะถูกยุบรวมเหลือเพียงการรันครั้งเดียว
	work := make(chan workItem, 1)
	// channel สัญญาณ restart: บัฟเฟอร์ขนาด 1 เพื่อไม่บล็อก worker ที่ส่ง signal
	restart := make(chan struct{}, 1)

	// เริ่ม background worker goroutine และรอให้มันหยุดก่อน return (defer)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.worker(ctx, interval, work, restart)
	}()
	// รอให้ worker หยุดทำงานก่อนที่ serve จะ return เพื่อป้องกัน goroutine leak
	defer wg.Wait()

	// ส่งงานสแกนตอนเปิดเครื่องผ่าน worker เพื่อไม่ให้ heartbeat ครั้งแรกล่าช้า
	if initialTrigger != "" {
		signalWork(work, workItem{scanTrigger: initialTrigger})
	}

	// สร้าง ticker สำหรับส่ง heartbeat ทุก interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// สร้าง timer สำหรับเรียก auto-scan ทุก scanInterval (เริ่มที่ dueIn)
	scanTimer := time.NewTimer(dueIn)
	defer scanTimer.Stop()

	// ส่ง heartbeat ครั้งแรกทันที เพื่อให้เซิร์ฟเวอร์เห็น agent โดยไม่ต้องรอ interval แรก
	r.beat(ctx, work)
	// event loop หลัก: วนรับ event จาก channel ต่างๆ
	for {
		select {
		case <-ctx.Done():
			// ctx ถูกยกเลิก (Ctrl+C หรือ service stop) → หยุดทำงานแบบนุ่มนวล
			fmt.Fprintln(r.out, "\nShutting down.")
			return nil // หยุดทำงานแบบนุ่มนวล ไม่ถือเป็น error
		case <-restart:
			// worker แจ้งว่า self-update เสร็จสิ้น → คืนค่า ErrRestartRequired ให้ service manager restart
			return ErrRestartRequired
		case <-ticker.C:
			// ถึงรอบส่ง heartbeat ตาม interval ที่กำหนด
			r.beat(ctx, work)
		case <-scanTimer.C:
			// ถึงรอบ auto-scan → ส่งงานให้ worker แล้วรีเซ็ต timer สำหรับรอบถัดไป
			signalWork(work, workItem{scanTrigger: "auto"})
			scanTimer.Reset(r.scanInterval)
		}
	}
}

// workItem is a unit of background work handed from the heartbeat loop to the
// worker. At most one field is set per item.
// (TH) workItem คือหน่วยงานที่ heartbeat loop ส่งให้ worker
// ในแต่ละรายการจะมีค่าตั้งไว้เพียงฟิลด์เดียว (scan หรือ update อย่างใดอย่างหนึ่ง)
type workItem struct {
	scanTrigger string                 // trigger ของการสแกน: "" = ไม่สแกน, "auto"/"manual"/"enroll" = สแกน
	update      *transport.AgentUpdate // ข้อมูล update ที่เสนอมา: nil = ไม่มีการเสนออัปเดต
}

// signalWork enqueues work without ever blocking the heartbeat loop. The
// channel is buffered to 1, so a request arriving while the worker is busy (or
// while one is already pending) is dropped. Nothing is lost: the server
// re-asserts manual_scan_requested / agent_update_available on every heartbeat
// until the agent satisfies it.
//
// (TH) ใส่งานเข้าคิวโดยไม่บล็อก heartbeat loop เลย channel มีบัฟเฟอร์ขนาด 1
// ดังนั้นคำขอที่เข้ามาขณะ worker กำลังยุ่ง (หรือมีงานค้างอยู่แล้ว) จะถูกทิ้งไป
// แต่ไม่มีอะไรสูญหาย เพราะเซิร์ฟเวอร์จะยืนยัน manual_scan_requested /
// agent_update_available ซ้ำในทุก heartbeat จนกว่า agent จะดำเนินการสำเร็จ
func signalWork(ch chan<- workItem, item workItem) {
	// ใช้ select กับ default เพื่อทำ non-blocking send
	// ถ้า channel เต็ม (worker ยุ่งหรือมีงานค้าง) ให้ทิ้ง item นี้ไปโดยไม่บล็อก
	select {
	case ch <- item:
	default:
	}
}

// beat sends one heartbeat — a fast call (10s cap) that must never block on a
// scan — and dispatches any server-requested work to the background worker.
// (TH) ส่ง heartbeat หนึ่งครั้ง เป็นการเรียกที่เร็ว (จำกัดไว้ 10 วินาที)
// ที่ต้องไม่บล็อกรอการสแกนเด็ดขาด และส่งงานที่เซิร์ฟเวอร์ขอมาให้ background worker
func (r *loop) beat(ctx context.Context, work chan<- workItem) {
	// สร้าง context ที่มี timeout 10 วินาทีสำหรับ heartbeat call
	beatCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	// ส่ง heartbeat พร้อมส่ง uptime เป็นวินาที
	resp, err := r.client.Heartbeat(beatCtx, int64(time.Since(r.started).Seconds()))
	cancel() // ยกเลิก context เสมอเพื่อ release resource
	if err != nil {
		// heartbeat ล้มเหลว log ไว้แต่ไม่ crash เพราะเน็ตเวิร์กอาจชั่วคราว
		fmt.Fprintf(r.errw, "heartbeat: %v\n", err)
		return
	}
	// ถ้าเซิร์ฟเวอร์ขอให้สแกน manual ให้ส่งงานสแกนให้ worker
	if resp.ManualScanRequested {
		fmt.Fprintln(r.errw, "→ manual scan requested by server")
		signalWork(work, workItem{scanTrigger: "manual"})
	}
	// ถ้าเซิร์ฟเวอร์มีอัปเดตให้ ส่งข้อมูล update ให้ worker ไปดำเนินการ
	if resp.AgentUpdateAvailable != nil {
		signalWork(work, workItem{update: resp.AgentUpdateAvailable})
	}
}

// worker is the single goroutine that performs every slow operation, keeping
// the heartbeat loop responsive. Work is serialized: one scan/update at a time.
// A periodic tick (at the heartbeat cadence) retries any queued uploads, taking
// over the role flushQueue used to play inside each heartbeat.
//
// (TH) worker คือ goroutine เดียวที่ทำงานหนักทุกอย่าง เพื่อให้ heartbeat loop
// ตอบสนองได้เร็ว งานจะถูกเรียงลำดับทำทีละอย่าง: สแกน/อัปเดตทีละหนึ่ง
// การ tick ตามรอบ (เท่ากับความถี่ heartbeat) จะ retry การอัปโหลดที่ค้างอยู่ในคิว
// ซึ่งมาแทนบทบาทที่ flushQueue เคยทำในทุก heartbeat
func (r *loop) worker(
	ctx context.Context,
	flushInterval time.Duration, // ความถี่ของการ retry queue (เท่ากับ heartbeat interval)
	work <-chan workItem,         // channel รับงานจาก heartbeat loop
	restart chan<- struct{},      // channel ส่งสัญญาณให้ serve restart process
) {
	// สร้าง ticker สำหรับ flush retry queue ตาม interval
	flush := time.NewTicker(flushInterval)
	defer flush.Stop()
	// event loop ของ worker: ทำงานตามที่ได้รับมา
	for {
		select {
		case <-ctx.Done():
			// ctx ถูกยกเลิก → หยุด worker goroutine
			return
		case <-flush.C:
			// ถึงรอบ flush queue → retry การอัปโหลดที่ค้างอยู่
			r.flushQueue(ctx)
		case item := <-work:
			// ได้รับงานใหม่จาก heartbeat loop
			if item.scanTrigger != "" {
				// มี trigger การสแกน → flush queue ก่อนแล้วค่อยสแกน
				r.flushQueue(ctx)
				r.runScan(ctx, item.scanTrigger)
			}
			if item.update != nil && r.maybeUpdate(ctx, item.update) {
				// self-update สำเร็จ ไบนารีตัวใหม่อยู่ในตำแหน่งแล้ว
				// ส่งสัญญาณให้ serve ออกจาก loop เพื่อให้ service manager restart
				select {
				case restart <- struct{}{}:
				default:
					// ถ้า restart channel เต็ม (ไม่น่าเกิด) ให้ทิ้งและ return เลย
				}
				return // หยุด worker หลังส่งสัญญาณ restart
			}
		}
	}
}

// loop bundles the per-run dependencies so the helpers don't need long
// parameter lists.
// (TH) loop รวบรวม dependency ของแต่ละการรัน เพื่อให้ helper method
// ไม่ต้องรับ parameter list ที่ยาวเกินไปในทุกการเรียก
type loop struct {
	client       *transport.Client // HTTP client สำหรับส่ง heartbeat และ scan payload
	queue        *queue.Queue      // retry queue สำหรับการอัปโหลดที่ล้มเหลว
	out          io.Writer         // output writer สำหรับข้อความความคืบหน้าปกติ
	errw         io.Writer         // error writer สำหรับ warning และ error message
	started      time.Time         // เวลาที่ agent เริ่มทำงาน ใช้คำนวณ uptime
	autoUpdate   bool              // ถ้า true: รับ self-update จาก server อัตโนมัติ
	exePath      string            // path ของ executable ตัวเอง ใช้แทนที่ binary ตอน self-update
	version      string            // เวอร์ชันที่ฝังอยู่ใน binary ณ ตอน build (จาก transport.Version)
	scanInterval time.Duration     // ความถี่ของ auto-scan (แปลงมาจาก config.ScanIntervalHours)
}

// runScan performs a scan and, on success, records the scan time so the
// scheduler can compute the next due time across restarts (spec 1.1). Scan
// failures are logged but don't update last_scan_at, so a failed scan will be
// retried at the next opportunity rather than deferred a full interval.
//
// (TH) ทำการสแกน และเมื่อสำเร็จจะบันทึกเวลาที่สแกนไว้ เพื่อให้ scheduler
// คำนวณรอบถัดไปได้แม้จะมีการรีสตาร์ท (ตาม spec 1.1) การสแกนที่ล้มเหลวจะถูก
// บันทึก log แต่ไม่อัปเดต last_scan_at ดังนั้นการสแกนที่ล้มเหลวจะถูก retry ใน
// โอกาสถัดไป แทนที่จะถูกเลื่อนออกไปเต็มรอบ
func (r *loop) runScan(ctx context.Context, trigger string) {
	// สแกนและอัปโหลด ถ้าล้มเหลวให้ log และ return โดยไม่บันทึก last_scan_at
	if err := r.scanAndUpload(ctx, trigger); err != nil {
		fmt.Fprintf(r.errw, "%s scan: %v\n", trigger, err)
		return // ไม่อัปเดต last_scan_at เพื่อให้ retry เร็วขึ้นในรอบถัดไป
	}
	// สแกนสำเร็จ → บันทึกเวลาล่าสุดเพื่อคำนวณรอบถัดไป
	if err := storage.SaveLastScan(time.Now()); err != nil {
		// บันทึก last_scan_at ล้มเหลว: ไม่ fatal แต่ควร log ไว้
		fmt.Fprintf(r.errw, "record last scan time: %v\n", err)
	}
}

// handleBeat does one heartbeat cycle. It returns true when a self-update was
// applied and the process should restart.
// (TH) ทำ heartbeat หนึ่งรอบแบบ synchronous (ใช้ใน one-shot mode)
// จะคืนค่า true เมื่อมีการ self-update เกิดขึ้นและโพรเซสควรรีสตาร์ท
func (r *loop) handleBeat(ctx context.Context) (restart bool) {
	// ระบายผลสแกนที่อัปโหลดล้มเหลวก่อนหน้านี้ออกจากคิว (ตาม spec 1.7)
	r.flushQueue(ctx)

	// สร้าง context ที่มี timeout 10 วินาทีสำหรับ heartbeat call
	beatCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	// ส่ง heartbeat พร้อม uptime
	resp, err := r.client.Heartbeat(beatCtx, int64(time.Since(r.started).Seconds()))
	cancel() // ยกเลิก context เสมอ
	if err != nil {
		// heartbeat ล้มเหลว: log ไว้และ return false (ไม่ต้อง restart)
		fmt.Fprintf(r.errw, "heartbeat: %v\n", err)
		return false
	}
	// ถ้าเซิร์ฟเวอร์ขอให้สแกน manual ให้สแกนทันที (synchronous ใน one-shot mode)
	if resp.ManualScanRequested {
		fmt.Fprintln(r.errw, "→ manual scan requested by server")
		r.runScan(ctx, "manual")
	}
	// ตรวจสอบและ apply self-update ถ้ามี คืนค่า true ถ้า update สำเร็จ
	return r.maybeUpdate(ctx, resp.AgentUpdateAvailable)
}

// maybeUpdate applies a self-update when one is offered, auto-update is enabled,
// and the offered version differs from the running one (spec 1.6). It returns
// true only when the new binary is successfully in place.
//
// (TH) ทำ self-update เมื่อมีการเสนออัปเดตมา, เปิด auto-update ไว้ และเวอร์ชัน
// ที่เสนอแตกต่างจากที่กำลังรันอยู่ (ตาม spec 1.6) จะคืนค่า true ก็ต่อเมื่อไบนารี
// ตัวใหม่อยู่ในตำแหน่งเรียบร้อยแล้วเท่านั้น
func (r *loop) maybeUpdate(ctx context.Context, up *transport.AgentUpdate) bool {
	// ถ้าไม่มีการเสนออัปเดต หรือ auto-update ปิดอยู่ ให้ข้ามไป
	if up == nil || !r.autoUpdate {
		return false
	}
	// ถ้าเวอร์ชันที่เสนอว่าง หรือตรงกับเวอร์ชันที่รันอยู่ ให้ข้ามไป
	if up.Version == "" || up.Version == r.version {
		return false
	}
	// ถ้าไม่รู้ path ของตัวเอง ไม่สามารถแทนที่ binary ได้
	if r.exePath == "" {
		fmt.Fprintln(r.errw, "⚠ update offered but agent path is unknown; skipping")
		return false
	}
	// ตัวตัดวงวน (loop-breaker): ตรวจสอบว่าเราติดตั้งไบนารีตัวนี้ไปแล้วหรือยัง
	// โดยเปรียบเทียบ SHA-256 กับที่บันทึกไว้ครั้งล่าสุด
	// ถ้าตรงกัน แปลว่า manifest version สูงกว่า version ที่ฝังใน binary จริงๆ
	// การติดตั้งซ้ำจะทำให้เกิด restart-loop ไม่รู้จบ จึงต้องข้ามไป
	if last, err := storage.LoadLastUpdate(); err != nil {
		// อ่าน marker ล้มเหลว: log ไว้ แต่ยังดำเนินการ update ต่อ (conservative)
		fmt.Fprintf(r.errw, "read last update marker: %v\n", err)
	} else if up.SHA256 != "" && strings.EqualFold(up.SHA256, last) {
		// SHA-256 ตรงกัน (case-insensitive) → ข้ามเพื่อป้องกัน restart-loop
		fmt.Fprintf(r.errw,
			"⚠ server keeps offering %s but this binary already is that download and still reports %s; "+
				"skipping to avoid a restart loop (fix: manifest version must match the binary's built version)\n",
			up.Version, r.version)
		return false
	}
	// แสดงข้อความว่ากำลัง download update
	fmt.Fprintf(r.errw, "→ update available: %s (running %s); downloading ...\n", up.Version, r.version)
	// สร้าง context ที่มี timeout 5 นาทีสำหรับการ download (ไฟล์อาจมีขนาดใหญ่)
	updCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	// ดาวน์โหลดและแทนที่ binary ด้วย updater.Apply
	if err := updater.Apply(updCtx, r.client, r.exePath, *up); err != nil {
		// update ล้มเหลว: log ไว้และอยู่กับ version ปัจจุบันต่อไป
		fmt.Fprintf(r.errw, "⚠ auto-update failed (staying on %s): %v\n", r.version, err)
		return false
	}
	// บันทึก SHA-256 ของไบนารีที่เพิ่งติดตั้ง เพื่อให้ loop-breaker ตรวจจับได้หลัง restart
	// ป้องกันกรณีที่ manifest version ไม่ตรงกับ version ที่ฝังใน binary
	if err := storage.SaveLastUpdate(up.SHA256); err != nil {
		// บันทึก marker ล้มเหลว: log ไว้ แต่ยังคืนค่า true เพราะ binary อยู่ในตำแหน่งแล้ว
		fmt.Fprintf(r.errw, "record last update marker: %v\n", err)
	}
	// แสดงข้อความยืนยันการ update สำเร็จ
	fmt.Fprintf(r.out, "✓ updated to %s; restarting to apply\n", up.Version)
	return true // คืนค่า true เพื่อให้ caller ทราบว่าต้อง restart process
}

// scanAndUpload runs a local scan and POSTs it to the backend. On upload
// failure the scan is persisted to the retry queue rather than lost (spec 1.7),
// so this returns an error only when the scan itself fails.
//
// (TH) สแกนใน local เครื่องแล้ว POST ไปยัง backend ถ้าอัปโหลดล้มเหลว ผลสแกนจะถูก
// บันทึกลงคิว retry แทนที่จะหายไป (ตาม spec 1.7) ดังนั้นฟังก์ชันนี้จะคืนค่า
// error ก็ต่อเมื่อการสแกนเองล้มเหลวเท่านั้น
func (r *loop) scanAndUpload(ctx context.Context, trigger string) error {
	// จำกัดเวลาสแกนไว้ที่ 2 นาที (สแกน software inventory อาจช้าบน Windows)
	scanCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// แสดงข้อความว่ากำลังสแกน
	fmt.Fprintln(r.errw, "Scanning ...")
	// รันการสแกน software inventory
	res, err := scanner.Run(scanCtx)
	if err != nil {
		// การสแกนล้มเหลว: คืนค่า error เพื่อให้ runScan ไม่อัปเดต last_scan_at
		return fmt.Errorf("scan: %w", err)
	}

	// แปลงผลสแกนเป็น payload สำหรับส่ง
	req := toScanRequest(res, trigger)
	// สร้าง Idempotency-Key ที่คงที่ตลอดทั้งความพยายามครั้งแรกและการ retry ทุกครั้ง
	key := queue.NewKey() // ใช้ key เดิมสำหรับทั้ง live try และ retry เพื่อป้องกันการ insert ซ้ำ

	// จำกัดเวลา upload ไว้ที่ 60 วินาที
	postCtx, cancel2 := context.WithTimeout(ctx, 60*time.Second)
	defer cancel2()
	// POST ผลสแกนไปยัง backend พร้อม Idempotency-Key
	accepted, err := r.client.PostScan(postCtx, req, key)
	if err != nil {
		// อัปโหลดล้มเหลว: บันทึกลงคิว retry เพื่อลองใหม่ภายหลัง
		if qErr := r.queue.Enqueue(req, key); qErr != nil {
			// บันทึกลงคิวไม่ได้ด้วย: คืนค่า error รวมสาเหตุทั้งสอง
			return fmt.Errorf("upload scan failed (%v) and could not queue: %w", err, qErr)
		}
		// บันทึกลงคิวสำเร็จ: log ไว้และคืนค่า nil (ไม่ถือเป็น error ในระดับนี้)
		fmt.Fprintf(r.errw, "⚠ upload failed, queued for retry: %v\n", err)
		return nil
	}
	// อัปโหลดสำเร็จ: แสดงจำนวน software entries และ scan UUID
	fmt.Fprintf(r.errw, "✓ uploaded %d software entries (scan %s)\n",
		len(res.Software), accepted.ScanUUID)
	return nil
}

// flushQueue retries every scan whose backoff has elapsed, removing it on
// success and rescheduling (with longer backoff) on continued failure.
// (TH) retry ผลสแกนทุกรายการที่ครบเวลา backoff แล้ว ถ้าสำเร็จจะลบออกจากคิว
// ถ้ายังล้มเหลวจะเลื่อนเวลาออกไปด้วย backoff ที่นานขึ้น
func (r *loop) flushQueue(ctx context.Context) {
	// ดึงรายการที่ถึงเวลา retry แล้ว ณ เวลาปัจจุบัน
	due, err := r.queue.Due(time.Now())
	if err != nil {
		// อ่านคิวล้มเหลว: log ไว้และ return โดยไม่ทำอะไร
		fmt.Fprintf(r.errw, "read retry queue: %v\n", err)
		return
	}
	// วน retry ทุก item ที่ถึงเวลาแล้ว
	for _, item := range due {
		// สร้าง context ที่มี timeout 60 วินาทีสำหรับแต่ละ upload
		postCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		// POST ผลสแกนด้วย Idempotency-Key เดิม (ป้องกันการ insert ซ้ำ)
		accepted, err := r.client.PostScan(postCtx, item.Request, item.IdempotencyKey)
		cancel() // ยกเลิก context ทันทีหลัง call เสร็จ
		if err != nil {
			// อัปโหลดยังล้มเหลว: เลื่อนเวลา retry ออกไปด้วย backoff ที่นานขึ้น
			if rErr := r.queue.Reschedule(item.ID, time.Now()); rErr != nil {
				// reschedule ล้มเหลว: log ไว้ (item ยังอยู่ในคิวด้วย NextAttempt เดิม)
				fmt.Fprintf(r.errw, "reschedule queued scan: %v\n", rErr)
			}
			continue // ข้ามไปยัง item ถัดไป
		}
		// อัปโหลดสำเร็จ: ลบ item ออกจากคิว
		if rErr := r.queue.Remove(item.ID); rErr != nil {
			// ลบล้มเหลว: log ไว้ (item อาจถูกลบไปแล้วจากที่อื่น หรือไม่มีสิทธิ์)
			fmt.Fprintf(r.errw, "remove queued scan: %v\n", rErr)
		}
		// แสดงข้อความยืนยันการ retry สำเร็จ
		fmt.Fprintf(r.errw, "✓ retried queued scan (scan %s)\n", accepted.ScanUUID)
	}
}

// toScanRequest maps a scanner.Result to the wire payload.
// (TH) แปลง scanner.Result ให้เป็น transport.ScanRequest ซึ่งเป็น payload
// สำหรับส่งผ่านเครือข่ายไปยัง backend
func toScanRequest(res scanner.Result, trigger string) transport.ScanRequest {
	// จอง slice ด้วยความจุเท่ากับจำนวน software รายการ เพื่อลด allocation
	items := make([]transport.SoftwareItem, 0, len(res.Software))
	// วนแปลงแต่ละ software entry จาก scanner format เป็น transport format
	for _, s := range res.Software {
		item := transport.SoftwareItem{
			Name:          s.Name,          // ชื่อซอฟต์แวร์
			Version:       s.Version,       // เวอร์ชัน
			Publisher:     s.Publisher,     // ผู้เผยแพร่/บริษัทที่พัฒนา
			InstallDate:   s.InstallDate,   // วันที่ติดตั้ง
			InstallPath:   s.InstallPath,   // path ที่ติดตั้ง
			InstallSizeKB: s.InstallSizeKB, // ขนาดที่ติดตั้ง (KB)
			Arch:          s.Arch,          // สถาปัตยกรรม (x64/x86/arm64)
			Source:        s.Source,        // แหล่งที่มาของข้อมูล (registry, snap, etc.)
		}
		// ถ้ามีข้อมูล digital signature ให้แปลงด้วย
		if s.Signature != nil {
			item.Signature = &transport.SignatureItem{
				Status:         string(s.Signature.Status),    // สถานะ signature (valid/invalid/unsigned)
				Signer:         s.Signature.Signer,            // ชื่อผู้ลงนาม
				Issuer:         s.Signature.Issuer,            // ผู้ออก certificate
				CertThumbprint: s.Signature.Thumbprint,        // fingerprint ของ certificate
			}
		}
		items = append(items, item)
	}
	// กำหนด scan type: ถ้า trigger เป็น "manual" ให้ใช้ "manual" ไม่อย่างนั้นใช้ "auto"
	scanType := "auto"
	if trigger == "manual" {
		scanType = "manual"
	}
	return transport.ScanRequest{
		StartedAt:   res.StartedAt.UTC().Format(time.RFC3339),   // เวลาเริ่มสแกน (UTC, RFC3339)
		CompletedAt: res.FinishedAt.UTC().Format(time.RFC3339),  // เวลาสิ้นสุดสแกน (UTC, RFC3339)
		ScanType:    scanType,                                    // ประเภทการสแกน ("auto" หรือ "manual")
		Trigger:     trigger,                                     // trigger ดั้งเดิม ("auto"/"manual"/"enroll")
		Software:    items,                                       // รายการซอฟต์แวร์ทั้งหมดที่สแกนได้
	}
}

// writerOrDiscard คืนค่า w ตามที่รับมา ถ้า w เป็น nil จะคืน io.Discard แทน
// เพื่อให้ log call ไม่ต้อง nil-check ทุกครั้ง
func writerOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard // ทิ้ง output ทั้งหมดเมื่อ caller ไม่ต้องการ output
	}
	return w
}
