//go:build windows
// ไฟล์นี้ compile เฉพาะบน Windows เท่านั้น

// Package service จัดการ Windows SCM service สำหรับ SoftSentry agent
package service

import (
	"context"       // ใช้ context.WithCancel สำหรับ cancel runner เมื่อ service หยุด
	"errors"        // ใช้ errors.Is สำหรับตรวจสอบ error type เฉพาะ (ErrRestartRequired)
	"fmt"           // ใช้ fmt.Errorf สำหรับ wrap error พร้อมข้อความอธิบาย
	"io"            // ใช้ io.Discard เป็น fallback writer เมื่อไม่สามารถเปิด log file ได้
	"os"            // ใช้ os.OpenFile สำหรับเปิด log file
	"path/filepath" // ใช้ filepath.Join สำหรับ join path ของ log file
	"time"          // ใช้ time.Second, time.Now, time.Sleep สำหรับ timeout และ retry logic

	"golang.org/x/sys/windows"      // ใช้ Windows API: OpenSCManager, OpenService, CloseServiceHandle
	"golang.org/x/sys/windows/svc" // ใช้ svc.Run, svc.IsWindowsService, svc.Status constants
	"golang.org/x/sys/windows/svc/mgr" // ใช้ mgr.Connect, mgr.Mgr, mgr.Service สำหรับ SCM interaction

	"github.com/softsentry/agent/internal/config" // ใช้ config.Dir() สำหรับดึง config directory
	"github.com/softsentry/agent/internal/runner" // ใช้ runner.Run, runner.Options, runner.ErrRestartRequired
)

// Install ลงทะเบียน SoftSentryAgent SCM service: auto-start, รันเป็น LocalSystem
// (default เมื่อ ServiceStartName ว่างเปล่า), restart เมื่อ fail 3 ครั้ง (spec 1.8)
// ต้องรัน process ในโหมด elevated (admin)
//
// Install เป็น idempotent: ถ้า service มีอยู่แล้ว (re-run หรือ in-place upgrade)
// จะ stop, delete, และ recreate ด้วย cfg.ExePath ใหม่แทนที่จะ fail
// วิธีนี้ทำให้ one-click installer สามารถแทนที่ installation เก่า (อาจเสีย) ได้โดยไม่ต้องทำอะไรเพิ่ม
//
// Parameter:
//   - cfg: Config ที่ระบุ ExePath ของ binary และ Args ที่ต้องการส่ง
//
// Return: error ถ้าการเชื่อมต่อ SCM, การลบ service เก่า, การสร้าง service ใหม่ หรือการ start ล้มเหลว
func Install(cfg Config) error {
	// เชื่อมต่อ Service Control Manager (SCM) ต้องการสิทธิ์ admin
	m, err := mgr.Connect()
	if err != nil {
		// เกิด error เชื่อมต่อ SCM ไม่ได้ อาจเป็นเพราะไม่มีสิทธิ์ admin
		return fmt.Errorf("connect to service manager (run as admin?): %w", err)
	}
	// ปิดการเชื่อมต่อ SCM เสมอเมื่อฟังก์ชันจบ
	defer m.Disconnect()

	// ถ้า service มีอยู่แล้ว ให้ stop และ delete ก่อนเพื่อ recreate ด้วย binary ใหม่
	// CreateService ด้านล่างจะ reject ถ้ามีชื่อซ้ำอยู่แล้ว
	if s, err := m.OpenService(Name); err == nil {
		// พบ service เก่า ให้ stop และลบออกก่อน
		err := stopAndDelete(s)
		// ปิด handle ของ service เก่า
		s.Close()
		if err != nil {
			// เกิด error ระหว่าง stop/delete service เก่า
			return err
		}
	}

	// สร้าง service ใหม่ (มีการ retry ถ้าชื่อยัง "marked for deletion")
	s, err := createService(m, cfg)
	if err != nil {
		return err
	}
	// ปิด handle ของ service ใหม่เสมอเมื่อเสร็จ
	defer s.Close()

	// ตั้งค่า recovery actions: restart สูงสุด 3 ครั้งเมื่อ fail โดย reset counter ทุกวัน
	restart := mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: 5 * time.Second}
	if err := s.SetRecoveryActions(
		[]mgr.RecoveryAction{restart, restart, restart}, // restart 3 ครั้งติดต่อกันได้
		uint32((24 * time.Hour).Seconds()),               // reset failure counter หลังผ่านไป 24 ชั่วโมง
	); err != nil {
		// เกิด error ตั้งค่า recovery actions ไม่ได้
		return fmt.Errorf("set recovery actions: %w", err)
	}

	// Start service ทันทีหลังสร้างเสร็จ
	if err := s.Start(); err != nil {
		// เกิด error สร้าง service สำเร็จแต่ start ไม่ได้
		return fmt.Errorf("service created but failed to start: %w", err)
	}
	return nil
}

// Uninstall หยุด service (ถ้ากำลังรันอยู่) และลบออกจาก SCM
// ต้องรันด้วยสิทธิ์ admin
//
// Return: error ถ้าเชื่อมต่อ SCM ไม่ได้, ไม่พบ service, หรือลบ service ล้มเหลว
func Uninstall() error {
	// เชื่อมต่อ SCM
	m, err := mgr.Connect()
	if err != nil {
		// เกิด error เชื่อมต่อ SCM ไม่ได้ อาจเป็นเพราะไม่มีสิทธิ์ admin
		return fmt.Errorf("connect to service manager (run as admin?): %w", err)
	}
	// ปิดการเชื่อมต่อ SCM เสมอ
	defer m.Disconnect()

	// เปิด handle ของ service ที่ต้องการลบ
	s, err := m.OpenService(Name)
	if err != nil {
		// ไม่พบ service ที่ต้องการลบ
		return fmt.Errorf("service %s is not installed: %w", Name, err)
	}
	// ปิด handle ของ service เสมอ
	defer s.Close()
	// stop service (ถ้ารันอยู่) แล้วลบออก
	return stopAndDelete(s)
}

// stopAndDelete หยุด service ที่กำลังรัน (best-effort, มี timeout เพื่อป้องกัน installer ค้าง
// ถ้า service ไม่ยอม stop) จากนั้นลบออกจาก SCM
// SCM จะ reserve ชื่อไว้จนกว่า handle ทั้งหมดจะปิด ดังนั้น CreateService ที่ตามมา
// อาจ fail ด้วย ERROR_SERVICE_MARKED_FOR_DELETE ชั่วคราว ซึ่ง createService จัดการเรื่องนี้ได้
//
// Parameter:
//   - s: *mgr.Service ที่ต้องการ stop และลบ
//
// Return: error ถ้าลบ service ล้มเหลว
func stopAndDelete(s *mgr.Service) error {
	// ส่ง Stop control ไปยัง service แล้วรอให้หยุดหรือจนครบ deadline 15 วินาที
	if st, err := s.Control(svc.Stop); err == nil {
		// กำหนด deadline รอ service หยุดสูงสุด 15 วินาที
		deadline := time.Now().Add(15 * time.Second)
		// วนรอจนกว่า service จะ Stopped หรือหมด deadline
		for st.State != svc.Stopped && time.Now().Before(deadline) {
			// รอ 300ms ก่อน query สถานะอีกครั้ง เพื่อไม่ให้ busy-wait
			time.Sleep(300 * time.Millisecond)
			// Query สถานะปัจจุบันของ service
			if st, err = s.Query(); err != nil {
				// ถ้า query ล้มเหลว ให้หยุดรอและพยายาม delete ต่อไป
				break
			}
		}
	}
	// ลบ service ออกจาก SCM
	if err := s.Delete(); err != nil {
		// เกิด error ลบ service ไม่ได้
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// createService สร้าง SCM service ใหม่ พร้อม retry ถ้าชื่อยัง "marked for deletion"
// จาก service เก่าที่เพิ่ง delete ไป (SCM จะ free ชื่อก็ต่อเมื่อ handle ทั้งหมดปิดแล้ว
// ซึ่งอาจช้ากว่า Delete call เล็กน้อย) error อื่นๆ คืนทันที
//
// Parameter:
//   - m: *mgr.Mgr ที่เชื่อมต่อ SCM อยู่
//   - cfg: Config ที่ระบุ ExePath และ Args
//
// Return: (*mgr.Service, nil) เมื่อสร้างสำเร็จ, (nil, error) เมื่อล้มเหลว
func createService(m *mgr.Mgr, cfg Config) (*mgr.Service, error) {
	// กำหนด deadline รอสร้าง service สูงสุด 10 วินาที
	deadline := time.Now().Add(10 * time.Second)
	for {
		// พยายามสร้าง service ด้วย config ที่กำหนด
		s, err := m.CreateService(Name, cfg.ExePath, mgr.Config{
			DisplayName:  DisplayName,       // ชื่อที่แสดงใน Services console
			Description:  Description,       // คำอธิบาย service
			StartType:    mgr.StartAutomatic, // เริ่ม auto เมื่อ Windows boot
			ErrorControl: mgr.ErrorNormal,   // log error แต่ continue boot ตามปกติ
		}, cfg.Args...)
		if err == nil {
			// สร้าง service สำเร็จ คืน service handle กลับ
			return s, nil
		}
		// ถ้า error เป็น ERROR_SERVICE_MARKED_FOR_DELETE และยังไม่หมด deadline
		// ให้ retry หลังรอ 300ms
		if errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) && time.Now().Before(deadline) {
			// รอ 300ms ก่อน retry เพื่อให้ SCM มีเวลา free ชื่อ
			time.Sleep(300 * time.Millisecond)
			continue
		}
		// error อื่นหรือหมด deadline แล้ว คืน error ทันที
		return nil, fmt.Errorf("create service: %w", err)
	}
}

// Status รายงานสถานะของ SCM service
// ใช้สิทธิ์แบบ read-only (SC_MANAGER_CONNECT + SERVICE_QUERY_STATUS)
// ทำให้ผู้ใช้ทั่วไป (ไม่ใช่ admin) สามารถ query สถานะได้
// ต่างจาก Install/Uninstall ที่ต้องการ admin
//
// Return:
//   - string: สถานะ service เช่น "running", "stopped", "not installed"
//   - error: เกิดข้อผิดพลาดในการ query
func Status() (string, error) {
	// เปิด SCM ด้วยสิทธิ์ขั้นต่ำ SC_MANAGER_CONNECT เท่านั้น ไม่ต้องการ admin
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		// เกิด error เปิด SCM ไม่ได้
		return "", fmt.Errorf("open service manager: %w", err)
	}
	// ปิด SCM handle เสมอเมื่อเสร็จ
	defer windows.CloseServiceHandle(scm)

	// แปลงชื่อ service เป็น UTF-16 pointer ที่ Windows API ต้องการ
	namePtr, err := windows.UTF16PtrFromString(Name)
	if err != nil {
		// เกิด error แปลง string เป็น UTF-16 ไม่ได้ (ปกติไม่ควรเกิด)
		return "", err
	}
	// เปิด service handle ด้วยสิทธิ์ SERVICE_QUERY_STATUS เท่านั้น
	h, err := windows.OpenService(scm, namePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		// ไม่พบ service หมายความว่ายังไม่ได้ติดตั้ง (ไม่ใช่ error จริง)
		return "not installed", nil //nolint:nilerr // absence is a status, not an error
	}
	// สร้าง mgr.Service wrapper เพื่อใช้ Query() method
	s := &mgr.Service{Name: Name, Handle: h}
	// ปิด service handle เสมอ
	defer s.Close()

	// Query สถานะปัจจุบันของ service
	st, err := s.Query()
	if err != nil {
		// เกิด error query สถานะ service ไม่ได้
		return "", fmt.Errorf("query service: %w", err)
	}
	// แปลง svc.State เป็น string ที่อ่านเข้าใจได้
	return stateString(st.State), nil
}

// RunIfService ตรวจสอบว่า process ถูก start โดย SCM หรือไม่
// ถ้าใช่ จะรัน agent loop ภายใต้ service control handler
// คืน (false, nil) สำหรับ foreground invocation ปกติ เพื่อให้ caller รัน loop ปกติแทน
//
// Parameter:
//   - opts: runner.Options ที่ส่งให้ runner.Run (Out/Err จะถูก override เป็น log file)
//
// Return:
//   - true, nil: รันในฐานะ service สำเร็จ (caller ไม่ต้องทำอะไรเพิ่ม)
//   - false, nil: ไม่ได้รันเป็น service ให้ caller รัน loop ปกติ
//   - false, error: เกิดข้อผิดพลาดในการ detect หรือรัน service
func RunIfService(opts runner.Options) (bool, error) {
	// ตรวจสอบว่า process ถูก start โดย Windows SCM หรือไม่
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		// เกิด error ตรวจสอบ service context ไม่ได้
		return false, fmt.Errorf("detect service context: %w", err)
	}
	// ถ้าไม่ได้รันเป็น service ให้คืน false เพื่อให้ caller รัน loop ปกติ
	if !isSvc {
		return false, nil
	}
	// เปิด log file สำหรับ service (ไม่มี console แนบมา)
	logw := serviceLog()
	// redirect stdout/stderr ของ runner ไปยัง log file
	opts.Out = logw
	opts.Err = logw
	// รัน service control loop ของ Windows พร้อม agent service handler
	return true, svc.Run(Name, &agentService{opts: opts})
}

// agentService เป็น adapter ระหว่าง runner.Run กับ Windows service control protocol
type agentService struct {
	opts runner.Options // runner options ที่จะส่งให้ runner.Run
}

// Execute คือ entry point ของ Windows service ที่ถูกเรียกโดย svc.Run
// จัดการ service lifecycle: รับ control request (Stop/Shutdown) และ relay สถานะกลับ SCM
//
// Parameter:
//   - args: argument จาก SCM (ไม่ได้ใช้)
//   - req: channel รับ ChangeRequest จาก SCM (Stop, Shutdown, Interrogate)
//   - status: channel ส่ง Status กลับ SCM
//
// Return:
//   - ssec: specific exit code flag (ไม่ได้ใช้, คืน false เสมอ)
//   - errno: exit code ส่งให้ SCM; 1 เพื่อ trigger "restart on failure" recovery, 0 สำหรับ normal exit
func (a *agentService) Execute(
	_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status,
) (ssec bool, errno uint32) {
	// ระบุ control ที่ service รองรับ: Stop และ Shutdown
	const accepts = svc.AcceptStop | svc.AcceptShutdown
	// แจ้ง SCM ว่ากำลัง start (ยังไม่พร้อมรับ request)
	status <- svc.Status{State: svc.StartPending}

	// สร้าง cancellable context สำหรับ runner
	ctx, cancel := context.WithCancel(context.Background())
	// cancel context เสมอเมื่อ Execute จบ เพื่อให้ runner หยุดถ้ายังไม่หยุดเอง
	defer cancel()
	// channel สำหรับรับสัญญาณว่า runner จบแล้ว
	done := make(chan struct{})
	var runErr error // เก็บ error จาก runner.Run
	// รัน agent loop ใน goroutine แยก เพื่อให้ Execute สามารถรับ control request ได้พร้อมกัน
	go func() {
		// ปิด done channel เมื่อ runner จบ เพื่อแจ้ง Execute loop
		defer close(done)
		runErr = runner.Run(ctx, a.opts)
	}()

	// แจ้ง SCM ว่า service พร้อมรับ control request แล้ว
	status <- svc.Status{State: svc.Running, Accepts: accepts}
	for {
		select {
		case <-done: // runner จบเองโดยไม่ได้รับ Stop/Shutdown
			// แจ้ง SCM ว่า service หยุดแล้ว
			status <- svc.Status{State: svc.Stopped}
			if errors.Is(runErr, runner.ErrRestartRequired) {
				// Exit non-zero so the SCM "restart on failure" recovery action
				// relaunches the (now updated) binary.
				// runner ออกด้วย ErrRestartRequired (มี update ใหม่) ให้คืน exit 1
				// เพื่อให้ SCM recovery action restart service ด้วย binary ใหม่
				return false, 1
			}
			// runner จบปกติ คืน exit 0
			return false, 0
		case c := <-req: // รับ control request จาก SCM
			switch c.Cmd {
			case svc.Interrogate:
				// SCM ถาม status ปัจจุบัน ให้ตอบด้วย status ล่าสุด
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// ได้รับคำสั่ง Stop หรือ Shutdown จาก SCM
				// แจ้ง SCM ว่ากำลังจะหยุด
				status <- svc.Status{State: svc.StopPending}
				// Cancel context เพื่อให้ runner หยุด
				cancel()
				// รอให้ runner จบจริงๆ ก่อนแจ้ง SCM
				<-done
				// แจ้ง SCM ว่าหยุดแล้ว
				status <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}

// serviceLog เปิด log file สำหรับ service (ไม่มี console แนบมา)
// ถ้าเปิดไม่ได้ให้ใช้ io.Discard แทน (ทิ้ง output ทั้งหมด)
//
// Return: io.Writer ที่ชี้ไปยัง log file หรือ io.Discard ถ้าเปิดไม่ได้
func serviceLog() io.Writer {
	// ดึง config directory สำหรับ agent
	dir, err := config.Dir()
	if err != nil {
		// ไม่สามารถดึง config directory ได้ ให้ discard output
		return io.Discard
	}
	// เปิด log file แบบ append (สร้างใหม่ถ้ายังไม่มี) ด้วย permission 0600
	f, err := os.OpenFile( //nolint:gosec // path derived from config.Dir
		filepath.Join(dir, "agent.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600,
	)
	if err != nil {
		// เปิด log file ไม่ได้ ให้ discard output แทน
		return io.Discard
	}
	// คืน file writer สำหรับเขียน log
	return f
}

// stateString แปลง svc.State (numeric constant) เป็น string ที่อ่านได้
// สำหรับแสดงสถานะ service ให้ผู้ใช้
//
// Parameter:
//   - s: svc.State ที่ได้จาก Service.Query()
//
// Return: string อธิบายสถานะ เช่น "running", "stopped"
func stateString(s svc.State) string {
	// ตรวจสอบทุก state ที่เป็นไปได้และคืน string ที่อ่านเข้าใจง่าย
	switch s {
	case svc.Stopped:
		return "stopped" // service หยุดแล้ว
	case svc.StartPending:
		return "start pending" // service กำลัง start อยู่
	case svc.StopPending:
		return "stop pending" // service กำลังจะหยุด
	case svc.Running:
		return "running" // service รันอยู่ปกติ
	case svc.ContinuePending:
		return "continue pending" // service กำลัง resume จาก pause
	case svc.PausePending:
		return "pause pending" // service กำลัง pause อยู่
	case svc.Paused:
		return "paused" // service ถูก pause แล้ว
	default:
		// state ที่ไม่รู้จัก แสดงค่า numeric เพื่อช่วย debug
		return fmt.Sprintf("unknown (%d)", s)
	}
}
