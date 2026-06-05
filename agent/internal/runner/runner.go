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
	"strings"
	"sync"
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
	initialTrigger := ""
	if dueIn == 0 {
		initialTrigger = "auto"
		if lastScan.IsZero() {
			initialTrigger = "enroll"
		}
		dueIn = scanInterval
	} else {
		fmt.Fprintf(out, "Next auto-scan in %s (last scan %s ago).\n",
			dueIn.Round(time.Minute), time.Since(lastScan).Round(time.Minute))
	}

	if opts.OneShot {
		// One-shot stays fully synchronous (used by tests/debug): scan if due,
		// then a single all-in-one heartbeat cycle.
		if initialTrigger != "" {
			r.runScan(ctx, initialTrigger)
		}
		restart := r.handleBeat(ctx)
		fmt.Fprintln(out, "✓ one-shot run complete")
		if restart {
			return ErrRestartRequired
		}
		return nil
	}

	return r.serve(ctx, interval, dueIn, initialTrigger)
}

// serve runs the long-lived agent. The heartbeat ticker runs on its own
// goroutine (this one) and never performs slow work: every potentially
// long-running operation — scan, queue flush, self-update — is handed to a
// single background worker. Decoupling them is the whole point: a multi-minute
// scan can no longer starve the heartbeat, so the backend keeps seeing the
// agent as online (it ages to stale→offline after 5 min of silence) and a
// server-requested manual scan is picked up on the very next beat instead of
// waiting for an in-flight scan to finish.
func (r *loop) serve(ctx context.Context, interval, dueIn time.Duration, initialTrigger string) error {
	fmt.Fprintf(r.out, "Agent running. Heartbeat every %s, auto-scan every %s. Press Ctrl+C to stop.\n",
		interval, r.scanInterval)

	// Buffered to 1 + coalesced: at most one scan/update is ever pending, so a
	// burst of requests while the worker is busy collapses into a single run.
	work := make(chan workItem, 1)
	restart := make(chan struct{}, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.worker(ctx, interval, work, restart)
	}()
	defer wg.Wait()

	// Kick off the start-up scan (if due) via the worker so it doesn't delay
	// the first heartbeat.
	if initialTrigger != "" {
		signalWork(work, workItem{scanTrigger: initialTrigger})
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	scanTimer := time.NewTimer(dueIn)
	defer scanTimer.Stop()

	// First beat immediately so the server registers us right away.
	r.beat(ctx, work)
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(r.out, "\nShutting down.")
			return nil // graceful stop, not an error
		case <-restart:
			return ErrRestartRequired
		case <-ticker.C:
			r.beat(ctx, work)
		case <-scanTimer.C:
			signalWork(work, workItem{scanTrigger: "auto"})
			scanTimer.Reset(r.scanInterval)
		}
	}
}

// workItem is a unit of background work handed from the heartbeat loop to the
// worker. At most one field is set per item.
type workItem struct {
	scanTrigger string                 // "" = no scan ("auto"/"manual"/"enroll")
	update      *transport.AgentUpdate // nil = no update offered
}

// signalWork enqueues work without ever blocking the heartbeat loop. The
// channel is buffered to 1, so a request arriving while the worker is busy (or
// while one is already pending) is dropped. Nothing is lost: the server
// re-asserts manual_scan_requested / agent_update_available on every heartbeat
// until the agent satisfies it.
func signalWork(ch chan<- workItem, item workItem) {
	select {
	case ch <- item:
	default:
	}
}

// beat sends one heartbeat — a fast call (10s cap) that must never block on a
// scan — and dispatches any server-requested work to the background worker.
func (r *loop) beat(ctx context.Context, work chan<- workItem) {
	beatCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	resp, err := r.client.Heartbeat(beatCtx, int64(time.Since(r.started).Seconds()))
	cancel()
	if err != nil {
		fmt.Fprintf(r.errw, "heartbeat: %v\n", err)
		return
	}
	if resp.ManualScanRequested {
		fmt.Fprintln(r.errw, "→ manual scan requested by server")
		signalWork(work, workItem{scanTrigger: "manual"})
	}
	if resp.AgentUpdateAvailable != nil {
		signalWork(work, workItem{update: resp.AgentUpdateAvailable})
	}
}

// worker is the single goroutine that performs every slow operation, keeping
// the heartbeat loop responsive. Work is serialized: one scan/update at a time.
// A periodic tick (at the heartbeat cadence) retries any queued uploads, taking
// over the role flushQueue used to play inside each heartbeat.
func (r *loop) worker(
	ctx context.Context,
	flushInterval time.Duration,
	work <-chan workItem,
	restart chan<- struct{},
) {
	flush := time.NewTicker(flushInterval)
	defer flush.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-flush.C:
			r.flushQueue(ctx)
		case item := <-work:
			if item.scanTrigger != "" {
				r.flushQueue(ctx)
				r.runScan(ctx, item.scanTrigger)
			}
			if item.update != nil && r.maybeUpdate(ctx, item.update) {
				// New binary is in place — tell serve to exit so the service
				// manager relaunches onto it, then stop the worker.
				select {
				case restart <- struct{}{}:
				default:
				}
				return
			}
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
	// Loop-breaker: if we already applied this exact binary (same SHA-256) and
	// the backend is *still* offering it, our reported version never advanced to
	// what the manifest claims — i.e. the manifest's version is higher than the
	// version baked into the binary. Re-applying it would only restart-loop
	// ("online then exits, forever"). Skip it and stay online on the current
	// build; a corrected binary (different content → different SHA-256) is still
	// picked up normally.
	if last, err := storage.LoadLastUpdate(); err != nil {
		fmt.Fprintf(r.errw, "read last update marker: %v\n", err)
	} else if up.SHA256 != "" && strings.EqualFold(up.SHA256, last) {
		fmt.Fprintf(r.errw,
			"⚠ server keeps offering %s but this binary already is that download and still reports %s; "+
				"skipping to avoid a restart loop (fix: manifest version must match the binary's built version)\n",
			up.Version, r.version)
		return false
	}
	fmt.Fprintf(r.errw, "→ update available: %s (running %s); downloading ...\n", up.Version, r.version)
	updCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := updater.Apply(updCtx, r.client, r.exePath, *up); err != nil {
		fmt.Fprintf(r.errw, "⚠ auto-update failed (staying on %s): %v\n", r.version, err)
		return false
	}
	// Remember what we just installed so the loop-breaker above can recognise a
	// version/manifest mismatch after the restart instead of looping forever.
	if err := storage.SaveLastUpdate(up.SHA256); err != nil {
		fmt.Fprintf(r.errw, "record last update marker: %v\n", err)
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
