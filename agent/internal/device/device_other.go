//go:build !windows

// build tag นี้ครอบ macOS/Linux และอื่น ๆ ที่ยังไม่รองรับการเก็บ device inventory
// รอบนี้เน้น Windows ก่อน — แพลตฟอร์มอื่นคืน stub ไว้ (struct/field ครบ ต่อยอดทีหลัง)

package device

import "context"

// collect บนแพลตฟอร์มที่ยังไม่รองรับ คืน Info{Supported:false} ทันที
// (TH) Collect() จะ stamp CollectedAt ให้เองภายหลัง
func collect(_ context.Context) Info {
	return Info{Supported: false}
}
