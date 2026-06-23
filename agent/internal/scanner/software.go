// Package scanner แจกแจงซอฟต์แวร์ที่ติดตั้งอยู่บนเครื่องและตรวจสอบลายเซ็นดิจิทัล
// การรวบรวมข้อมูลเฉพาะแพลตฟอร์มอยู่ในไฟล์ที่มี build tag (windows.go, darwin.go)
// ไฟล์นี้เก็บ type ที่ใช้ร่วมกันและ helper function (dedupe, sort)
package scanner

import (
	"sort"    // ใช้ฟังก์ชันเรียงลำดับมาตรฐานของ Go
	"strings" // ใช้ตัดแต่ง string และเปรียบเทียบนามสกุล/ชื่อไฟล์
)

// SignatureStatus คือ enum ที่แสดงสถานะลายเซ็นดิจิทัลของซอฟต์แวร์
// ค่าเหล่านี้ต้องตรงกับที่ backend รับได้ (ดู backend app/schemas/scans.py SignatureStatus)
type SignatureStatus string

const (
	SigValid    SignatureStatus = "valid"    // ลายเซ็นถูกต้องและยังไม่หมดอายุ
	SigExpired  SignatureStatus = "expired"  // ลายเซ็นหมดอายุแล้ว (cert NotAfter ผ่านไปแล้ว)
	SigInvalid  SignatureStatus = "invalid"  // มีลายเซ็นแต่ตรวจสอบไม่ผ่าน (ดู StatusReason ว่าเพราะอะไร)
	SigUnsigned SignatureStatus = "unsigned" // ไฟล์ไม่มีลายเซ็นดิจิทัล
	SigUnknown  SignatureStatus = "unknown"  // ตรวจสอบไม่ได้ เช่น ไฟล์หาย/ถูกล็อก/อ่าน PE ไม่ได้ (spec 3.1 edge case 1)
)

// reason code ของ status=invalid — อธิบายว่า "ตรวจไม่ผ่านเพราะอะไร"
// ค่าเหล่านี้ต้องตรงกับ key ใน dashboard messages (signatures.reason.*)
// HRESULT ที่ไม่รู้จักจะเก็บเป็น hex ดิบ (เช่น "0x800B0004") แทน code
const (
	ReasonTampered      = "tampered"       // TRUST_E_BAD_DIGEST — ไฟล์ถูกแก้ไขหลังลงนาม (hash ไม่ตรง)
	ReasonUntrustedRoot = "untrusted_root" // CERT_E_UNTRUSTEDROOT — root CA ไม่อยู่ใน trust store / self-signed
	ReasonBrokenChain   = "broken_chain"   // CERT_E_CHAINING — สร้าง certificate chain ไม่ครบ
	ReasonRevoked       = "revoked"        // CERT_E_REVOKED — ใบรับรองถูกเพิกถอน
	ReasonDistrusted    = "distrusted"     // TRUST_E_EXPLICIT_DISTRUST — ถูกตั้งไม่เชื่อถือบนเครื่องนี้
)

// CertNode คือ certificate หนึ่งใบใน chain (leaf → intermediate → root)
// JSON key ต้องตรงกับ dashboard CertChainNode (subject/issuer/valid_from/valid_to)
type CertNode struct {
	Subject   string `json:"subject,omitempty"`    // subject CN ของใบรับรองนี้
	Issuer    string `json:"issuer,omitempty"`     // issuer CN (ผู้ออกใบรับรองนี้)
	ValidFrom string `json:"valid_from,omitempty"` // วันเริ่มมีผล (YYYY-MM-DD)
	ValidTo   string `json:"valid_to,omitempty"`   // วันหมดอายุ (YYYY-MM-DD)
}

// Signature เก็บผลลัพธ์การตรวจสอบลายเซ็น Authenticode (Windows) หรือ codesign (macOS)
type Signature struct {
	Status       SignatureStatus `json:"status"`                        // สถานะลายเซ็น: valid/expired/invalid/unsigned/unknown
	StatusReason string          `json:"status_reason,omitempty"`       // เหตุผลที่ตรวจไม่ผ่าน (เฉพาะ status=invalid): reason code หรือ HRESULT hex
	Signer       string          `json:"signer,omitempty"`              // ชื่อผู้ลงนาม (leaf certificate CN)
	Issuer       string          `json:"issuer,omitempty"`              // ชื่อ CA ที่ออกใบรับรอง (issuer CN)
	Thumbprint   string          `json:"cert_thumbprint,omitempty"`     // fingerprint ของ leaf cert (hex SHA-1 ตัวพิมพ์ใหญ่)
	ValidFrom    string          `json:"cert_valid_from,omitempty"`     // วันที่ leaf cert เริ่มมีผล (YYYY-MM-DD)
	ValidTo      string          `json:"cert_valid_to,omitempty"`       // วันที่ leaf cert หมดอายุ (YYYY-MM-DD)
	Algorithm    string          `json:"signature_algorithm,omitempty"` // อัลกอริทึมที่ใช้เซ็น เช่น sha256RSA
	Chain        []CertNode      `json:"chain,omitempty"`               // certificate chain (leaf เป็นตัวแรก)
}

// Software คือข้อมูลแอปพลิเคชันที่ติดตั้งอยู่บนเครื่อง ตามที่ agent ตรวจพบ
// ถูกส่งไปยัง backend ในรูปแบบ JSON
type Software struct {
	Name          string     `json:"name"`                      // ชื่อซอฟต์แวร์ที่แสดงในระบบ
	Version       string     `json:"version"`                   // เวอร์ชันของซอฟต์แวร์
	Publisher     string     `json:"publisher,omitempty"`       // ผู้พัฒนาหรือผู้เผยแพร่ซอฟต์แวร์
	InstallDate   string     `json:"install_date,omitempty"`    // วันที่ติดตั้งในรูปแบบ YYYY-MM-DD (จาก registry, อาจไม่มีเวลา)
	InstalledAt   string     `json:"installed_at,omitempty"`    // เวลาติดตั้งจริง (best-effort) RFC3339 — จากเวลาสร้างโฟลเดอร์/exe
	InstallPath   string     `json:"install_path,omitempty"`    // path ของไฟล์ executable หลัก
	InstallSizeKB int64      `json:"install_size_kb,omitempty"` // ขนาดการติดตั้งเป็น kilobytes
	Arch          string     `json:"arch,omitempty"`            // สถาปัตยกรรม: x64 หรือ x86
	Source        string     `json:"source"`                    // แหล่งที่มาของข้อมูล: registry, filesystem, appstore, หรือ plist
	Signature     *Signature `json:"signature,omitempty"`       // ผลการตรวจสอบลายเซ็นดิจิทัล (nil ถ้ายังไม่ตรวจ)
}

// dedupe ลบรายการซ้ำที่มี (name, version, install_path) เหมือนกันออก
// Windows registry มักรายงาน product เดียวกันซ้ำจาก WOW6432Node และ per-user hive (spec 1.3)
// รายการแรกที่พบจะถูกเก็บไว้ รายการซ้ำต่อมาจะถูกทิ้ง
// Parameter:
//   - in: รายการ Software ทั้งหมดที่อาจมีซ้ำ
//
// Return:
//   - []Software: รายการที่ไม่มีซ้ำแล้ว
func dedupe(in []Software) []Software {
	// สร้าง map ใช้ triple (name, version, install_path) เป็น key เพื่อติดตามรายการที่เห็นแล้ว
	seen := make(map[[3]string]struct{}, len(in))
	out := make([]Software, 0, len(in)) // pre-allocate slice ด้วยขนาดเท่า input เพื่อประสิทธิภาพ

	// วนซ้ำทุกรายการและข้ามรายการที่เคยเห็นแล้ว
	for _, s := range in {
		key := [3]string{s.Name, s.Version, s.InstallPath} // สร้าง key จาก 3 ฟิลด์
		if _, ok := seen[key]; ok {
			continue // key นี้เคยเห็นแล้ว — ข้ามรายการซ้ำนี้
		}
		seen[key] = struct{}{} // บันทึก key นี้ว่าเคยเห็นแล้ว
		out = append(out, s)   // เพิ่มรายการแรกที่พบเข้าผลลัพธ์
	}
	return out // คืนรายการที่ไม่มีซ้ำ
}

// verifiableExts คือนามสกุลไฟล์ PE ที่ WinVerifyTrust + CryptQueryObject
// อ่าน embedded Authenticode ได้ด้วย logic เดียวกัน
// .msi/.cab ใช้ SIP คนละตัว (signer extraction จะไม่ทำงานเหมือนกัน) จึงไม่รวมไว้
var verifiableExts = map[string]struct{}{
	".exe": {}, ".dll": {}, ".sys": {}, ".ocx": {}, ".cpl": {}, ".scr": {},
}

// isVerifiablePE คืน true ถ้า path ลงท้ายด้วยนามสกุล PE ที่ตรวจลายเซ็นได้
// Parameter:
//   - path: path ของไฟล์ที่ต้องการตรวจ
//
// Return:
//   - bool: true ถ้าเป็นไฟล์ PE ที่ตรวจ Authenticode ได้
func isVerifiablePE(path string) bool {
	dot := strings.LastIndex(path, ".")
	if dot < 0 {
		return false // ไม่มีนามสกุล
	}
	_, ok := verifiableExts[strings.ToLower(path[dot:])]
	return ok
}

// baseName คืนส่วนชื่อไฟล์จาก path โดยตัดทั้ง \ และ / (ใช้แทน filepath.Base
// เพื่อให้ทดสอบได้บนทุก OS ไม่ขึ้นกับ separator ของแพลตฟอร์ม)
// Parameter:
//   - p: path ที่ต้องการตัดชื่อไฟล์
//
// Return:
//   - string: ชื่อไฟล์ส่วนท้าย
func baseName(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// exeCandidate คือ .exe หนึ่งไฟล์ที่พบในโฟลเดอร์ติดตั้ง พร้อมขนาดไฟล์
type exeCandidate struct {
	Path string // path เต็มของไฟล์ .exe
	Size int64  // ขนาดไฟล์เป็น byte (ใช้เป็น tie-breaker)
}

// helperExePrefixes คือ prefix ของชื่อ .exe ที่มักเป็นตัวช่วย ไม่ใช่โปรแกรมหลัก
// (uninstaller, installer, updater ฯลฯ) จึงถูกลดความสำคัญลงเมื่อเลือก main exe
var helperExePrefixes = []string{
	"unins", "uninst", "setup", "install", "update", "vcredist", "vc_redist", "crashpad", "crashreport",
}

// chooseExecutable เลือก .exe ที่น่าจะเป็นโปรแกรมหลักจากรายการที่พบใน
// โฟลเดอร์ติดตั้ง — ตัวช่วย (installer/updater) ถูกลดคะแนน, .exe ที่ชื่อตรงกับ
// DisplayName ได้คะแนนสูงสุด, ถ้าเสมอกันเลือกไฟล์ใหญ่กว่า
// Parameter:
//   - displayName: ชื่อโปรแกรมจาก registry DisplayName
//   - cands: รายการ .exe ที่พบในโฟลเดอร์
//
// Return:
//   - string: path ของ .exe ที่เลือก หรือ "" ถ้าไม่มี candidate
func chooseExecutable(displayName string, cands []exeCandidate) string {
	dn := normalizeName(displayName)
	best := ""
	bestScore := -1 << 30 // เริ่มต่ำสุดเพื่อให้ candidate แรกชนะเสมอ
	var bestSize int64

	for _, c := range cands {
		name := strings.TrimSuffix(strings.ToLower(baseName(c.Path)), ".exe")
		score := 0

		// ตัวช่วยเช่น uninstaller ถูกลดคะแนนมาก
		for _, p := range helperExePrefixes {
			if strings.HasPrefix(name, p) {
				score -= 100
				break
			}
		}

		// ชื่อตรงหรือใกล้เคียง DisplayName ได้คะแนนบวก
		nb := normalizeName(name)
		if dn != "" && nb != "" {
			switch {
			case dn == nb:
				score += 50
			case strings.Contains(dn, nb) || strings.Contains(nb, dn):
				score += 25
			}
		}

		// เลือกตามคะแนนก่อน ถ้าเสมอเลือกไฟล์ใหญ่กว่า
		if score > bestScore || (score == bestScore && c.Size > bestSize) {
			best, bestScore, bestSize = c.Path, score, c.Size
		}
	}
	return best
}

// normalizeName แปลง string เป็นตัวพิมพ์เล็กและตัดอักขระที่ไม่ใช่ a-z0-9 ออก
// เพื่อให้เทียบชื่อโปรแกรมกับชื่อไฟล์ได้แม้มีช่องว่าง/เครื่องหมายต่างกัน
// Parameter:
//   - s: string ต้นฉบับ
//
// Return:
//   - string: ชื่อที่ normalize แล้ว
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// algNames map OID ของ signature algorithm เป็นชื่อที่อ่านง่าย (ตามที่ certmgr แสดง)
var algNames = map[string]string{
	"1.2.840.113549.1.1.5":  "sha1RSA",
	"1.2.840.113549.1.1.11": "sha256RSA",
	"1.2.840.113549.1.1.12": "sha384RSA",
	"1.2.840.113549.1.1.13": "sha512RSA",
	"1.2.840.113549.1.1.10": "RSASSA-PSS",
	"1.2.840.113549.1.1.4":  "md5RSA",
	"1.2.840.10045.4.3.2":   "sha256ECDSA",
	"1.2.840.10045.4.3.3":   "sha384ECDSA",
	"1.2.840.10045.4.3.4":   "sha512ECDSA",
}

// algorithmName แปลง OID เป็นชื่อ algorithm ที่อ่านง่าย ถ้าไม่รู้จักคืน OID เดิม
// Parameter:
//   - oid: OID string เช่น "1.2.840.113549.1.1.11"
//
// Return:
//   - string: ชื่อ algorithm หรือ OID เดิมถ้าไม่อยู่ใน map
func algorithmName(oid string) string {
	if n, ok := algNames[oid]; ok {
		return n
	}
	return oid
}

// orderChain เรียง certificate chain ให้ leaf อยู่ตัวแรก แล้วไล่ตาม issuer ขึ้นไป
// (leaf → intermediate → root) โดยจับคู่ subject ของตัวถัดไปกับ issuer ของตัวปัจจุบัน
// node ที่เชื่อมไม่ได้จะถูกต่อท้ายไว้เพื่อไม่ให้ข้อมูลหาย
// Parameter:
//   - nodes: รายการ cert ที่ดึงจาก embedded store (ลำดับไม่แน่นอน)
//   - leafSubject: subject CN ของ leaf signer ที่ใช้เป็นจุดเริ่ม
//
// Return:
//   - []CertNode: chain ที่เรียงแล้ว (leaf เป็นตัวแรก)
func orderChain(nodes []CertNode, leafSubject string) []CertNode {
	if len(nodes) <= 1 {
		return nodes
	}
	used := make([]bool, len(nodes))

	// หา leaf จาก subject ที่ตรงกับ signer ถ้าไม่เจอเริ่มจากตัวแรก
	cur := -1
	for i, n := range nodes {
		if n.Subject == leafSubject && leafSubject != "" {
			cur = i
			break
		}
	}
	if cur == -1 {
		cur = 0
	}

	ordered := make([]CertNode, 0, len(nodes))
	for cur != -1 {
		used[cur] = true
		ordered = append(ordered, nodes[cur])
		issuer := nodes[cur].Issuer
		if issuer == "" || issuer == nodes[cur].Subject {
			break // self-signed root — จบ chain
		}
		next := -1
		for i, n := range nodes {
			if !used[i] && n.Subject == issuer {
				next = i
				break
			}
		}
		cur = next
	}

	// ต่อ node ที่เชื่อมไม่ได้ไว้ท้ายสุด
	for i, n := range nodes {
		if !used[i] {
			ordered = append(ordered, n)
		}
	}
	return ordered
}

// sortStable เรียงซอฟต์แวร์ตามชื่อ (Name) ก่อน แล้วตามเวอร์ชัน (Version)
// ทำให้ผลลัพธ์การสแกนมีลำดับแน่นอนทุกครั้ง (deterministic output)
// ใช้ SliceStable เพื่อรักษาลำดับเดิมของรายการที่มี key เท่ากัน
// Parameter:
//   - in: รายการ Software ที่ต้องการเรียงลำดับ (เรียงในที่อยู่เดิม in-place)
func sortStable(in []Software) {
	// เรียงลำดับโดยเปรียบ Name ก่อน ถ้าชื่อเหมือนกันให้เปรียบ Version
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Name != in[j].Name {
			return in[i].Name < in[j].Name // เรียงตามชื่อ A→Z
		}
		return in[i].Version < in[j].Version // ถ้าชื่อเหมือนกัน เรียงตามเวอร์ชัน
	})
}

// mergeWindows รวมรายการจาก registry และ filesystem ให้ปลอดภัยต่อ DB unique
// constraint (machine_id, name, version): registry ชนะเมื่อ (normalizeName, version)
// ชนกัน และ filesystem entry ที่ name+version ซ้ำกันเองก็เก็บแค่ตัวแรก
// (dedupe() ปกติ key เป็น (name,version,install_path) จึงไม่ตัด path ต่างให้ — จึงต้องมีขั้นนี้)
// Parameter:
//   - reg: รายการจาก registry collector
//   - fs:  รายการจาก filesystem collector
//
// Return:
//   - []Software: reg ตามด้วย fs ที่ไม่ชน (name,version) กับ reg หรือกันเอง
func mergeWindows(reg, fs []Software) []Software {
	type nv = [2]string
	seen := make(map[nv]struct{}, len(reg)+len(fs))
	out := make([]Software, 0, len(reg)+len(fs))

	for _, r := range reg {
		seen[nv{normalizeName(r.Name), r.Version}] = struct{}{}
		out = append(out, r)
	}
	for _, f := range fs {
		key := nv{normalizeName(f.Name), f.Version}
		if _, dup := seen[key]; dup {
			continue // ชนกับ registry หรือ fs entry ก่อนหน้า — ทิ้ง
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out
}
