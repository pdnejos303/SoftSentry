// Package scanner enumerates installed software and verifies digital
// signatures. Platform-specific collection lives in build-tagged files
// (windows.go, darwin.go); this file holds the shared types and helpers.
package scanner

import "sort"

// SignatureStatus mirrors the values the backend accepts
// (see backend app/schemas/scans.py SignatureStatus).
type SignatureStatus string

const (
	SigValid    SignatureStatus = "valid"
	SigExpired  SignatureStatus = "expired"
	SigInvalid  SignatureStatus = "invalid"
	SigUnsigned SignatureStatus = "unsigned"
)

// Signature describes the Authenticode/codesign result for a binary.
type Signature struct {
	Status     SignatureStatus `json:"status"`
	Signer     string          `json:"signer,omitempty"`
	Issuer     string          `json:"issuer,omitempty"`
	Thumbprint string          `json:"cert_thumbprint,omitempty"`
}

// Software is one installed application as the agent observed it.
type Software struct {
	Name          string     `json:"name"`
	Version       string     `json:"version"`
	Publisher     string     `json:"publisher,omitempty"`
	InstallDate   string     `json:"install_date,omitempty"` // YYYY-MM-DD
	InstallPath   string     `json:"install_path,omitempty"`
	InstallSizeKB int64      `json:"install_size_kb,omitempty"`
	Arch          string     `json:"arch,omitempty"`
	Source        string     `json:"source"` // registry|appstore|plist
	Signature     *Signature `json:"signature,omitempty"`
}

// dedupe collapses entries sharing (name, version, install_path) — WOW6432Node
// and the per-user hive routinely report the same product twice (spec 1.3).
// The first occurrence wins; later duplicates are dropped.
func dedupe(in []Software) []Software {
	seen := make(map[[3]string]struct{}, len(in))
	out := make([]Software, 0, len(in))
	for _, s := range in {
		key := [3]string{s.Name, s.Version, s.InstallPath}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

// sortStable orders software by name then version for deterministic output.
func sortStable(in []Software) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Name != in[j].Name {
			return in[i].Name < in[j].Name
		}
		return in[i].Version < in[j].Version
	})
}
