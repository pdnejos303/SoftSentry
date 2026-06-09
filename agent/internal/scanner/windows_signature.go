//go:build windows
// build tag นี้บอกให้รวมไฟล์นี้เฉพาะเมื่อ build สำหรับ Windows เท่านั้น

// Package scanner จัดการการตรวจสอบลายเซ็น Authenticode บน Windows
// โดยเรียก WinVerifyTrust และ Crypt32 API โดยตรงผ่าน syscall
package scanner

import (
	"path/filepath" // ใช้แปลง path เป็น absolute path
	"runtime"       // ใช้ KeepAlive เพื่อป้องกัน GC เก็บ pointer ก่อนเวลา
	"strings"       // ใช้ตัดแต่ง string และตรวจสอบนามสกุล .exe
	"sync"          // ใช้ Mutex สำหรับป้องกัน race condition ใน cache
	"unsafe"        // ใช้แปลง Go struct เป็น pointer ส่งให้ Win32 API

	"golang.org/x/sys/windows" // ใช้ windows.GUID, windows.Handle และ UTF16 utility
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
	procCertFindCert     = modCrypt32.NewProc("CertFindCertificateInStore")                  // ค้นหา certificate ใน store ด้วย CERT_INFO
	procCertGetNameStr   = modCrypt32.NewProc("CertGetNameStringW")                          // แปลง subject/issuer ของ cert เป็น string
	procCertFreeCtx      = modCrypt32.NewProc("CertFreeCertificateContext")                  // คืนหน่วยความจำ CERT_CONTEXT
	procCertCloseStore   = modCrypt32.NewProc("CertCloseStore")                              // คืนหน่วยความจำ HCERTSTORE
	procCryptMsgClose    = modCrypt32.NewProc("CryptMsgClose")                               // คืนหน่วยความจำ HCRYPTMSG
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

	// ตรวจสอบเฉพาะไฟล์ .exe เท่านั้น — path ว่างหรือไม่ใช่ .exe ให้คืน nil
	if path == "" || !strings.HasSuffix(strings.ToLower(path), ".exe") {
		return nil // ไม่ใช่ executable ที่ตรวจสอบได้ — ข้ามไป
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

	// ดึงชื่อ signer และ issuer เฉพาะถ้าไฟล์มีลายเซ็น (ไม่ว่าถูกต้องหรือหมดอายุ)
	if status != SigUnsigned {
		sig.Signer, sig.Issuer = extractSigner(path) // ดึงชื่อ CN จาก PKCS#7 signature
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
)

// extractSigner ดึงชื่อ leaf signer CN และ issuer CN จาก Authenticode signature
// ที่ฝังอยู่ในไฟล์ที่ path กำหนด โดยใช้ 3 Win32 call:
//  1. CryptQueryObject  — เปิด embedded PKCS#7 store และ message
//  2. CryptMsgGetParam  — ดึง CERT_INFO ของ signer ตัวแรก (leaf)
//  3. CertFindCertificateInStore + CertGetNameStringW — ค้นหาและแปลงชื่อเป็น string
//
// ทั้งฟังก์ชันห่อด้วย recover() เพื่อให้ syscall panic ไม่ crash scan loop
// การดึงชื่อเป็นแบบ best-effort เท่านั้น
// Parameter:
//   - path: absolute path ของไฟล์ .exe ที่มีลายเซ็น
//
// Return:
//   - signer: ชื่อ CN ของ leaf signer
//   - issuer: ชื่อ CN ของ CA ที่ออกใบรับรอง
func extractSigner(path string) (signer, issuer string) {
	// recover ทุก panic เพื่อให้ syscall plumbing ไม่ crash scan loop
	defer func() { _ = recover() }()

	// แปลง path เป็น UTF-16 pointer สำหรับส่งให้ Win32 API
	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", "" // แปลง path ล้มเหลว — คืนค่าว่าง
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
		return "", "" // CryptQueryObject ล้มเหลว — ไฟล์อาจไม่มี PKCS#7 signature
	}

	// คืนหน่วยความจำ store และ message เมื่อฟังก์ชันนี้จบ
	defer procCertCloseStore.Call(hStore, certCloseStoreCheckFlag) // ปิด certificate store
	defer procCryptMsgClose.Call(hMsg)                             // ปิด PKCS#7 message

	// ขั้นตอนที่ 2: ดึง CERT_INFO ของ signer[0] — ใช้ two-call pattern (ขนาดก่อน แล้วข้อมูล)
	var sz uint32
	// เรียกครั้งแรกเพื่อรับขนาด buffer ที่ต้องการ (ส่ง 0 เป็น output buffer)
	procCryptMsgGetParam.Call(hMsg, cmsgSignerCertInfoParam, 0, 0, uintptr(unsafe.Pointer(&sz)))
	if sz == 0 {
		return "", "" // ไม่มี signer หรือดึงขนาดไม่สำเร็จ
	}

	// จัดสรร buffer ตามขนาดที่ได้
	buf := make([]byte, sz)
	// เรียกครั้งที่สองเพื่อรับข้อมูล CERT_INFO จริงๆ
	r, _, _ := procCryptMsgGetParam.Call(
		hMsg, cmsgSignerCertInfoParam, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&sz)),
	)
	if r == 0 {
		return "", "" // CryptMsgGetParam ล้มเหลว — ไม่สามารถดึง CERT_INFO ได้
	}

	// ขั้นตอนที่ 3: ค้นหา CERT_CONTEXT จริงๆ ใน embedded store และแปลงชื่อเป็น string
	cert, _, _ := procCertFindCert.Call(
		hStore,                               // store ที่เปิดจาก PKCS#7 ข้างต้น
		x509AsnEncoding|pkcs7AsnEncoding,     // รองรับทั้ง X.509 และ PKCS#7 encoding
		0,                                    // ไม่มี flags เพิ่มเติม
		certFindSubjectCert,                  // ค้นหาด้วย CERT_INFO (subject match)
		uintptr(unsafe.Pointer(&buf[0])),     // CERT_INFO ที่ได้จากขั้นตอนที่ 2
		0,                                    // เริ่มค้นหาจาก cert แรก
	)
	if cert == 0 {
		return "", "" // ไม่พบ certificate ที่ตรงกับ CERT_INFO ใน store
	}
	defer procCertFreeCtx.Call(cert) // คืน CERT_CONTEXT เมื่อฟังก์ชันนี้จบ

	// ดึงชื่อ subject (signer) และ issuer จาก certificate
	return certName(cert, 0), certName(cert, certNameIssuerFlag)
}

// certName เรียก CertGetNameStringW สองครั้ง:
// ครั้งแรกเพื่อรับขนาด buffer (n รวม null terminator), ครั้งที่สองเพื่อรับข้อมูล
// flags=0 คืน subject; flags=certNameIssuerFlag คืน issuer
// Parameter:
//   - cert: CERT_CONTEXT ที่ได้จาก CertFindCertificateInStore
//   - flags: 0 สำหรับ subject CN, certNameIssuerFlag สำหรับ issuer CN
//
// Return:
//   - string: ชื่อ CN ที่อ่านได้ หรือ "" ถ้าไม่สำเร็จ
func certName(cert uintptr, flags uintptr) string {
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
