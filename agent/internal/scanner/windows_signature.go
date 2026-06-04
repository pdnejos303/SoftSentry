//go:build windows

package scanner

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// authenticodeVerifier wraps WinVerifyTrust with a per-scan path cache so the
// same executable is only checked once (spec 1.4 performance note).
type authenticodeVerifier struct {
	mu    sync.Mutex
	cache map[string]*Signature
}

func newAuthenticodeVerifier() *authenticodeVerifier {
	return &authenticodeVerifier{cache: make(map[string]*Signature)}
}

var (
	modWintrust          = windows.NewLazySystemDLL("wintrust.dll")
	procWinVerifyTrust   = modWintrust.NewProc("WinVerifyTrust")
	modCrypt32           = windows.NewLazySystemDLL("crypt32.dll")
	procCryptQueryObject = modCrypt32.NewProc("CryptQueryObject")
	procCryptMsgGetParam = modCrypt32.NewProc("CryptMsgGetParam")
	procCertFindCert     = modCrypt32.NewProc("CertFindCertificateInStore")
	procCertGetNameStr   = modCrypt32.NewProc("CertGetNameStringW")
	procCertFreeCtx      = modCrypt32.NewProc("CertFreeCertificateContext")
	procCertCloseStore   = modCrypt32.NewProc("CertCloseStore")
	procCryptMsgClose    = modCrypt32.NewProc("CryptMsgClose")
)

// WINTRUST_ACTION_GENERIC_VERIFY_V2 — the standard Authenticode policy GUID.
var actionGenericVerifyV2 = windows.GUID{
	Data1: 0xaac56b,
	Data2: 0xcd44,
	Data3: 0x11d0,
	Data4: [8]byte{0x8c, 0xc2, 0x00, 0xc0, 0x4f, 0xc2, 0x95, 0xee},
}

const (
	wtdUINone            = 2
	wtdRevokeNone        = 0
	wtdChoiceFile        = 1
	wtdStateActionVerify = 1
	wtdStateActionClose  = 2
	wtdSaferFlag         = 0x100

	// HRESULTs returned by WinVerifyTrust (winerror.h).
	trustENoSignature = 0x800B0100
	trustECertExpired = 0x800B0101
)

type wintrustFileInfo struct {
	cbStruct       uint32
	pcwszFilePath  *uint16
	hFile          windows.Handle
	pgKnownSubject *windows.GUID
}

type wintrustData struct {
	cbStruct            uint32
	pPolicyCallbackData uintptr
	pSIPClientData      uintptr
	dwUIChoice          uint32
	fdwRevocationChecks uint32
	dwUnionChoice       uint32
	pFile               uintptr
	dwStateAction       uint32
	hWVTStateData       windows.Handle
	pwszURLReference    *uint16
	dwProvFlags         uint32
	dwUIContext         uint32
	pSignatureSettings  uintptr
}

// verify returns the signature status for the file at path. install paths are
// frequently directories; in that case we treat it as having no verifiable
// binary and report unsigned only when truly unsigned is unknowable -> nil.
func (v *authenticodeVerifier) verify(path string) *Signature {
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if path == "" || !strings.HasSuffix(strings.ToLower(path), ".exe") {
		return nil // only verify concrete executables
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}

	v.mu.Lock()
	if cached, ok := v.cache[abs]; ok {
		v.mu.Unlock()
		return cached
	}
	v.mu.Unlock()

	sig := v.run(abs)

	v.mu.Lock()
	v.cache[abs] = sig
	v.mu.Unlock()
	return sig
}

func (v *authenticodeVerifier) run(path string) *Signature {
	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}

	fileInfo := wintrustFileInfo{
		cbStruct:      uint32(unsafe.Sizeof(wintrustFileInfo{})),
		pcwszFilePath: wpath,
	}
	data := wintrustData{
		cbStruct:            uint32(unsafe.Sizeof(wintrustData{})),
		dwUIChoice:          wtdUINone,
		fdwRevocationChecks: wtdRevokeNone,
		dwUnionChoice:       wtdChoiceFile,
		pFile:               uintptr(unsafe.Pointer(&fileInfo)),
		dwStateAction:       wtdStateActionVerify,
		dwProvFlags:         wtdSaferFlag,
	}

	ret, _, _ := procWinVerifyTrust.Call(
		0, // hwnd = INVALID_HANDLE/NULL (no UI)
		uintptr(unsafe.Pointer(&actionGenericVerifyV2)),
		uintptr(unsafe.Pointer(&data)),
	)
	status := mapTrustResult(uint32(ret))

	// Always close the verifier state to free the cached context.
	data.dwStateAction = wtdStateActionClose
	procWinVerifyTrust.Call(
		0,
		uintptr(unsafe.Pointer(&actionGenericVerifyV2)),
		uintptr(unsafe.Pointer(&data)),
	)
	runtime.KeepAlive(fileInfo)
	runtime.KeepAlive(wpath)

	sig := &Signature{Status: status}
	if status != SigUnsigned {
		sig.Signer, sig.Issuer = extractSigner(path)
	}
	return sig
}

// mapTrustResult turns a WinVerifyTrust HRESULT into our status enum
// (protocol §3 mapping).
func mapTrustResult(hr uint32) SignatureStatus {
	switch hr {
	case 0: // S_OK
		return SigValid
	case trustENoSignature:
		return SigUnsigned
	case trustECertExpired:
		return SigExpired
	default:
		return SigInvalid
	}
}

// --- best-effort signer name extraction (never fatal) ----------------------

const (
	certQueryObjectFile            = 1
	certQueryContentFlagPKCS7Embed = 0x400
	certQueryFormatFlagBinary      = 2
	cmsgSignerCertInfoParam        = 7
	certNameSimpleDisplayType      = 4
	certNameIssuerFlag             = 1
	x509AsnEncoding                = 0x1
	pkcs7AsnEncoding               = 0x10000
	certFindSubjectCert            = 0x000B0000
	certCloseStoreCheckFlag        = 0
)

func extractSigner(path string) (signer, issuer string) {
	defer func() { _ = recover() }() // syscall plumbing must never crash a scan

	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", ""
	}

	var hStore, hMsg uintptr
	ok, _, _ := procCryptQueryObject.Call(
		certQueryObjectFile,
		uintptr(unsafe.Pointer(wpath)),
		certQueryContentFlagPKCS7Embed,
		certQueryFormatFlagBinary,
		0,
		0, 0, 0,
		uintptr(unsafe.Pointer(&hStore)),
		uintptr(unsafe.Pointer(&hMsg)),
		0,
	)
	if ok == 0 {
		return "", ""
	}
	defer procCertCloseStore.Call(hStore, certCloseStoreCheckFlag)
	defer procCryptMsgClose.Call(hMsg)

	var sz uint32
	procCryptMsgGetParam.Call(hMsg, cmsgSignerCertInfoParam, 0, 0, uintptr(unsafe.Pointer(&sz)))
	if sz == 0 {
		return "", ""
	}
	buf := make([]byte, sz)
	r, _, _ := procCryptMsgGetParam.Call(
		hMsg, cmsgSignerCertInfoParam, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&sz)),
	)
	if r == 0 {
		return "", ""
	}

	cert, _, _ := procCertFindCert.Call(
		hStore,
		x509AsnEncoding|pkcs7AsnEncoding,
		0,
		certFindSubjectCert,
		uintptr(unsafe.Pointer(&buf[0])),
		0,
	)
	if cert == 0 {
		return "", ""
	}
	defer procCertFreeCtx.Call(cert)

	return certName(cert, 0), certName(cert, certNameIssuerFlag)
}

func certName(cert uintptr, flags uintptr) string {
	n, _, _ := procCertGetNameStr.Call(cert, certNameSimpleDisplayType, flags, 0, 0, 0)
	if n <= 1 {
		return ""
	}
	out := make([]uint16, n)
	procCertGetNameStr.Call(cert, certNameSimpleDisplayType, flags, 0,
		uintptr(unsafe.Pointer(&out[0])), n)
	return windows.UTF16ToString(out)
}
