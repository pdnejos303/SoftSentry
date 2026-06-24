//go:build windows

// windowsupdate.go ห่อการเรียก Windows Update Agent (WUA) ผ่าน COM เพื่อตรวจว่ามี
// อัปเดตค้างกี่ตัว ตัวไหนเป็น security และ KB ของอัปเดตล่าสุดที่ลงสำเร็จ
//
// COM Search() บล็อกยาว (วินาที–นาที) และไม่สนใจ context จึงรันใน goroutine แยกที่
// lock OS thread + init COM เอง แล้ว select กับ timeout ของ caller ถ้า timeout เรา
// คืน error (ปล่อย goroutine ทำต่อจนจบ — เกิดอย่างมากรอบละครั้ง ห่างกันเป็นชั่วโมง)
package device

import (
	"context" // ใช้ timeout/cancellation ครอบ COM call ที่บล็อกยาว
	"fmt"     // ใช้สร้าง error และ prefix "KB"
	"runtime" // ใช้ LockOSThread สำหรับ COM apartment
	"strings" // ใช้ตรวจชื่อ category ว่าเป็น security

	ole "github.com/go-ole/go-ole"          // COM/OLE primitives (CGO-free)
	"github.com/go-ole/go-ole/oleutil"      // helper เรียก method/property แบบ late-bound
)

// wuaResult คือผลจาก WUA online scan หนึ่งครั้ง
type wuaResult struct {
	Pending         []PendingUpdate // อัปเดตที่ค้าง (IsInstalled=0)
	LastInstalledKB string          // KB ของอัปเดตล่าสุดที่ลงสำเร็จ (จาก history)
	LastInstalledAt string          // เวลาที่ลงล่าสุด (RFC3339 จาก history)
}

// runWUAScan ยิง WUA online scan ภายใต้ timeout — แยก goroutine เพื่อไม่ให้ COM
// ที่บล็อกยาวค้างทั้ง scan
func runWUAScan(ctx context.Context) (wuaResult, error) {
	ctx, cancel := context.WithTimeout(ctx, wuaScanTimeout)
	defer cancel()

	type outcome struct {
		res wuaResult
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		res, err := wuaScanCOM()
		ch <- outcome{res, err}
	}()

	select {
	case <-ctx.Done():
		return wuaResult{}, ctx.Err()
	case o := <-ch:
		return o.res, o.err
	}
}

// wuaScanCOM ทำงาน COM จริงทั้งหมดในชุดเดียว (อยู่บน locked OS thread แล้ว)
// ใช้ recover กัน panic จาก COM ที่ไม่คาดคิดให้กลายเป็น error แทนที่จะ crash agent
func wuaScanCOM() (res wuaResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("wua scan panicked: %v", r)
		}
	}()

	// init COM แบบ apartment-threaded; ถ้าถูก init ไว้แล้วด้วย mode อื่นจะได้
	// RPC_E_CHANGED_MODE ซึ่งไม่ใช่ fatal (ยังใช้ apartment เดิมได้)
	if cerr := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); cerr != nil {
		if oerr, ok := cerr.(*ole.OleError); !ok || oerr.Code() != uintptr(0x80010106) {
			return res, fmt.Errorf("CoInitializeEx: %w", cerr)
		}
	}
	defer ole.CoUninitialize()

	sessionUnk, err := oleutil.CreateObject("Microsoft.Update.Session")
	if err != nil {
		return res, fmt.Errorf("create update session: %w", err)
	}
	defer sessionUnk.Release()
	session, err := sessionUnk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return res, fmt.Errorf("query session dispatch: %w", err)
	}
	defer session.Release()

	searcherV, err := oleutil.CallMethod(session, "CreateUpdateSearcher")
	if err != nil {
		return res, fmt.Errorf("create searcher: %w", err)
	}
	searcher := searcherV.ToIDispatch()
	defer searcher.Release()

	res.Pending, err = wuaSearchPending(searcher)
	if err != nil {
		return res, err
	}
	// history เป็น best-effort: ล้มเหลวไม่ทำให้ scan ทั้งก้อนพัง
	res.LastInstalledKB, res.LastInstalledAt = wuaLastInstalled(searcher)
	return res, nil
}

// wuaSearchPending เรียก IUpdateSearcher.Search แล้วแปลงผลเป็น []PendingUpdate
func wuaSearchPending(searcher *ole.IDispatch) ([]PendingUpdate, error) {
	resultV, err := oleutil.CallMethod(searcher, "Search", "IsInstalled=0 and IsHidden=0 and Type='Software'")
	if err != nil {
		return nil, fmt.Errorf("wua search: %w", err)
	}
	result := resultV.ToIDispatch()
	defer result.Release()

	updatesV, err := oleutil.GetProperty(result, "Updates")
	if err != nil {
		return nil, fmt.Errorf("get updates: %w", err)
	}
	updates := updatesV.ToIDispatch()
	defer updates.Release()

	count := propInt(updates, "Count")
	pending := make([]PendingUpdate, 0, count)
	for i := 0; i < count; i++ {
		itemV, err := oleutil.GetProperty(updates, "Item", i)
		if err != nil {
			continue
		}
		update := itemV.ToIDispatch()
		pending = append(pending, parseUpdate(update))
		update.Release()
	}
	return pending, nil
}

// parseUpdate ดึง Title, KB, severity และตัดสินว่าเป็น security จาก IUpdate หนึ่งตัว
func parseUpdate(update *ole.IDispatch) PendingUpdate {
	p := PendingUpdate{
		Title:    propStr(update, "Title"),
		Severity: propStr(update, "MsrcSeverity"),
	}
	p.KB = firstKB(update)
	p.Security = isSecurity(update, p.Severity)
	return p
}

// firstKB คืน "KB#####" ตัวแรกจาก IUpdate.KBArticleIDs (ว่างถ้าไม่มี)
func firstKB(update *ole.IDispatch) string {
	kbV, err := oleutil.GetProperty(update, "KBArticleIDs")
	if err != nil {
		return ""
	}
	kb := kbV.ToIDispatch()
	defer kb.Release()
	if propInt(kb, "Count") == 0 {
		return ""
	}
	idV, err := oleutil.GetProperty(kb, "Item", 0)
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(idV.ToString())
	if id == "" {
		return ""
	}
	return "KB" + id
}

// isSecurity = severity เป็น Critical/Important หรือมี category ชื่อมีคำว่า "Security"
func isSecurity(update *ole.IDispatch, severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "important":
		return true
	}
	catsV, err := oleutil.GetProperty(update, "Categories")
	if err != nil {
		return false
	}
	cats := catsV.ToIDispatch()
	defer cats.Release()
	for i := 0; i < propInt(cats, "Count"); i++ {
		itemV, err := oleutil.GetProperty(cats, "Item", i)
		if err != nil {
			continue
		}
		cat := itemV.ToIDispatch()
		name := strings.ToLower(propStr(cat, "Name"))
		cat.Release()
		if strings.Contains(name, "security") {
			return true
		}
	}
	return false
}

// wuaLastInstalled อ่าน history หาอัปเดตล่าสุดที่ลงสำเร็จ (ResultCode=2) ที่มี KB
func wuaLastInstalled(searcher *ole.IDispatch) (kb string, when string) {
	totalV, err := oleutil.CallMethod(searcher, "GetTotalHistoryCount")
	if err != nil {
		return "", ""
	}
	total := int(totalV.Val)
	if total <= 0 {
		return "", ""
	}
	if total > 50 {
		total = 50 // ดูแค่ 50 รายการล่าสุดก็พอ
	}
	histV, err := oleutil.CallMethod(searcher, "QueryHistory", 0, total)
	if err != nil {
		return "", ""
	}
	hist := histV.ToIDispatch()
	defer hist.Release()

	for i := 0; i < propInt(hist, "Count"); i++ {
		entryV, err := oleutil.GetProperty(hist, "Item", i)
		if err != nil {
			continue
		}
		entry := entryV.ToIDispatch()
		resultCode := propInt(entry, "ResultCode") // 2 = Succeeded
		title := propStr(entry, "Title")
		date := propStr(entry, "Date")
		entry.Release()
		if resultCode != 2 {
			continue
		}
		if k := kbFromTitle(title); k != "" {
			return k, date
		}
	}
	return "", ""
}

// kbFromTitle ดึง "KB#####" จากชื่ออัปเดต (history entry ไม่มี KBArticleIDs)
func kbFromTitle(title string) string {
	idx := strings.Index(strings.ToUpper(title), "KB")
	if idx < 0 {
		return ""
	}
	rest := title[idx+2:]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return ""
	}
	return "KB" + rest[:end]
}

// ── small COM property helpers ─────────────────────────────────────────────────

func propStr(d *ole.IDispatch, name string) string {
	v, err := oleutil.GetProperty(d, name)
	if err != nil {
		return ""
	}
	defer v.Clear()
	return strings.TrimSpace(v.ToString())
}

func propInt(d *ole.IDispatch, name string) int {
	v, err := oleutil.GetProperty(d, name)
	if err != nil {
		return 0
	}
	defer v.Clear()
	return int(v.Val)
}
