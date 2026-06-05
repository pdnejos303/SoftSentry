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

// Lazy-load the two DLLs we need. NewLazySystemDLL resolves from System32 so
// the DLL search path cannot be hijacked by a file in the working directory.
var (
	modWintrust          = windows.NewLazySystemDLL("wintrust.dll")
	procWinVerifyTrust   = modWintrust.NewProc("WinVerifyTrust")
	modCrypt32           = windows.NewLazySystemDLL("crypt32.dll")
	procCryptQueryObject = modCrypt32.NewProc("CryptQueryObject")    // open PKCS#7 store from a file
	procCryptMsgGetParam = modCrypt32.NewProc("CryptMsgGetParam")    // pull a parameter out of a PKCS#7 message
	procCertFindCert     = modCrypt32.NewProc("CertFindCertificateInStore") // locate a cert in a store by CERT_INFO
	procCertGetNameStr   = modCrypt32.NewProc("CertGetNameStringW")  // render a cert subject/issuer as a string
	procCertFreeCtx      = modCrypt32.NewProc("CertFreeCertificateContext") // release a CERT_CONTEXT
	procCertCloseStore   = modCrypt32.NewProc("CertCloseStore")      // release a HCERTSTORE
	procCryptMsgClose    = modCrypt32.NewProc("CryptMsgClose")        // release a HCRYPTMSG
)

// WINTRUST_ACTION_GENERIC_VERIFY_V2 — the standard Authenticode policy GUID.
var actionGenericVerifyV2 = windows.GUID{
	Data1: 0xaac56b,
	Data2: 0xcd44,
	Data3: 0x11d0,
	Data4: [8]byte{0x8c, 0xc2, 0x00, 0xc0, 0x4f, 0xc2, 0x95, 0xee},
}

const (
	// WTD_UI_NONE — suppress any UI dialog during verification.
	wtdUINone = 2
	// WTD_REVOKE_NONE — skip online revocation check (acceptable for inventory; avoids network calls per-binary).
	wtdRevokeNone = 0
	// WTD_CHOICE_FILE — the union member in WINTRUST_DATA points to a WINTRUST_FILE_INFO.
	wtdChoiceFile = 1
	// WINTRUST_DATA.dwStateAction values: verify then close to free state.
	wtdStateActionVerify = 1
	wtdStateActionClose  = 2
	// WTD_SAFER_FLAG — runs the SAFER (Software Restriction Policy) check; standard for file verification.
	wtdSaferFlag = 0x100

	// HRESULTs returned by WinVerifyTrust (winerror.h / msdn).
	trustENoSignature = 0x800B0100 // TRUST_E_NOSIGNATURE  – file has no embedded signature
	trustECertExpired = 0x800B0101 // CERT_E_EXPIRED       – signing cert is past its NotAfter date
)

// wintrustFileInfo mirrors Win32 WINTRUST_FILE_INFO.
// Field order and sizes must match the C layout exactly — passed by pointer
// directly to WinVerifyTrust via unsafe.
type wintrustFileInfo struct {
	cbStruct       uint32         // sizeof(WINTRUST_FILE_INFO) — required before call
	pcwszFilePath  *uint16        // UTF-16 path to the file being verified
	hFile          windows.Handle // optional pre-opened handle (0 = open from path)
	pgKnownSubject *windows.GUID  // optional subject-type override (nil = auto-detect)
}

// wintrustData mirrors Win32 WINTRUST_DATA.
// Field order and sizes must match the C layout exactly.
type wintrustData struct {
	cbStruct            uint32         // sizeof(WINTRUST_DATA)
	pPolicyCallbackData uintptr        // policy-specific callback (nil)
	pSIPClientData      uintptr        // SIP client data (nil)
	dwUIChoice          uint32         // WTD_UI_NONE — suppress any dialog
	fdwRevocationChecks uint32         // WTD_REVOKE_NONE — skip CRL/OCSP (avoids network call per binary)
	dwUnionChoice       uint32         // WTD_CHOICE_FILE — union member is WINTRUST_FILE_INFO
	pFile               uintptr        // pointer to wintrustFileInfo
	dwStateAction       uint32         // WINTRUST_ACTION_VERIFY then _CLOSE to free state
	hWVTStateData       windows.Handle // opaque state — must pass to the _CLOSE call
	pwszURLReference    *uint16        // unused (nil)
	dwProvFlags         uint32         // WTD_SAFER_FLAG — run SRP check
	dwUIContext         uint32         // unused (0)
	pSignatureSettings  uintptr        // WINTRUST_SIGNATURE_SETTINGS* — nil for default
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

// CryptQueryObject / CryptMsgGetParam / CertFindCertificateInStore constants
// (wincrypt.h). These let us walk the embedded PKCS#7 signature to get the
// leaf signer's CN and the issuer CN without a heavyweight crypto library.
const (
	certQueryObjectFile            = 1       // query a file path, not a blob
	certQueryContentFlagPKCS7Embed = 0x400   // Authenticode embeds a PKCS#7 SignedData
	certQueryFormatFlagBinary      = 2       // DER-encoded binary
	cmsgSignerCertInfoParam        = 7       // CMSG_SIGNER_CERT_INFO_PARAM — gets CERT_INFO for signer[0]
	certNameSimpleDisplayType      = 4       // CERT_NAME_SIMPLE_DISPLAY_TYPE — friendly CN string
	certNameIssuerFlag             = 1       // CERT_NAME_ISSUER_FLAG — return issuer instead of subject
	x509AsnEncoding                = 0x1     // X509_ASN_ENCODING
	pkcs7AsnEncoding               = 0x10000 // PKCS_7_ASN_ENCODING
	certFindSubjectCert            = 0x000B0000 // CERT_FIND_SUBJECT_CERT — match by CERT_INFO
	certCloseStoreCheckFlag        = 0       // don't assert that all contexts have been freed
)

// extractSigner extracts the leaf signer CN and issuer CN from the Authenticode
// signature embedded in path. It uses three Win32 calls:
//  1. CryptQueryObject  – open the embedded PKCS#7 store and message
//  2. CryptMsgGetParam  – get the CERT_INFO of the first (leaf) signer
//  3. CertFindCertificateInStore + CertGetNameStringW – look up and render the name
//
// The whole function is wrapped in a recover() so any unexpected syscall panic
// cannot crash the scan loop — extracting the name is best-effort only.
func extractSigner(path string) (signer, issuer string) {
	defer func() { _ = recover() }() // syscall plumbing must never crash a scan

	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", ""
	}

	// Step 1: open the embedded PKCS#7 signature from the file.
	// hStore is the embedded certificate store; hMsg is the PKCS#7 message.
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

	// Step 2: get the CERT_INFO of signer[0] (two-call pattern: size then data).
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

	// Step 3: find the actual CERT_CONTEXT in the embedded store and render names.
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

// certName calls CertGetNameStringW twice: first to get the buffer size (n
// includes the null terminator), then to fill it. flags=0 returns the subject;
// flags=certNameIssuerFlag returns the issuer.
func certName(cert uintptr, flags uintptr) string {
	n, _, _ := procCertGetNameStr.Call(cert, certNameSimpleDisplayType, flags, 0, 0, 0)
	if n <= 1 { // n==1 means only the null terminator — empty name
		return ""
	}
	out := make([]uint16, n)
	procCertGetNameStr.Call(cert, certNameSimpleDisplayType, flags, 0,
		uintptr(unsafe.Pointer(&out[0])), n)
	return windows.UTF16ToString(out)
}
