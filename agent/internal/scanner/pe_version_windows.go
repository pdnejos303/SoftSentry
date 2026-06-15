// pe_version_windows.go อ่าน version resource จากไฟล์ PE จริงผ่าน version.dll
// (_windows.go suffix = build เฉพาะบน Windows) ทั้งฟังก์ชันห่อ recover() กัน
// syscall panic ทำให้ scan loop ล่ม — best-effort อ่านไม่ได้คืน struct ว่าง
package scanner

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Lazy-load version.dll จาก System32 (กัน DLL hijacking เหมือน windows_signature.go)
var (
	modVersion                 = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSize = modVersion.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfo     = modVersion.NewProc("GetFileVersionInfoW")
	procVerQueryValue          = modVersion.NewProc("VerQueryValueW")
)

// readPEVersion ดึง ProductName/FileDescription/CompanyName/FileVersion จาก
// version resource ของไฟล์ PE ที่ path กำหนด คืน struct ว่างถ้าไฟล์ไม่มี resource
func readPEVersion(path string) (out peVersionInfo) {
	defer func() { _ = recover() }() // syscall อาจ panic — กัน scan ล่ม

	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}

	// ขนาด buffer ที่ต้องใช้เก็บ version blob
	var handle uint32
	size, _, _ := procGetFileVersionInfoSize.Call(
		uintptr(unsafe.Pointer(wpath)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if size == 0 {
		return // ไม่มี version resource
	}

	// ดึง version blob ทั้งก้อน
	buf := make([]byte, size)
	r, _, _ := procGetFileVersionInfo.Call(
		uintptr(unsafe.Pointer(wpath)),
		0,
		size,
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r == 0 {
		return
	}

	// ลองทุก lang-codepage ที่ไฟล์ประกาศ + fallback ยอดนิยม เก็บค่าแรกที่ไม่ว่าง
	for _, lc := range translationCodes(buf) {
		out.ProductName = firstNonEmpty(out.ProductName, verQueryString(buf, lc, "ProductName"))
		out.FileDescription = firstNonEmpty(out.FileDescription, verQueryString(buf, lc, "FileDescription"))
		out.CompanyName = firstNonEmpty(out.CompanyName, verQueryString(buf, lc, "CompanyName"))
		out.FileVersion = firstNonEmpty(out.FileVersion, verQueryString(buf, lc, "FileVersion"))
	}
	return
}

// firstNonEmpty คืน a ถ้าไม่ว่าง ไม่งั้นคืน b — ใช้สะสมค่าจากหลาย codepage
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// translationCodes อ่าน \VarFileInfo\Translation เพื่อหา lang-codepage ที่ไฟล์ใช้
// (รูปแบบ "040904b0") แล้วเติม fallback ยอดนิยมต่อท้าย
func translationCodes(buf []byte) []string {
	codes := []string{}

	// ptr เป็น unsafe.Pointer (ไม่ใช่ uintptr) เพื่อให้ GC ตามรอย pointer ที่ชี้เข้า
	// buf ได้ถูกต้อง และเลี่ยง cast uintptr→pointer ที่ go vet เตือน unsafeptr
	var ptr unsafe.Pointer
	var n uint32
	sub, _ := windows.UTF16PtrFromString(`\VarFileInfo\Translation`)
	ok, _, _ := procVerQueryValue.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(sub)),
		uintptr(unsafe.Pointer(&ptr)),
		uintptr(unsafe.Pointer(&n)),
	)
	// แต่ละ translation = lang (uint16) + codepage (uint16) = 4 byte
	if ok != 0 && ptr != nil && n >= 4 {
		count := int(n) / 4
		pairs := unsafe.Slice((*uint16)(ptr), count*2) // [lang,codepage, lang,codepage, ...]
		for i := 0; i < count; i++ {
			codes = append(codes, fmt.Sprintf("%04x%04x", pairs[i*2], pairs[i*2+1]))
		}
	}

	// fallback ยอดนิยม (US English + Unicode/common codepages) เผื่อไฟล์ไม่ประกาศ
	for _, fb := range []string{"040904b0", "040904e4", "000004b0", "040904b9"} {
		codes = append(codes, fb)
	}
	return codes
}

// verQueryString อ่านค่า string หนึ่ง field จาก \StringFileInfo\<langcp>\<field>
func verQueryString(buf []byte, langCodepage, field string) string {
	sub, err := windows.UTF16PtrFromString(`\StringFileInfo\` + langCodepage + `\` + field)
	if err != nil {
		return ""
	}
	// ptr เป็น unsafe.Pointer ตรงๆ (เลี่ยง cast uintptr→pointer ที่ go vet เตือน)
	var ptr unsafe.Pointer
	var n uint32
	ok, _, _ := procVerQueryValue.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(sub)),
		uintptr(unsafe.Pointer(&ptr)),
		uintptr(unsafe.Pointer(&n)),
	)
	if ok == 0 || ptr == nil || n == 0 {
		return ""
	}
	// ptr ชี้ไปยัง UTF-16 string ยาว n code unit (รวม null) — แปลงเป็น Go string
	u16 := unsafe.Slice((*uint16)(ptr), n)
	return windows.UTF16ToString(u16)
}
