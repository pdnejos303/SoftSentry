// Package runner holds the long-running agent loop (initial scan + periodic
// heartbeat + triggered scan/update). It lives outside cmd/ so both the
// foreground `run` command and the OS service handlers (Windows SCM / macOS
// launchd) can drive the same logic.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/queue"
	"github.com/softsentry/agent/internal/scanner"
	"github.com/softsentry/agent/internal/schedule"
	"github.com/softsentry/agent/internal/storage"
	"github.com/softsentry/agent/internal/transport"
	"github.com/softsentry/agent/internal/updater"
)

// DefaultHeartbeatInterval is the cadence between heartbeats (spec 1.5).
const DefaultHeartbeatInterval = 60 * time.Second

// ErrRestartRequired is returned by Run after a successful self-update: the
// new binary is in place and the process must exit so the service manager
// (Windows SCM recovery / macOS launchd KeepAlive) restarts onto it.
var ErrRestartRequired = errors.New("agent updated; restart required to apply")

// Options tunes a Run invocation.
type Options struct {
	// OneShot scans + heartbeats once, then returns (used by tests/debug).
	OneShot bool
	// HeartbeatInterval overrides DefaultHeartbeatInterval when > 0.
	HeartbeatInterval time.Duration
	// Out/Err receive human-readable progress; either may be nil (discarded).
	Out io.Writer
	Err io.Writer
}

// Run loads config + token, then drives the agent loop until ctx is cancelled
// (or once when OneShot). It returns ctx.Err() on cancellation, or a setup
// error if the agent is not enrolled.
func Run(ctx context.Context, opts Options) error {
	out := writerOrDiscard(opts.Out)
	errw := writerOrDiscard(opts.Err)
	interval := opts.HeartbeatInterval
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.ServerURL == "" {
		return fmt.Errorf("not enrolled — run `softsentry-agent enroll` first")
	}
	tok, err := storage.LoadToken()
	if err != nil {
		return err
	}
	client := transport.New(cfg.ServerURL, tok)
	q, err := queue.Default()
	if err != nil {
		return fmt.Errorf("open retry queue: %w", err)
	}
	started := time.Now()

	// Resolve our own path for self-update; if unknown, auto-update is disabled.
	exePath, err := os.Executable()
	if err != nil {
		exePath = ""
	}

	scanInterval := schedule.Interval(cfg.ScanIntervalHours)
	r := &loop{
		client:       client,
		queue:        q,
		out:          out,
		errw:         errw,
		started:      started,
		autoUpdate:   cfg.AutoUpdateEnabled,
		exePath:      exePath,
		version:      transport.Version,
		scanInterval: scanInterval,
	}

	// Decide whether the start-up scan is due (spec 1.1): a fresh enrollment
	// scans immediately; a restart computes the next scan from last_scan_at so
	// it doesn't re-scan too soon.
	lastScan, err := storage.LoadLastScan()
	if err != nil {
		fmt.Fprintf(errw, "read last scan time: %v\n", err)
	}
	dueIn := schedule.DueIn(lastScan, scanInterval, time.Now())
	if dueIn == 0 {
		trigger := "auto"
		if lastScan.IsZero() {
			trigger = "enroll"
		}
		r.runScan(ctx, trigger)
		dueIn = scanInterval
	} else {
		fmt.Fprintf(out, "Next auto-scan in %s (last scan %s ago).\n",
			dueIn.Round(time.Minute), time.Since(lastScan).Round(time.Minute))
	}

	if opts.OneShot {
		restart := r.handleBeat(ctx)
		fmt.Fprintln(out, "✓ one-shot run complete")
		if restart {
			return ErrRestartRequired
		}
		return nil
	}

	fmt.Fprintf(out, "Agent running. Heartbeat every %s, auto-scan every %s. Press Ctrl+C to stop.\n",
		interval, scanInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	scanTimer := time.NewTimer(dueIn)
	defer scanTimer.Stop()
	if r.handleBeat(ctx) {
		return ErrRestartRequired
	}
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(out, "\nShutting down.")
			return nil // graceful stop, not an error
		case <-ticker.C:
			if r.handleBeat(ctx) {
				return ErrRestartRequired
			}
		case <-scanTimer.C:
			r.runScan(ctx, "auto")
			scanTimer.Reset(scanInterval)
		}
	}
}

// loop bundles the per-run dependencies so the helpers don't need long
// parameter lists.
type loop struct {
	client       *transport.Client
	queue        *queue.Queue
	out          io.Writer
	errw         io.Writer
	started      time.Time
	autoUpdate   bool
	exePath      string
	version      string
	scanInterval time.Duration
}

// runScan performs a scan and, on success, records the scan time so the
// scheduler can compute the next due time across restarts (spec 1.1). Scan
// failures are logged but don't update last_scan_at, so a failed scan will be
// retried at the next opportunity rather than deferred a full interval.
func (r *loop) runScan(ctx context.Context, trigger string) {
	if err := r.scanAndUpload(ctx, trigger); err != nil {
		fmt.Fprintf(r.errw, "%s scan: %v\n", trigger, err)
		return
	}
	if err := storage.SaveLastScan(time.Now()); err != nil {
		fmt.Fprintf(r.errw, "record last scan time: %v\n", err)
	}
}

// handleBeat does one heartbeat cycle. It returns true when a self-update was
// applied and the process should restart.
func (r *loop) handleBeat(ctx context.Context) (restart bool) {
	// Drain any scans that failed to upload earlier (spec 1.7).
	r.flushQueue(ctx)

	beatCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	resp, err := r.client.Heartbeat(beatCtx, int64(time.Since(r.started).Seconds()))
	cancel()
	if err != nil {
		fmt.Fprintf(r.errw, "heartbeat: %v\n", err)
		return false
	}
	if resp.ManualScanRequested {
		fmt.Fprintln(r.errw, "→ manual scan requested by server")
		r.runScan(ctx, "manual")
	}
	return r.maybeUpdate(ctx, resp.AgentUpdateAvailable)
}

// maybeUpdate applies a self-update when one is offered, auto-update is enabled,
// and the offered version differs from the running one (spec 1.6). It returns
// true only when the new binary is successfully in place.
func (r *loop) maybeUpdate(ctx context.Context, up *transport.AgentUpdate) bool {
	if up == nil || !r.autoUpdate {
		return false
	}
	if up.Version == "" || up.Version == r.version {
		return false
	}
	if r.exePath == "" {
		fmt.Fprintln(r.errw, "⚠ update offered but agent path is unknown; skipping")
		return false
	}
	fmt.Fprintf(r.errw, "→ update available: %s (running %s); downloading ...\n", up.Version, r.version)
	updCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := updater.Apply(updCtx, r.client, r.exePath, *up); err != nil {
		fmt.Fprintf(r.errw, "⚠ auto-update failed (staying on %s): %v\n", r.version, err)
		return false
	}
	fmt.Fprintf(r.out, "✓ updated to %s; restarting to apply\n", up.Version)
	return true
}

// scanAndUpload runs a local scan and POSTs it to the backend. On upload
// failure the scan is persisted to the retry queue rather than lost (spec 1.7),
// so this returns an error only when the scan itself fails.
func (r *loop) scanAndUpload(ctx context.Context, trigger string) error {
	scanCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	fmt.Fprintln(r.errw, "Scanning ...")
	res, err := scanner.Run(scanCtx)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	req := toScanRequest(res, trigger)
	key := queue.NewKey() // stable Idempotency-Key across the live try + any retries

	postCtx, cancel2 := context.WithTimeout(ctx, 60*time.Second)
	defer cancel2()
	accepted, err := r.client.PostScan(postCtx, req, key)
	if err != nil {
		if qErr := r.queue.Enqueue(req, key); qErr != nil {
			return fmt.Errorf("upload scan failed (%v) and could not queue: %w", err, qErr)
		}
		fmt.Fprintf(r.errw, "⚠ upload failed, queued for retry: %v\n", err)
		return nil
	}
	fmt.Fprintf(r.errw, "✓ uploaded %d software entries (scan %s)\n",
		len(res.Software), accepted.ScanUUID)
	return nil
}

// flushQueue retries every scan whose backoff has elapsed, removing it on
// success and rescheduling (with longer backoff) on continued failure.
func (r *loop) flushQueue(ctx context.Context) {
	due, err := r.queue.Due(time.Now())
	if err != nil {
		fmt.Fprintf(r.errw, "read retry queue: %v\n", err)
		return
	}
	for _, item := range due {
		postCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		accepted, err := r.client.PostScan(postCtx, item.Request, item.IdempotencyKey)
		cancel()
		if err != nil {
			if rErr := r.queue.Reschedule(item.ID, time.Now()); rErr != nil {
				fmt.Fprintf(r.errw, "reschedule queued scan: %v\n", rErr)
			}
			continue
		}
		if rErr := r.queue.Remove(item.ID); rErr != nil {
			fmt.Fprintf(r.errw, "remove queued scan: %v\n", rErr)
		}
		fmt.Fprintf(r.errw, "✓ retried queued scan (scan %s)\n", accepted.ScanUUID)
	}
}

// toScanRequest maps a scanner.Result to the wire payload.
func toScanRequest(res scanner.Result, trigger string) transport.ScanRequest {
	items := make([]transport.SoftwareItem, 0, len(res.Software))
	for _, s := range res.Software {
		item := transport.SoftwareItem{
			Name:          s.Name,
			Version:       s.Version,
			Publisher:     s.Publisher,
			InstallDate:   s.InstallDate,
			InstallPath:   s.InstallPath,
			InstallSizeKB: s.InstallSizeKB,
			Arch:          s.Arch,
			Source:        s.Source,
		}
		if s.Signature != nil {
			item.Signature = &transport.SignatureItem{
				Status:         string(s.Signature.Status),
				Signer:         s.Signature.Signer,
				Issuer:         s.Signature.Issuer,
				CertThumbprint: s.Signature.Thumbprint,
			}
		}
		items = append(items, item)
	}
	scanType := "auto"
	if trigger == "manual" {
		scanType = "manual"
	}
	return transport.ScanRequest{
		StartedAt:   res.StartedAt.UTC().Format(time.RFC3339),
		CompletedAt: res.FinishedAt.UTC().Format(time.RFC3339),
		ScanType:    scanType,
		Trigger:     trigger,
		Software:    items,
	}
}

func writerOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
