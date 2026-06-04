// Package queue is the agent's durable local retry buffer for scan uploads
// (spec 1.7). When a POST /agents/scans fails (network down, server 5xx) the
// scan is persisted to disk and retried with exponential backoff so results
// survive process restarts and transient outages.
//
// Storage is a directory of one-JSON-file-per-scan rather than SQLite: the spec
// names SQLite, but a CGO SQLite driver breaks the non-negotiable single-binary
// cross-compile rule, and for a 10-item FIFO buffer SQLite is overkill. Each
// item is written with a temp-file + atomic rename, which gives the same
// crash-safety the spec relies on (edge case 5) without any dependency.
package queue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/transport"
)

const (
	maxItems    = 10              // spec 1.7: cap at 10 scans, FIFO-drop oldest
	baseBackoff = 1 * time.Second // 1s, 2s, 4s ...
	maxBackoff  = 5 * time.Minute // ... capped at 5 min
	fileSuffix  = ".json"
)

// Item is one queued scan awaiting upload.
type Item struct {
	ID             string                `json:"id"`
	IdempotencyKey string                `json:"idempotency_key"`
	Request        transport.ScanRequest `json:"request"`
	Attempts       int                   `json:"attempts"`
	CreatedAt      time.Time             `json:"created_at"`
	NextAttempt    time.Time             `json:"next_attempt"`

	filename string // on-disk name; not serialized
}

// Queue is a directory-backed FIFO retry buffer. Safe for concurrent use.
type Queue struct {
	dir string
	mu  sync.Mutex
}

// Open returns a Queue backed by dir, creating the directory if needed.
func Open(dir string) (*Queue, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create queue dir %s: %w", dir, err)
	}
	return &Queue{dir: dir}, nil
}

// Default returns the Queue at the per-OS config location (queue/ subdir).
func Default() (*Queue, error) {
	base, err := config.Dir()
	if err != nil {
		return nil, err
	}
	return Open(filepath.Join(base, "queue"))
}

// Enqueue persists a failed scan for later retry. The first retry is scheduled
// one backoff interval out (the live POST already counts as attempt 1). If the
// queue is full the oldest item is dropped to make room (FIFO).
func (q *Queue) Enqueue(req transport.ScanRequest, idempotencyKey string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	item := Item{
		ID:             NewKey(),
		IdempotencyKey: idempotencyKey,
		Request:        req,
		Attempts:       1,
		CreatedAt:      now,
		NextAttempt:    now.Add(backoff(1)),
	}
	// FIFO ordering is by creation time encoded in the filename.
	item.filename = fmt.Sprintf("%020d-%s%s", now.UnixNano(), item.ID, fileSuffix)
	if err := q.write(item); err != nil {
		return err
	}
	return q.enforceCap()
}

// Due returns items whose next-attempt time has arrived, oldest first.
func (q *Queue) Due(now time.Time) ([]Item, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	items, err := q.load()
	if err != nil {
		return nil, err
	}
	out := items[:0]
	for _, it := range items {
		if !it.NextAttempt.After(now) {
			out = append(out, it)
		}
	}
	return out, nil
}

// Remove deletes a successfully-uploaded item.
func (q *Queue) Remove(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	name, err := q.findByID(id)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(q.dir, name))
}

// Reschedule bumps the attempt count and pushes the next retry out by the
// exponential backoff for the new attempt count.
func (q *Queue) Reschedule(id string, now time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	name, err := q.findByID(id)
	if err != nil {
		return err
	}
	item, err := q.read(name)
	if err != nil {
		return err
	}
	item.Attempts++
	item.NextAttempt = now.Add(backoff(item.Attempts))
	item.filename = name
	return q.write(item)
}

// Depth reports the number of queued items.
func (q *Queue) Depth() (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	names, err := q.names()
	if err != nil {
		return 0, err
	}
	return len(names), nil
}

// --- internals -------------------------------------------------------------

// names returns queue filenames sorted oldest-first (FIFO).
func (q *Queue) names() ([]string, error) {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return nil, fmt.Errorf("read queue dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), fileSuffix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // unix-nano prefix makes lexical == chronological
	return names, nil
}

func (q *Queue) load() ([]Item, error) {
	names, err := q.names()
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(names))
	for _, name := range names {
		it, err := q.read(name)
		if err != nil {
			// A corrupt/partial file should not wedge the whole queue.
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

func (q *Queue) read(name string) (Item, error) {
	data, err := os.ReadFile(filepath.Join(q.dir, name)) // #nosec G304 — internal dir
	if err != nil {
		return Item{}, err
	}
	var it Item
	if err := json.Unmarshal(data, &it); err != nil {
		return Item{}, err
	}
	it.filename = name
	return it, nil
}

// write atomically persists an item (temp file + rename).
func (q *Queue) write(it Item) error {
	data, err := json.Marshal(it)
	if err != nil {
		return fmt.Errorf("marshal queue item: %w", err)
	}
	final := filepath.Join(q.dir, it.filename)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write queue item: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit queue item: %w", err)
	}
	return nil
}

func (q *Queue) findByID(id string) (string, error) {
	names, err := q.names()
	if err != nil {
		return "", err
	}
	suffix := "-" + id + fileSuffix
	for _, name := range names {
		if strings.HasSuffix(name, suffix) {
			return name, nil
		}
	}
	return "", errors.New("queue item not found: " + id)
}

// enforceCap drops the oldest items until at most maxItems remain.
func (q *Queue) enforceCap() error {
	names, err := q.names()
	if err != nil {
		return err
	}
	for len(names) > maxItems {
		if err := os.Remove(filepath.Join(q.dir, names[0])); err != nil {
			return fmt.Errorf("drop oldest queue item: %w", err)
		}
		names = names[1:]
	}
	return nil
}

// backoff returns the retry delay for the given attempt count: base*2^(n-1),
// capped at maxBackoff. attempts < 1 is treated as 1.
func backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 30 { // guard against shift overflow
		return maxBackoff
	}
	d := baseBackoff << (attempts - 1)
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}

// NewKey returns a random hex identifier used both as the item ID and, when the
// caller has none, as an Idempotency-Key so retries never double-insert.
func NewKey() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is unrecoverable in practice; fall back to time.
		return fmt.Sprintf("%020d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
