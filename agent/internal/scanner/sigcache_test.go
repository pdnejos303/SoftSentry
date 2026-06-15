package scanner

import (
	"testing"
	"time"
)

func sig(status SignatureStatus) *Signature { return &Signature{Status: status} }

func TestSigCacheHitOnSameMtimeAndSize(t *testing.T) {
	c := &sigCache{Version: sigCacheVersion, Entries: map[string]sigCacheEntry{}, touched: map[string]struct{}{}}
	mt := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	c.set("c:\\app.exe", mt, 100, sig(SigValid))

	e, ok := c.get("c:\\app.exe")
	if !ok {
		t.Fatal("expected entry present")
	}
	if !e.MTime.Equal(mt) || e.Size != 100 || e.Signature.Status != SigValid {
		t.Errorf("entry mismatch: %+v", e)
	}
}

func TestSigCachePrunesUntouchedOnCompleteSave(t *testing.T) {
	c := &sigCache{Version: sigCacheVersion, Entries: map[string]sigCacheEntry{}, touched: map[string]struct{}{}}
	mt := time.Now()
	c.Entries["old.exe"] = sigCacheEntry{MTime: mt, Size: 1, Signature: sig(SigValid)} // present, never touched
	c.set("new.exe", mt, 2, sig(SigUnsigned))                                          // touched

	kept := c.prunedEntries(true)
	if _, ok := kept["old.exe"]; ok {
		t.Error("untouched entry should be pruned on complete save")
	}
	if _, ok := kept["new.exe"]; !ok {
		t.Error("touched entry must survive complete save")
	}
}

func TestSigCacheKeepsAllOnIncompleteSave(t *testing.T) {
	c := &sigCache{Version: sigCacheVersion, Entries: map[string]sigCacheEntry{}, touched: map[string]struct{}{}}
	mt := time.Now()
	c.Entries["old.exe"] = sigCacheEntry{MTime: mt, Size: 1, Signature: sig(SigValid)}
	c.set("new.exe", mt, 2, sig(SigUnsigned))

	kept := c.prunedEntries(false)
	if _, ok := kept["old.exe"]; !ok {
		t.Error("incomplete save must keep untouched (not-yet-revisited) entries for resumability")
	}
	if _, ok := kept["new.exe"]; !ok {
		t.Error("incomplete save must keep newly added entries")
	}
}

func TestCacheKeyLowercases(t *testing.T) {
	if cacheKey("C:\\App\\Foo.EXE") != "c:\\app\\foo.exe" {
		t.Errorf("cacheKey did not lowercase: %q", cacheKey("C:\\App\\Foo.EXE"))
	}
}
