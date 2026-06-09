//go:build windows
// build tag นี้บอกให้รวมไฟล์นี้เฉพาะเมื่อ build สำหรับ Windows เท่านั้น

// Package scanner จัดการการตรวจสอบลายเซ็น Authenticode บน Windows
// โดยเรียก WinVerifyTrust และ Crypt32 API โดยตรงผ่าน syscall
package scanner

import (
	"encoding/hex"  // ใช้แปลง SHA-1 thumbprint เป็น hex string
	"path/filepath" // ใช้แปลง path เป็น absolute path
	"runtime"       // ใช้ KeepAlive เพื่อป้องกัน GC เก็บ pointer ก่อนเวลา
	"strings"       // ใช้ตัดแต่ง string และตรวจสอบนามสกุล .exe
	"sync"          // ใช้ Mutex สำหรับป้องกัน race condition ใน cache
	"time"          // ใช้แปลง FILETIME ของ cert เป็นวันที่ YYYY-MM-DD
	"unsafe"        // ใช้แปลง Go struct เป็น pointer ส่งให้ Win32 API

	"golang.org/x/sys/windows" // ใช้ windows.GUID, windows.Handle, typed CertContext และ UTF16 utility
)

// authenticodeVerifier ห่อหุ้ม WinVerifyTrust พร้อม path cache ต่อหนึ่งการสแกน
// ทำให้ executable เดิมถูกตรวจสอบเพียงครั้งเดียว ไม่ว่าจะปรากฏกี่ครั้ง
// (spec 1.4 performance note)
type authenticodeVerifier struct {
	mu    sync.Mutex          // mutex ป้องกัน race condition เมื่อ goroutine หลายตัวเข้าถึง cache
	cache map[string]*Signature // cache เก็บผลลัพธ์การตรวจสอบ โดยใช้ absolute path เป็น key
}

// newAuthenticodeVerifier สร้าง authenticodeVerifier ใหม่พร้อม cache ว่าง
// Return:
//   - *authenticodeVerifier: verifier ที่พร้อมใช้งาน
func newAuthenticodeVerifier() *authenticodeVerifier {
	return &authenticodeVerifier{cache: make(map[string]*Signature)}
}

// Lazy-load DLL ที่ต้องการ — NewLazySystemDLL โหลดจาก System32
// เพื่อป้องกัน DLL hijacking จากไฟล์ในโฟลเดอร์ working directory
var (
	modWintrust          = windows.NewLazySystemDLL("wintrust.dll")                          // DLL สำหรับตรวจสอบ Authenticode signature
	procWinVerifyTrust   = modWintrust.NewProc("WinVerifyTrust")                             // ฟังก์ชันหลักในการตรวจสอบลายเซ็น Authenticode
	modCrypt32           = windows.NewLazySystemDLL("crypt32.dll")                           // DLL สำหรับจัดการ certificate และ crypto
	procCryptQueryObject = modCrypt32.NewProc("CryptQueryObject")                            // เปิด PKCS#7 store จากไฟล์
	procCryptMsgGetParam = modCrypt32.NewProc("CryptMsgGetParam")                            // ดึง parameter จาก PKCS#7 message
	procCertGetNameStr   = modCrypt32.NewProc("CertGetNameStringW")                          // แปลง subject/issuer ของ cert เป็น string
	procCertCloseStore   = modCrypt32.NewProc("CertCloseStore")                              // คืนหน่วยความจำ HCERTSTORE
	procCryptMsgClose    = modCrypt32.NewProc("CryptMsgClose")                               // คืนหน่วยความจำ HCRYPTMSG
	procCertGetCtxProp   = modCrypt32.NewProc("CertGetCertificateContextProperty")           // ดึง property เช่น SHA-1 hash ของ cert
)

// actionGenericVerifyV2 คือ GUID ของ Authenticode policy มาตรฐาน
// WINTRUST_ACTION_GENERIC_VERIFY_V2 — ใช้กับ WinVerifyTrust เสมอ
var actionGenericVerifyV2 = windows.GUID{
	Data1: 0xaac56b,                                         // ส่วนแรกของ GUID (32-bit)
	Data2: 0xcd44,                                           // ส่วนที่สองของ GUID (16-bit)
	Data3: 0x11d0,                                           // ส่วนที่สามของ GUID (16-bit)
	Data4: [8]byte{0x8c, 0xc2, 0x00, 0xc0, 0x4f, 0xc2, 0x95, 0xee}, // ส่วนสุดท้ายของ GUID (8 bytes)
}

const (
	// wtdUINone = WTD_UI_NONE — ปิด UI dialog ทุกชนิดระหว่างการตรวจสอบ
	wtdUINone = 2
	// wtdRevokeNone = WTD_REVOKE_NONE — ข้ามการตรวจสอบ certificate revocation ออนไลน์
	// ยอมรับได้สำหรับ inventory scan เพราะหลีกเลี่ยง network call ต่อหนึ่ง binary
	wtdRevokeNone = 0
	// wtdChoiceFile = WTD_CHOICE_FILE — ระบุว่า union member ใน WINTRUST_DATA คือ WINTRUST_FILE_INFO
	wtdChoiceFile = 1
	// wtdStateActionVerify = WINTRUST_ACTION_VERIFY — เริ่มการตรวจสอบและสร้าง state
	wtdStateActionVerify = 1
	// wtdStateActionClose = WINTRUST_ACTION_CLOSE — ปิดและคืน state หน่วยความจำ
	wtdStateActionClose = 2
	// wtdSaferFlag = WTD_SAFER_FLAG — รัน SAFER (Software Restriction Policy) check ด้วย
	wtdSaferFlag = 0x100

	// HRESULTs ที่ WinVerifyTrust คืนค่ากลับมา (winerror.h / MSDN)
	trustENoSignature = 0x800B0100 // TRUST_E_NOSIGNATURE — ไฟล์ไม่มีลายเซ็นฝังอยู่
	trustECertExpired = 0x800B0101 // CERT_E_EXPIRED — ใบรับรองผู้ลงนามหมดอายุแล้ว

	// HRESULTs ที่หมายถึง "ตรวจสอบไม่ได้" (ไม่ใช่ลายเซ็นเสีย) → map เป็น unknown
	errFileNotFound          = 0x80070002 // ERROR_FILE_NOT_FOUND — ไฟล์หาย (เช่น ถูก uninstall)
	errPathNotFound          = 0x80070003 // ERROR_PATH_NOT_FOUND — โฟลเดอร์หาย
	errSharingViolation      = 0x80070020 // ERROR_SHARING_VIOLATION — ไฟล์ถูกล็อก/กำลังใช้งาน
	cryptEFileError          = 0x80092003 // CRYPT_E_FILE_ERROR — อ่านไฟล์ไม่ได้
	trustEProviderUnknown    = 0x800B0001 // TRUST_E_PROVIDER_UNKNOWN — ไม่มี provider ที่รองรับ
	trustESubjectFormUnknown = 0x800B0003 // TRUST_E_SUBJECT_FORM_UNKNOWN — รูปแบบไฟล์ที่ตรวจไม่ได้
)

// wintrustFileInfo mirrors Win32 struct WINTRUST_FILE_INFO
// ลำดับ field และขนาดต้องตรงกับ C layout ทุกประการ
// เพราะส่ง pointer นี้โดยตรงไปยัง WinVerifyTrust ผ่าน unsafe
type wintrustFileInfo struct {
	cbStruct       uint32         // sizeof(WINTRUST_FILE_INFO) — ต้องกำหนดก่อนเรียก
	pcwszFilePath  *uint16        // pointer ไปยัง UTF-16 path ของไฟล์ที่ต้องการตรวจสอบ
	hFile          windows.Handle // handle ของไฟล์ที่เปิดแล้ว (0 = เปิดจาก path อัตโนมัติ)
	pgKnownSubject *windows.GUID  // override ประเภท subject (nil = ตรวจจับอัตโนมัติ)
}

// wintrustData mirrors Win32 struct WINTRUST_DATA
// ลำดับ field และขนาดต้องตรงกับ C layout ทุกประการ
type wintrustData struct {
	cbStruct            uint32         // sizeof(WINTRUST_DATA)
	pPolicyCallbackData uintptr        // policy-specific callback (nil = ไม่ใช้)
	pSIPClientData      uintptr        // SIP client data (nil = ไม่ใช้)
	dwUIChoice          uint32         // WTD_UI_NONE — ปิด dialog ทั้งหมด
	fdwRevocationChecks uint32         // WTD_REVOKE_NONE — ข้าม CRL/OCSP เพื่อไม่ต้องเชื่อมต่อเน็ต
	dwUnionChoice       uint32         // WTD_CHOICE_FILE — union member คือ WINTRUST_FILE_INFO
	pFile               uintptr        // pointer ไปยัง wintrustFileInfo
	dwStateAction       uint32         // VERIFY ก่อน แล้ว CLOSE เพื่อคืน state
	hWVTStateData       windows.Handle // opaque state — ต้องส่งกลับใน CLOSE call
	pwszURLReference    *uint16        // ไม่ใช้ (nil)
	dwProvFlags         uint32         // WTD_SAFER_FLAG — รัน Software Restriction Policy ด้วย
	dwUIContext          uint32        // ไม่ใช้ (0)
	pSignatureSettings  uintptr        // WINTRUST_SIGNATURE_SETTINGS* — nil ใช้ค่า default
}

// verify คืนผลการตรวจสอบลายเซ็นสำหรับ path ที่กำหนด
// ถ้า path เป็นโฟลเดอร์หรือไม่ใช่ .exe จะคืน nil (ไม่มีข้อมูล)
// ผลลัพธ์จะถูก cache ตาม absolute path เพื่อหลีกเลี่ยงการตรวจสอบซ้ำ
// Parameter:
//   - path: path ของไฟล์ที่ต้องการตรวจสอบ
//
// Return:
//   - *Signature: ผลการตรวจสอบ หรือ nil ถ้าไม่ใช่ executable ที่ตรวจสอบได้
func (v *authenticodeVerifier) verify(path string) *Signature {
	// ตัด quote และ whitespace รอบๆ path ออก
	path = strings.Trim(strings.TrimSpace(path), `"`)

	// ตรวจสอบเฉพาะไฟล์ PE (exe/dll/sys/...) — path ว่างหรือไม่ใช่ PE ให้คืน nil
	// (โฟลเดอร์ data-only package → ไม่มีไฟล์ให้ตรวจ → signature_id = NULL ตาม spec edge case 2)
	if !isVerifiablePE(path) {
		return nil // ไม่ใช่ไฟล์ที่ตรวจ Authenticode ได้ — ข้ามไป
	}

	// แปลง path เป็น absolute path เพื่อใช้เป็น cache key ที่แน่นอน
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil // แปลง path ล้มเหลว — ข้ามไป
	}

	// ตรวจสอบ cache ก่อน (ล็อก mutex เพื่อ thread safety)
	v.mu.Lock()
	if cached, ok := v.cache[abs]; ok {
		v.mu.Unlock()
		return cached // คืนผลลัพธ์จาก cache ถ้ามีอยู่แล้ว
	}
	v.mu.Unlock() // ปลดล็อกก่อนเรียก run ที่อาจใช้เวลานาน

	// ตรวจสอบลายเซ็นจริงๆ
	sig := v.run(abs)

	// บันทึกผลลัพธ์เข้า cache (ล็อก mutex อีกครั้ง)
	v.mu.Lock()
	v.cache[abs] = sig // เก็บผลลัพธ์เข้า cache สำหรับการเรียกครั้งต่อไป
	v.mu.Unlock()
	return sig // คืนผลการตรวจสอบ
}

// run เรียก WinVerifyTrust สำหรับ path ที่กำหนดและแปลงผลลัพธ์เป็น Signature
// Parameter:
//   - path: absolute path ของ .exe ที่ต้องการตรวจสอบ
//
// Return:
//   - *Signature: ผลการตรวจสอบลายเซ็น
func (v *authenticodeVerifier) run(path string) *Signature {
	// แปลง path เป็น UTF-16 pointer สำหรับส่งให้ Win32 API
	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil // แปลง path เป็น UTF-16 ล้มเหลว
	}

	// สร้าง WINTRUST_FILE_INFO struct พร้อม path ของไฟล์
	fileInfo := wintrustFileInfo{
		cbStruct:      uint32(unsafe.Sizeof(wintrustFileInfo{})), // กำหนดขนาด struct (จำเป็น)
		pcwszFilePath: wpath,                                      // UTF-16 path ของไฟล์
	}

	// สร้าง WINTRUST_DATA struct พร้อมการตั้งค่าทั้งหมด
	data := wintrustData{
		cbStruct:            uint32(unsafe.Sizeof(wintrustData{})), // กำหนดขนาด struct
		dwUIChoice:          wtdUINone,                             // ปิด UI dialog ทั้งหมด
		fdwRevocationChecks: wtdRevokeNone,                         // ข้ามการตรวจ CRL/OCSP
		dwUnionChoice:       wtdChoiceFile,                         // union คือ file info
		pFile:               uintptr(unsafe.Pointer(&fileInfo)),    // pointer ไปยัง file info
		dwStateAction:       wtdStateActionVerify,                  // เริ่มการตรวจสอบ
		dwProvFlags:         wtdSaferFlag,                          // รัน SAFER check ด้วย
	}

	// เรียก WinVerifyTrust เพื่อตรวจสอบลายเซ็น
	ret, _, _ := procWinVerifyTrust.Call(
		0, // hwnd = NULL (ไม่มี UI window)
		uintptr(unsafe.Pointer(&actionGenericVerifyV2)), // GUID ของ Authenticode policy
		uintptr(unsafe.Pointer(&data)),                  // pointer ไปยัง WINTRUST_DATA
	)

	// แปลง HRESULT ที่ได้เป็น SignatureStatus enum ของเรา
	status := mapTrustResult(uint32(ret))

	// ต้องปิด state เสมอหลังจากการตรวจสอบ เพื่อคืนหน่วยความจำที่ WinVerifyTrust จัดสรร
	data.dwStateAction = wtdStateActionClose // เปลี่ยน action เป็น CLOSE
	procWinVerifyTrust.Call(
		0,
		uintptr(unsafe.Pointer(&actionGenericVerifyV2)),
		uintptr(unsafe.Pointer(&data)), // ส่ง state data เดิม เพื่อปิดอย่างถูกต้อง
	)

	// ป้องกัน GC เก็บ fileInfo และ wpath ก่อน WinVerifyTrust ใช้งานเสร็จ
	runtime.KeepAlive(fileInfo)
	runtime.KeepAlive(wpath)

	// สร้าง Signature พร้อมสถานะที่ได้
	sig := &Signature{Status: status}

	// ดึงรายละเอียด certificate เฉพาะเมื่อไฟล์มีลายเซ็นให้อ่าน
	// (unsigned = ไม่มี, unknown = อ่านไฟล์ไม่ได้อยู่แล้ว)
	if status != SigUnsigned && status != SigUnknown {
		d := extractCertDetails(path)
		sig.Signer = d.signer
		sig.Issuer = d.issuer
		sig.Thumbprint = d.thumbprint
		sig.ValidFrom = d.validFrom
		sig.ValidTo = d.validTo
		sig.Algorithm = d.algorithm
		sig.Chain = d.chain
	}
	return sig // คืน Signature ที่สมบูรณ์
}

// mapTrustResult แปลง HRESULT จาก WinVerifyTrust เป็น SignatureStatus enum ของเรา
// (protocol §3 mapping)
// Parameter:
//   - hr: HRESULT ที่ได้จาก WinVerifyTrust
//
// Return:
//   - SignatureStatus: สถานะลายเซ็น
func mapTrustResult(hr uint32) SignatureStatus {
	switch hr {
	case 0: // S_OK — การตรวจสอบผ่าน ลายเซ็นถูกต้อง
		return SigValid
	case trustENoSignature: // TRUST_E_NOSIGNATURE — ไฟล์ไม่มีลายเซ็น
		return SigUnsigned
	case trustECertExpired: // CERT_E_EXPIRED — ใบรับรองหมดอายุ
		return SigExpired
	case errFileNotFound, errPathNotFound, errSharingViolation,
		cryptEFileError, trustEProviderUnknown, trustESubjectFormUnknown:
		// ตรวจไม่ได้ (ไฟล์หาย/ล็อก/อ่าน PE ไม่ได้) — ไม่ใช่ลายเซ็นเสีย
		return SigUnknown
	default: // HRESULT อื่นๆ — ลายเซ็นไม่ถูกต้อง เช่น ไฟล์ถูกแก้ไข
		return SigInvalid
	}
}

// --- best-effort signer name extraction (ไม่ fatal ถ้าล้มเหลว) ----------------------

// ค่าคงที่จาก wincrypt.h สำหรับ CryptQueryObject, CryptMsgGetParam, CertFindCertificateInStore
// ใช้ walk ผ่าน embedded PKCS#7 signature เพื่อดึงชื่อ CN ของ leaf signer และ issuer
// โดยไม่ต้องพึ่ง library crypto ขนาดใหญ่
const (
	certQueryObjectFile            = 1         // query จากไฟล์ ไม่ใช่ blob ใน memory
	certQueryContentFlagPKCS7Embed = 0x400     // Authenticode ฝัง PKCS#7 SignedData ในไฟล์
	certQueryFormatFlagBinary      = 2         // encoding แบบ DER binary
	cmsgSignerCertInfoParam        = 7         // CMSG_SIGNER_CERT_INFO_PARAM — ดึง CERT_INFO ของ signer[0]
	certNameSimpleDisplayType      = 4         // CERT_NAME_SIMPLE_DISPLAY_TYPE — ชื่อ CN แบบ friendly string
	certNameIssuerFlag             = 1         // CERT_NAME_ISSUER_FLAG — คืน issuer แทน subject
	x509AsnEncoding                = 0x1       // X509_ASN_ENCODING — encoding มาตรฐาน X.509
	pkcs7AsnEncoding               = 0x10000   // PKCS_7_ASN_ENCODING — encoding PKCS#7
	certFindSubjectCert            = 0x000B0000 // CERT_FIND_SUBJECT_CERT — ค้นหา cert ด้วย CERT_INFO
	certCloseStoreCheckFlag        = 0          // ปิด store โดยไม่ assert ว่าทุก context ถูก free แล้ว
	certSHA1HashPropID             = 3          // CERT_SHA1_HASH_PROP_ID — property id ของ SHA-1 thumbprint
	maxChainCerts                  = 16         // จำกัดจำนวน cert ที่ enumerate กัน loop ผิดปกติ
)

// certDetails รวมข้อมูลทั้งหมดที่ดึงได้จาก embedded Authenticode signature
type certDetails struct {
	signer     string     // leaf subject CN
	issuer     string     // leaf issuer CN
	thumbprint string     // SHA-1 ของ leaf cert (hex ตัวพิมพ์ใหญ่)
	algorithm  string     // signature algorithm เช่น sha256RSA
	validFrom  string     // วันเริ่มมีผลของ leaf cert (YYYY-MM-DD)
	validTo    string     // วันหมดอายุของ leaf cert (YYYY-MM-DD)
	chain      []CertNode // chain ทั้งหมด (leaf เป็นตัวแรก)
}

// extractCertDetails ดึงข้อมูล certificate ทั้งหมดจาก embedded Authenticode
// signature ของไฟล์ที่ path กำหนด: leaf CN/issuer/วันที่/algorithm/thumbprint
// และ certificate chain ทั้งหมดใน embedded store
//
// ทั้งฟังก์ชันห่อด้วย recover() เพื่อให้ syscall panic ไม่ crash scan loop
// (best-effort — field ที่ดึงไม่ได้จะเป็น "")
// Parameter:
//   - path: absolute path ของไฟล์ PE ที่มีลายเซ็น
//
// Return:
//   - certDetails: ข้อมูล certificate ที่ดึงได้ (field ว่างถ้าดึงไม่สำเร็จ)
func extractCertDetails(path string) (out certDetails) {
	// recover ทุก panic เพื่อให้ syscall plumbing ไม่ crash scan loop
	defer func() { _ = recover() }()

	// แปลง path เป็น UTF-16 pointer สำหรับส่งให้ Win32 API
	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return // แปลง path ล้มเหลว — คืน certDetails ว่าง
	}

	// ขั้นตอนที่ 1: เปิด embedded PKCS#7 signature จากไฟล์
	// hStore คือ embedded certificate store, hMsg คือ PKCS#7 message
	var hStore, hMsg uintptr
	ok, _, _ := procCryptQueryObject.Call(
		certQueryObjectFile,                          // query จากไฟล์
		uintptr(unsafe.Pointer(wpath)),               // UTF-16 path ของไฟล์
		certQueryContentFlagPKCS7Embed,               // ต้องการ embedded PKCS#7
		certQueryFormatFlagBinary,                    // encoding แบบ DER binary
		0,                                            // reserved = 0
		0, 0, 0,                                      // output parameters ที่ไม่ต้องการ
		uintptr(unsafe.Pointer(&hStore)),             // รับ HCERTSTORE กลับมา
		uintptr(unsafe.Pointer(&hMsg)),               // รับ HCRYPTMSG กลับมา
		0,                                            // output content type (ไม่ต้องการ)
	)
	if ok == 0 {
		return // CryptQueryObject ล้มเหลว — ไฟล์อาจไม่มี PKCS#7 signature
	}

	// คืนหน่วยความจำ store และ message เมื่อฟังก์ชันนี้จบ
	defer procCertCloseStore.Call(hStore, certCloseStoreCheckFlag) // ปิด certificate store
	defer procCryptMsgClose.Call(hMsg)                             // ปิด PKCS#7 message

	// ขั้นตอนที่ 2: ดึง CERT_INFO ของ signer[0] — ใช้ two-call pattern (ขนาดก่อน แล้วข้อมูล)
	var sz uint32
	// เรียกครั้งแรกเพื่อรับขนาด buffer ที่ต้องการ (ส่ง 0 เป็น output buffer)
	procCryptMsgGetParam.Call(hMsg, cmsgSignerCertInfoParam, 0, 0, uintptr(unsafe.Pointer(&sz)))
	if sz == 0 {
		return // ไม่มี signer หรือดึงขนาดไม่สำเร็จ
	}

	// จัดสรร buffer ตามขนาดที่ได้
	buf := make([]byte, sz)
	// เรียกครั้งที่สองเพื่อรับข้อมูล CERT_INFO จริงๆ
	r, _, _ := procCryptMsgGetParam.Call(
		hMsg, cmsgSignerCertInfoParam, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&sz)),
	)
	if r == 0 {
		return // CryptMsgGetParam ล้มเหลว — ไม่สามารถดึง CERT_INFO ได้
	}

	// ขั้นตอนที่ 3: หา CERT_CONTEXT ของ leaf ด้วย typed API (คืน *CertContext ตรงๆ
	// เลี่ยงการ cast uintptr→pointer ที่ go vet เตือน unsafeptr)
	leaf, err := windows.CertFindCertificateInStore(
		windows.Handle(hStore),           // store ที่เปิดจาก PKCS#7 ข้างต้น
		x509AsnEncoding|pkcs7AsnEncoding, // รองรับทั้ง X.509 และ PKCS#7 encoding
		0,                                // ไม่มี flags เพิ่มเติม
		certFindSubjectCert,              // ค้นหาด้วย CERT_INFO (subject match)
		unsafe.Pointer(&buf[0]),          // CERT_INFO ที่ได้จากขั้นตอนที่ 2
		nil,                              // เริ่มค้นหาจาก cert แรก
	)
	if err != nil || leaf == nil {
		return // ไม่พบ certificate ที่ตรงกับ CERT_INFO ใน store
	}
	defer windows.CertFreeCertificateContext(leaf) // คืน CERT_CONTEXT เมื่อฟังก์ชันนี้จบ

	// อ่านข้อมูลทั้งหมดของ leaf cert
	out.signer = certName(leaf, 0)
	out.issuer = certName(leaf, certNameIssuerFlag)
	out.thumbprint = certThumbprint(leaf)

	// อ่าน CERT_INFO ผ่าน typed struct ของ x/sys/windows (ปลอดภัยต่อ struct layout
	// และ architecture มากกว่าการคำนวณ offset เอง)
	if leaf.CertInfo != nil {
		info := leaf.CertInfo
		out.validFrom = filetimeToDate(info.NotBefore)
		out.validTo = filetimeToDate(info.NotAfter)
		out.algorithm = algorithmName(windows.BytePtrToString(info.SignatureAlgorithm.ObjId))
	}

	// enumerate ทุก cert ใน embedded store เพื่อสร้าง chain (leaf เป็นตัวแรก)
	out.chain = enumerateChain(hStore, out.signer)
	return
}

// certThumbprint ดึง SHA-1 hash (thumbprint) ของ certificate แล้วคืนเป็น hex
// ตัวพิมพ์ใหญ่ ตรงกับที่ certmgr/PowerShell แสดง
// Parameter:
//   - c: typed CERT_CONTEXT ของ certificate
//
// Return:
//   - string: thumbprint hex 40 ตัว หรือ "" ถ้าดึงไม่สำเร็จ
func certThumbprint(c *windows.CertContext) string {
	sz := uint32(20)        // SHA-1 = 20 byte
	buf := make([]byte, 20) // buffer สำหรับ hash
	r, _, _ := procCertGetCtxProp.Call(
		uintptr(unsafe.Pointer(c)), certSHA1HashPropID,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&sz)),
	)
	if r == 0 || sz == 0 {
		return "" // ดึง property ไม่สำเร็จ
	}
	return strings.ToUpper(hex.EncodeToString(buf[:sz]))
}

// filetimeToDate แปลง Windows FILETIME เป็นสตริงวันที่ YYYY-MM-DD (UTC)
// คืน "" สำหรับค่าที่ไม่ถูกต้อง (เช่น 0 หรือก่อน Unix epoch)
// Parameter:
//   - ft: windows.Filetime จาก CERT_INFO (NotBefore/NotAfter)
//
// Return:
//   - string: วันที่ YYYY-MM-DD หรือ ""
func filetimeToDate(ft windows.Filetime) string {
	ns := ft.Nanoseconds() // ns ตั้งแต่ Unix epoch
	if ns <= 0 {
		return ""
	}
	return time.Unix(0, ns).UTC().Format("2006-01-02")
}

// enumerateChain วนอ่านทุก certificate ใน embedded store แล้วสร้าง CertNode
// จากนั้นเรียงด้วย orderChain ให้ leaf อยู่ตัวแรก
// Parameter:
//   - hStore: HCERTSTORE ของ embedded PKCS#7
//   - leafSubject: subject CN ของ leaf ใช้เป็นจุดเริ่มของ chain
//
// Return:
//   - []CertNode: chain ที่เรียงแล้ว
func enumerateChain(hStore uintptr, leafSubject string) []CertNode {
	var nodes []CertNode
	var prev *windows.CertContext
	// CertEnumCertificatesInStore free context ก่อนหน้าให้อัตโนมัติทุกครั้งที่เรียก
	// ตัวสุดท้ายจะถูก free ตอน CertCloseStore — จึงไม่ต้อง free เอง
	for range maxChainCerts {
		c, err := windows.CertEnumCertificatesInStore(windows.Handle(hStore), prev)
		if err != nil || c == nil {
			break // ครบทุก cert แล้ว (ERROR_NO_MORE_FILES) หรือ error
		}
		node := CertNode{
			Subject: certName(c, 0),
			Issuer:  certName(c, certNameIssuerFlag),
		}
		if c.CertInfo != nil {
			node.ValidFrom = filetimeToDate(c.CertInfo.NotBefore)
			node.ValidTo = filetimeToDate(c.CertInfo.NotAfter)
		}
		nodes = append(nodes, node)
		prev = c
	}
	return orderChain(nodes, leafSubject)
}

// certName เรียก CertGetNameStringW สองครั้ง:
// ครั้งแรกเพื่อรับขนาด buffer (n รวม null terminator), ครั้งที่สองเพื่อรับข้อมูล
// flags=0 คืน subject; flags=certNameIssuerFlag คืน issuer
// Parameter:
//   - c: typed CERT_CONTEXT ของ certificate
//   - flags: 0 สำหรับ subject CN, certNameIssuerFlag สำหรับ issuer CN
//
// Return:
//   - string: ชื่อ CN ที่อ่านได้ หรือ "" ถ้าไม่สำเร็จ
func certName(c *windows.CertContext, flags uintptr) string {
	cert := uintptr(unsafe.Pointer(c))
	// เรียกครั้งแรกเพื่อรับขนาด buffer (n = จำนวน UTF-16 code unit รวม null terminator)
	n, _, _ := procCertGetNameStr.Call(cert, certNameSimpleDisplayType, flags, 0, 0, 0)
	if n <= 1 {
		return "" // n==1 หมายถึงมีแค่ null terminator — ชื่อว่างเปล่า
	}

	// จัดสรร buffer ขนาด n UTF-16 code unit
	out := make([]uint16, n)
	// เรียกครั้งที่สองเพื่อรับชื่อจริงๆ ลงใน buffer
	procCertGetNameStr.Call(cert, certNameSimpleDisplayType, flags, 0,
		uintptr(unsafe.Pointer(&out[0])), n)

	// แปลง UTF-16 slice เป็น Go string (ตัด null terminator อัตโนมัติ)
	return windows.UTF16ToString(out)
}
