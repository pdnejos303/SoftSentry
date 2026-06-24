// Package transport ใช้งาน typed HTTP client สำหรับเรียก SoftSentry backend
// รวม helper สำหรับ enroll, heartbeat, scan upload, binary download, และ health check
package transport

import (
	"bytes"         // ใช้สร้าง byte buffer สำหรับ request body
	"context"       // ใช้ส่ง context สำหรับ timeout/cancellation ของแต่ละ HTTP request
	"encoding/json" // ใช้ marshal/unmarshal JSON สำหรับ request และ response body
	"errors"        // ใช้สร้าง sentinel error สำหรับกรณีที่ token ไม่มี
	"fmt"           // ใช้ฟอร์แมต error message และ User-Agent header
	"io"            // ใช้ Reader/Writer interface สำหรับ streaming binary download
	"net/http"      // ใช้ HTTP client และ constants เช่น http.MethodPost
	"runtime"       // ใช้ดึง OS/Arch ปัจจุบันสำหรับ User-Agent header
	"strings"       // ใช้ตัด trailing slash จาก base URL
	"time"          // ใช้กำหนด timeout ของ HTTP client
)

// Version คือ version ของ agent binary ปัจจุบัน เป็น var (ไม่ใช่ const)
// เพื่อให้ release build สามารถ stamp ค่าจริงผ่าน linker flag ได้ เช่น:
//
//	go build -ldflags "-X github.com/softsentry/agent/internal/transport.Version=1.2.0"
//
// ค่านี้ต้องตรงกับ version ที่บันทึกใน manifest.json บน backend สำหรับ binary นี้
// ถ้า manifest ระบุ version สูงกว่าค่าที่ bake ใน binary
// backend จะเสนอ "update" ที่ agent ไม่มีทางทำสำเร็จได้
// ทำให้ restart-download-install วนซ้ำไม่สิ้นสุด
// build tooling (Makefile / build.ps1) จะ stamp ค่านี้และเขียน manifest ที่ตรงกัน
var Version = "0.1.0"

// BuildStamp คือ "ลายนิ้วมือของ build" ที่เปลี่ยนทุกครั้งที่ build (ค่า default = "dev"
// สำหรับ build ใน editor) build-publish.sh จะ stamp เป็น UTC timestamp ของตอน build ผ่าน
//
//	go build -ldflags "-X github.com/softsentry/agent/internal/transport.BuildStamp=20260623T052800Z"
//
// และเขียนค่าเดียวกันลง manifest.json → backend เสิร์ฟต่อ → หน้า deploy โชว์ค่านี้
// installer wizard ก็โชว์ค่านี้บนหน้าแรกด้วย ดังนั้นถ้า "ค่าบนหน้า deploy" = "ค่าบน
// หน้าแรกของตัวที่โหลดมารัน" → ยืนยันได้ 100% ว่าตัวที่โหลด = ตัวที่ build จาก source
// ล่าสุด ไม่ใช่ของเก่าค้าง (เพราะ Version ถูก fix เป็น 0.1.0 เลยใช้เทียบความสดไม่ได้)
var BuildStamp = "dev"

// Client รวม net/http พร้อม timeout และ helper เฉพาะของ SoftSentry
type Client struct {
	baseURL    string       // URL ฐานของ backend เช่น "http://192.168.1.88:47800" (ไม่มี trailing slash)
	bearer     string       // Bearer token สำหรับ Authorization header (ว่างได้สำหรับ call ก่อน enroll)
	httpClient *http.Client // HTTP client พร้อม timeout ที่กำหนดไว้
}

// New สร้าง Client ที่ชี้ไปยัง server ที่กำหนด
// token อาจเป็น empty string สำหรับ call ที่ทำก่อน enroll เช่น health check
func New(serverURL, bearerToken string) *Client {
	return &Client{
		// ตัด trailing slash ออกจาก base URL เพื่อให้ path concatenation สะอาด
		baseURL: strings.TrimRight(serverURL, "/"),
		// เก็บ bearer token สำหรับใส่ใน Authorization header
		bearer: bearerToken,
		// สร้าง HTTP client พร้อม timeout 30 วินาทีเพื่อป้องกัน hang
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// APIError แสดงถึง response ที่มี status code 400 ขึ้นไป (non-2xx)
type APIError struct {
	Status int    // HTTP status code ที่ server ตอบกลับมา
	Body   string // body ของ response เพื่อช่วย debug
}

// Error ใช้งาน error interface สำหรับ APIError
// คืน string อธิบาย status code และ response body
func (e *APIError) Error() string {
	return fmt.Sprintf("server returned %d: %s", e.Status, e.Body)
}

// do เรียก doWithHeaders โดยไม่ส่ง custom headers พิเศษ
// เป็น convenience wrapper สำหรับ call ทั่วไป
func (c *Client) do(
	ctx context.Context, // context สำหรับ timeout/cancellation
	method, path string, // HTTP method และ path ของ endpoint
	body any, // request body (nil ถ้าไม่มี body)
	out any, // pointer สำหรับรับ response JSON (nil ถ้าไม่ต้องการ)
) error {
	// ส่งต่อไปยัง doWithHeaders โดยไม่มี extra headers
	return c.doWithHeaders(ctx, method, path, body, out, nil)
}

// doWithHeaders ส่ง HTTP request พร้อม optional custom headers
// และ decode response JSON ลงใน out ถ้ากำหนดไว้
func (c *Client) doWithHeaders(
	ctx context.Context, // context สำหรับ timeout/cancellation
	method, path string, // HTTP method และ path ของ endpoint
	body any, // request body (nil ถ้าไม่มี body)
	out any, // pointer สำหรับรับ response JSON (nil ถ้าไม่ต้องการ)
	headers map[string]string, // custom headers พิเศษเพิ่มเติม เช่น Idempotency-Key
) error {
	// เตรียม request body โดย marshal เป็น JSON ถ้ามี body
	var bodyReader io.Reader
	if body != nil {
		// แปลง body struct เป็น JSON bytes
		buf, err := json.Marshal(body)
		if err != nil {
			// คืน error ถ้า marshal ล้มเหลว (เช่น field มี type ที่ไม่รองรับ)
			return fmt.Errorf("marshal request: %w", err)
		}
		// สร้าง Reader จาก JSON bytes เพื่อส่งใน request body
		bodyReader = bytes.NewReader(buf)
	}

	// สร้าง HTTP request พร้อม context สำหรับ cancellation
	req, err := http.NewRequestWithContext(
		ctx, method, c.baseURL+path, bodyReader,
	)
	if err != nil {
		// คืน error ถ้าสร้าง request ล้มเหลว (เช่น URL ผิดรูปแบบ)
		return fmt.Errorf("build request: %w", err)
	}
	// กำหนด Accept header ให้ server รู้ว่า client ต้องการ JSON response
	req.Header.Set("Accept", "application/json")
	// กำหนด User-Agent header พร้อม version, OS, และ architecture ของ agent
	req.Header.Set(
		"User-Agent",
		fmt.Sprintf("softsentry-agent/%s (%s/%s)", Version, runtime.GOOS, runtime.GOARCH),
	)
	// กำหนด Content-Type header เฉพาะเมื่อมี request body
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// ใส่ Authorization header ถ้ามี bearer token (skip สำหรับ call ก่อน enroll)
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	// ใส่ custom headers พิเศษ เช่น Idempotency-Key สำหรับ scan upload
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// ส่ง HTTP request ไปยัง server
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// คืน error ถ้า network request ล้มเหลว (เช่น server ไม่ตอบ, timeout)
		return fmt.Errorf("http request: %w", err)
	}
	// ปิด response body เสมอเมื่อ function คืนค่า เพื่อป้องกัน connection leak
	defer resp.Body.Close()

	// อ่าน response body ทั้งหมด
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		// คืน error ถ้าอ่าน response body ล้มเหลว
		return fmt.Errorf("read response: %w", err)
	}

	// ถ้า status code >= 400 แสดงว่า server ส่ง error response
	if resp.StatusCode >= 400 {
		// wrap status code และ body ไว้ใน APIError เพื่อให้ผู้เรียกตรวจสอบได้
		return &APIError{Status: resp.StatusCode, Body: string(data)}
	}
	// decode response JSON ลงใน out ถ้ากำหนดไว้และมีข้อมูล
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			// คืน error ถ้า decode JSON ล้มเหลว (response format ไม่ตรงกับที่คาดหวัง)
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// EnrollRequest คือ payload ที่ agent ส่งไปยัง server เพื่อ enroll
type EnrollRequest struct {
	EnrollmentToken string `json:"enrollment_token"`      // one-time token จาก deployment package
	Hostname        string `json:"hostname"`              // ชื่อเครื่อง endpoint
	OS              string `json:"os"`                    // ระบบปฏิบัติการ เช่น "windows", "darwin"
	OSVersion       string `json:"os_version"`            // version ของ OS เช่น "10.0.26200"
	Arch            string `json:"arch"`                  // architecture เช่น "amd64", "arm64"
	AgentVersion    string `json:"agent_version"`         // version ของ agent binary
	Fingerprint     string `json:"fingerprint,omitempty"` // hardware id ที่คงที่ ใช้ dedup การ enroll ซ้ำ (ละเว้นถ้าว่าง)
}

// EnrollResponse คือ reply ที่ server ส่งกลับมาเมื่อ enroll สำเร็จ
type EnrollResponse struct {
	MachineUUID       string `json:"machine_uuid"`        // UUID ของเครื่องนี้ในระบบ SoftSentry
	AgentToken        string `json:"agent_token"`         // permanent bearer token สำหรับ call ถัดไป
	ScanIntervalHours int    `json:"scan_interval_hours"` // ความถี่การสแกนเป็นชั่วโมง
}

// Enroll แลก one-time enrollment token กับ permanent agent token
// เรียก POST /api/v1/agents/enroll และคืน EnrollResponse
func (c *Client) Enroll(ctx context.Context, req EnrollRequest) (*EnrollResponse, error) {
	// เตรียม struct สำหรับรับ response
	var out EnrollResponse
	// ส่ง POST request ไปยัง enroll endpoint
	if err := c.do(ctx, http.MethodPost, "/api/v1/agents/enroll", req, &out); err != nil {
		// คืน error ถ้า enroll ล้มเหลว (เช่น token หมดอายุหรือใช้ไปแล้ว)
		return nil, err
	}
	return &out, nil
}

// AgentUpdate อธิบาย binary ของ agent เวอร์ชันใหม่ที่ server เสนอให้
// ได้รับมาจาก heartbeat response (agent_update_available) หรือ GET binary/latest
type AgentUpdate struct {
	Version     string `json:"version"`      // version string ของ binary ใหม่ เช่น "0.2.0"
	DownloadURL string `json:"download_url"` // path สำหรับดาวน์โหลด binary เช่น "/agent-binaries/..."
	SHA256      string `json:"sha256"`       // checksum SHA-256 ของ binary สำหรับยืนยันความถูกต้อง
	// Forced = true เมื่อ admin สั่ง "Update agent now" จาก dashboard: agent ต้อง
	// apply แม้ปิด auto-update / version เท่าเดิม / เคยลง SHA นี้แล้ว (loop-breaker)
	// แต่ยังตรวจ SHA-256 เสมอ. ปกติ (version-compare path) field นี้ไม่มา = false
	Forced bool `json:"forced"`
}

// HeartbeatResponse รวม flag ต่างๆ ที่ server ตอบกลับมาจาก heartbeat (protocol §2)
// AgentUpdateAvailable เป็น nil เมื่อ server ไม่มี update ให้
type HeartbeatResponse struct {
	ConfigChanged        bool         `json:"config_changed"`         // true ถ้า config เปลี่ยนและต้อง reload
	ManualScanRequested  bool         `json:"manual_scan_requested"`  // true ถ้า IT admin กด scan ทันที
	AgentUpdateAvailable *AgentUpdate `json:"agent_update_available"` // ข้อมูล update ถ้ามี หรือ nil
}

// ScanProgress คือ snapshot ความคืบหน้าการสแกนที่แนบไปกับ heartbeat เพื่อให้
// dashboard แสดง progress bar สด ตรงกับ backend ScanProgressIn schema
// (nil = ไม่มีการสแกนอยู่ จึงไม่แนบ field นี้)
type ScanProgress struct {
	Phase       string `json:"phase"`                  // idle/counting/scanning/verifying/uploading
	Done        int    `json:"done"`                   // จำนวนที่ทำเสร็จแล้ว
	Total       int    `json:"total"`                  // จำนวนทั้งหมด
	CurrentPath string `json:"current_path,omitempty"` // ไฟล์ที่กำลังประมวลผล
	UpdatedAt   string `json:"updated_at,omitempty"`   // เวลาอัปเดตล่าสุด (RFC3339)
}

// Heartbeat ส่ง liveness ping ไปยัง server และรับ flag ตอบกลับมา
// ต้องการ bearer token และคืน error ถ้าไม่มี
// progress (อาจเป็น nil) แนบ snapshot ความคืบหน้าการสแกนเพื่อให้ dashboard แสดงผลสด
func (c *Client) Heartbeat(ctx context.Context, uptimeSeconds int64, progress *ScanProgress) (*HeartbeatResponse, error) {
	// ตรวจสอบว่ามี bearer token ก่อน heartbeat ต้องการ authentication
	if c.bearer == "" {
		return nil, errors.New("heartbeat requires bearer token")
	}
	// สร้าง request body พร้อม agent version, uptime, และ server_url ที่ agent คุยด้วย
	// (= baseURL จาก config) เพื่อให้ dashboard แสดงปลายทางจริงที่แต่ละ agent รายงานเข้า
	body := map[string]any{
		"agent_version":  Version,
		"uptime_seconds": uptimeSeconds,
		"server_url":     c.baseURL,
	}
	// แนบ progress เฉพาะตอนมีการสแกนอยู่ (ไม่งั้นเว้นไว้ให้ backend คงค่าเดิม/idle)
	if progress != nil {
		body["scan_progress"] = progress
	}
	// เตรียม struct สำหรับรับ response
	var out HeartbeatResponse
	// ส่ง POST request ไปยัง heartbeat endpoint
	if err := c.do(ctx, http.MethodPost, "/api/v1/agents/heartbeat", body, &out); err != nil {
		// คืน error ถ้า heartbeat ล้มเหลว (เช่น network ขาด, token หมดอายุ)
		return nil, err
	}
	return &out, nil
}

// SignatureItem ตรงกับ backend SignatureIn schema
// เก็บข้อมูล digital signature ของ software
type SignatureItem struct {
	Status             string      `json:"status"`                        // สถานะ signature เช่น "valid", "invalid", "unsigned", "unknown"
	Signer             string      `json:"signer,omitempty"`              // ชื่อผู้ลงนาม (ถ้ามี)
	Issuer             string      `json:"issuer,omitempty"`              // ชื่อ CA ที่ออก certificate (ถ้ามี)
	CertThumbprint     string      `json:"cert_thumbprint,omitempty"`     // SHA-1 fingerprint ของ leaf cert (ถ้ามี)
	CertValidFrom      string      `json:"cert_valid_from,omitempty"`     // วันเริ่มมีผลของ leaf cert (YYYY-MM-DD)
	CertValidTo        string      `json:"cert_valid_to,omitempty"`       // วันหมดอายุของ leaf cert (YYYY-MM-DD)
	SignatureAlgorithm string      `json:"signature_algorithm,omitempty"` // algorithm ที่ใช้เซ็น เช่น sha256RSA
	Chain              []ChainNode `json:"chain,omitempty"`               // certificate chain (leaf เป็นตัวแรก)
}

// ChainNode ตรงกับ element ใน backend SignatureIn.chain
// เก็บข้อมูล certificate หนึ่งใบใน chain
type ChainNode struct {
	Subject   string `json:"subject,omitempty"`    // subject CN ของใบรับรองนี้
	Issuer    string `json:"issuer,omitempty"`     // issuer CN
	ValidFrom string `json:"valid_from,omitempty"` // วันเริ่มมีผล (YYYY-MM-DD)
	ValidTo   string `json:"valid_to,omitempty"`   // วันหมดอายุ (YYYY-MM-DD)
}

// SoftwareItem ตรงกับ backend SoftwareIn schema
// เก็บข้อมูล software รายการหนึ่งที่สแกนได้
type SoftwareItem struct {
	Name          string         `json:"name"`                      // ชื่อ software
	Version       string         `json:"version"`                   // version ของ software
	Publisher     string         `json:"publisher,omitempty"`       // ผู้พัฒนา/ผู้จำหน่าย (ถ้ามี)
	InstallDate   string         `json:"install_date,omitempty"`    // วันที่ติดตั้งในรูปแบบ YYYYMMDD (ถ้ามี)
	InstallPath   string         `json:"install_path,omitempty"`    // path ที่ติดตั้ง (ถ้ามี)
	InstallSizeKB int64          `json:"install_size_kb,omitempty"` // ขนาดที่ติดตั้งเป็น KB (ถ้ามี)
	Arch          string         `json:"arch,omitempty"`            // architecture เช่น "x64", "x86" (ถ้ามี)
	Source        string         `json:"source"`                    // แหล่งที่มาของข้อมูล เช่น "registry", "applications"
	Signature     *SignatureItem `json:"signature,omitempty"`       // ข้อมูล digital signature (ถ้ามี)
}

// ScanRequest ตรงกับ backend ScanIn schema (POST /api/v1/agents/scans)
// รวมข้อมูล metadata ของการสแกนและรายการ software ทั้งหมด
type ScanRequest struct {
	StartedAt   string         `json:"started_at"`        // เวลาเริ่มสแกนในรูปแบบ RFC3339
	CompletedAt string         `json:"completed_at"`      // เวลาสแกนเสร็จในรูปแบบ RFC3339
	ScanType    string         `json:"scan_type"`         // ประเภทการสแกน เช่น "full"
	Trigger     string         `json:"trigger,omitempty"` // สาเหตุที่สแกน เช่น "scheduled", "manual" (ถ้ามี)
	Software    []SoftwareItem `json:"software"`          // รายการ software ทั้งหมดที่สแกนได้
	Device      *DeviceInfo    `json:"device,omitempty"`  // ข้อมูลฮาร์ดแวร์ + Windows Update (omit ถ้าเก็บไม่ได้)
}

// DeviceInfo ตรงกับ backend DeviceIn schema — ข้อมูลฮาร์ดแวร์/สเปคและสถานะ
// Windows Update ของเครื่อง (typed mirror ของ device.Info ฝั่ง agent)
type DeviceInfo struct {
	System        DeviceSystem    `json:"system"`                   // รุ่น/ผู้ผลิต/serial/RAM รวม
	CPU           DeviceCPU       `json:"cpu"`                      // รุ่น/cores/clock
	Memory        DeviceMemory    `json:"memory"`                   // RAM รวม + ราย DIMM
	Disks         []DeviceDisk    `json:"disks,omitempty"`          // ไดรฟ์เก็บข้อมูล
	GPUs          []DeviceGPU     `json:"gpus,omitempty"`           // การ์ดจอ
	Network       []DeviceNIC     `json:"network,omitempty"`        // การ์ดเครือข่าย (MAC)
	Firmware      DeviceFirmware  `json:"firmware"`                 // BIOS/UEFI + motherboard
	Security      DeviceSecurity  `json:"security"`                 // Secure Boot + TPM
	Battery       *DeviceBattery  `json:"battery,omitempty"`        // แบต (nil ถ้าเป็น desktop)
	Monitors      []DeviceMonitor `json:"monitors,omitempty"`       // จอที่เชื่อมต่อ
	WindowsUpdate *DeviceWU       `json:"windows_update,omitempty"` // สถานะ Windows Update
}

// DeviceSystem mirror ของ device.System
type DeviceSystem struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
	SystemType   string `json:"system_type,omitempty"`
	Domain       string `json:"domain,omitempty"`
	TotalRAMMB   int64  `json:"total_ram_mb,omitempty"`
}

// DeviceCPU mirror ของ device.CPU
type DeviceCPU struct {
	Model        string `json:"model,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Cores        int    `json:"cores,omitempty"`
	LogicalCount int    `json:"logical_count,omitempty"`
	ClockMHz     int    `json:"clock_mhz,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

// DeviceMemory mirror ของ device.Memory
type DeviceMemory struct {
	TotalMB int64                `json:"total_mb,omitempty"`
	Modules []DeviceMemoryModule `json:"modules,omitempty"`
}

// DeviceMemoryModule mirror ของ device.MemoryModule
type DeviceMemoryModule struct {
	CapacityMB   int64  `json:"capacity_mb,omitempty"`
	SpeedMHz     int    `json:"speed_mhz,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	PartNumber   string `json:"part_number,omitempty"`
	Slot         string `json:"slot,omitempty"`
}

// DeviceDisk mirror ของ device.Disk
type DeviceDisk struct {
	Model         string `json:"model,omitempty"`
	SizeGB        int64  `json:"size_gb,omitempty"`
	MediaType     string `json:"media_type,omitempty"`
	InterfaceType string `json:"interface_type,omitempty"`
	Serial        string `json:"serial,omitempty"`
}

// DeviceGPU mirror ของ device.GPU
type DeviceGPU struct {
	Name      string `json:"name,omitempty"`
	DriverVer string `json:"driver_version,omitempty"`
	VRAMMB    int64  `json:"vram_mb,omitempty"`
}

// DeviceNIC mirror ของ device.NetworkAdapter
type DeviceNIC struct {
	Name string `json:"name,omitempty"`
	MAC  string `json:"mac,omitempty"`
	Type string `json:"type,omitempty"`
}

// DeviceFirmware mirror ของ device.Firmware
type DeviceFirmware struct {
	BIOSVendor  string `json:"bios_vendor,omitempty"`
	BIOSVersion string `json:"bios_version,omitempty"`
	BIOSDate    string `json:"bios_date,omitempty"`
	Motherboard string `json:"motherboard,omitempty"`
	BoardSerial string `json:"board_serial,omitempty"`
}

// DeviceSecurity mirror ของ device.Security
type DeviceSecurity struct {
	SecureBoot string `json:"secure_boot,omitempty"`
	TPMPresent bool   `json:"tpm_present"`
	TPMEnabled bool   `json:"tpm_enabled"`
	TPMVersion string `json:"tpm_version,omitempty"`
}

// DeviceBattery mirror ของ device.Battery
type DeviceBattery struct {
	Name          string `json:"name,omitempty"`
	ChargePercent int    `json:"charge_percent,omitempty"`
	Status        string `json:"status,omitempty"`
}

// DeviceMonitor mirror ของ device.Monitor
type DeviceMonitor struct {
	Name   string `json:"name,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// DeviceWU mirror ของ device.WindowsUpdate
type DeviceWU struct {
	Status          string             `json:"status"`
	PendingCount    int                `json:"pending_count"`
	SecurityPending int                `json:"security_pending"`
	RebootPending   bool               `json:"reboot_pending"`
	LastInstalledKB string             `json:"last_installed_kb,omitempty"`
	LastInstalledAt string             `json:"last_installed_at,omitempty"`
	LastCheckedAt   string             `json:"last_checked_at,omitempty"`
	Source          string             `json:"source,omitempty"`
	Pending         []DevicePendingKB  `json:"pending,omitempty"`
}

// DevicePendingKB mirror ของ device.PendingUpdate
type DevicePendingKB struct {
	KB       string `json:"kb,omitempty"`
	Title    string `json:"title,omitempty"`
	Security bool   `json:"security"`
	Severity string `json:"severity,omitempty"`
}

// ScanAccepted คือ response 202 ที่ server ตอบกลับมาหลังรับ scan สำเร็จ
// รวม UUID ของ scan record บน server
type ScanAccepted struct {
	ScanUUID string `json:"scan_uuid"` // UUID ของ scan record สำหรับ tracking
}

// PostScan upload ผลการสแกนไปยัง server
// idempotencyKey ถ้ากำหนดจะให้ server de-duplicate request ซ้ำภายใน 24 ชั่วโมง (protocol "Idempotency")
func (c *Client) PostScan(
	ctx context.Context, req ScanRequest, idempotencyKey string,
) (*ScanAccepted, error) {
	// ตรวจสอบว่ามี bearer token ก่อน scan upload ต้องการ authentication
	if c.bearer == "" {
		return nil, errors.New("scan upload requires bearer token")
	}
	// เตรียม struct สำหรับรับ response
	var out ScanAccepted
	// เตรียม headers map สำหรับใส่ Idempotency-Key ถ้ามี
	headers := map[string]string{}
	// ใส่ Idempotency-Key header ถ้ากำหนดไว้ เพื่อให้ server de-dup retry ได้
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	// ส่ง POST request ไปยัง scans endpoint พร้อม custom headers
	if err := c.doWithHeaders(
		ctx, http.MethodPost, "/api/v1/agents/scans", req, &out, headers,
	); err != nil {
		// คืน error ถ้า upload ล้มเหลว (เช่น network ขาด, server error)
		return nil, err
	}
	return &out, nil
}

// GetBinary download agent binary จาก server path ที่กำหนด (เช่น download_url จาก AgentUpdate)
// ส่ง data ไปยัง w โดยตรง (streaming) ใช้โดย self-updater
func (c *Client) GetBinary(ctx context.Context, path string, w io.Writer) error {
	// ตรวจสอบว่ามี bearer token ก่อน binary download ต้องการ authentication
	if c.bearer == "" {
		return errors.New("binary download requires bearer token")
	}
	// สร้าง HTTP GET request พร้อม context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		// คืน error ถ้าสร้าง request ล้มเหลว (เช่น URL ผิดรูปแบบ)
		return fmt.Errorf("build request: %w", err)
	}
	// ใส่ Authorization header สำหรับ authenticated download
	req.Header.Set("Authorization", "Bearer "+c.bearer)
	// ส่ง HTTP request ไปยัง server
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// คืน error ถ้า network request ล้มเหลว
		return fmt.Errorf("http request: %w", err)
	}
	// ปิด response body เสมอเมื่อ function คืนค่า
	defer resp.Body.Close()
	// ถ้า status code >= 400 อ่าน body บางส่วนเพื่อ debug แล้วคืน APIError
	if resp.StatusCode >= 400 {
		// จำกัดการอ่าน body เพื่อป้องกัน OOM จาก response ขนาดใหญ่ที่ผิดปกติ
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &APIError{Status: resp.StatusCode, Body: string(data)}
	}
	// stream response body ไปยัง w โดยตรง โดยไม่ buffer ทั้งหมดในหน่วยความจำ
	if _, err := io.Copy(w, resp.Body); err != nil {
		// คืน error ถ้า streaming ล้มเหลว (เช่น disk เต็ม, network ขาด)
		return fmt.Errorf("stream binary: %w", err)
	}
	return nil
}

// Health ping endpoint /health ที่ไม่ต้องการ authentication
// ใช้ตรวจสอบว่า backend พร้อมรับ request หรือไม่
func (c *Client) Health(ctx context.Context) (map[string]string, error) {
	// เตรียม map สำหรับรับ response JSON เช่น {"status": "ok"}
	out := map[string]string{}
	// ส่ง GET request ไปยัง /health โดยไม่มี body
	if err := c.do(ctx, http.MethodGet, "/health", nil, &out); err != nil {
		// คืน error ถ้า health check ล้มเหลว (เช่น backend ยังไม่พร้อม)
		return nil, err
	}
	return out, nil
}
