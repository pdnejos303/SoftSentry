package osutil

import "testing"

// detectFingerprint must be idempotent: the same machine has to report the same
// stable id on every call, otherwise the server-side re-enrollment dedup (which
// keys on this value) would create duplicate machine rows.
func TestDetectFingerprintStable(t *testing.T) {
	a := detectFingerprint()
	b := detectFingerprint()
	if a != b {
		t.Fatalf("fingerprint not stable across calls: %q != %q", a, b)
	}
}

// Detect() must surface the fingerprint on HostInfo so the enroll handshake can
// send it.
func TestDetectIncludesFingerprint(t *testing.T) {
	info, err := Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if info.Fingerprint != detectFingerprint() {
		t.Fatalf("HostInfo.Fingerprint %q != detectFingerprint() %q", info.Fingerprint, detectFingerprint())
	}
}
