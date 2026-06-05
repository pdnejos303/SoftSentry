//go:build windows

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/runner"
)

// Install registers the SoftSentryAgent SCM service: auto-start, run as
// LocalSystem (the default when ServiceStartName is empty), restart on failure
// 3 times (spec 1.8). Requires an elevated (admin) process.
//
// Install is idempotent: if the service already exists (a re-run / in-place
// upgrade) it is stopped, deleted, and re-created pointing at cfg.ExePath,
// rather than failing. This is what lets the one-click installer replace a
// previous (possibly broken) install with no manual steps.
func Install(cfg Config) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run as admin?): %w", err)
	}
	defer m.Disconnect()

	// Remove any existing instance first so we can recreate it cleanly on the
	// new binary (CreateService below would otherwise reject a duplicate name).
	if s, err := m.OpenService(Name); err == nil {
		err := stopAndDelete(s)
		s.Close()
		if err != nil {
			return err
		}
	}

	s, err := createService(m, cfg)
	if err != nil {
		return err
	}
	defer s.Close()

	// Restart up to 3 times on failure, resetting the failure counter daily.
	restart := mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: 5 * time.Second}
	if err := s.SetRecoveryActions(
		[]mgr.RecoveryAction{restart, restart, restart},
		uint32((24 * time.Hour).Seconds()),
	); err != nil {
		return fmt.Errorf("set recovery actions: %w", err)
	}

	if err := s.Start(); err != nil {
		return fmt.Errorf("service created but failed to start: %w", err)
	}
	return nil
}

// Uninstall stops (if running) and deletes the SCM service. Requires admin.
func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run as admin?): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(Name)
	if err != nil {
		return fmt.Errorf("service %s is not installed: %w", Name, err)
	}
	defer s.Close()
	return stopAndDelete(s)
}

// stopAndDelete stops a running service (best-effort, bounded wait so a service
// that ignores Stop can't hang the installer) then deletes it. The SCM keeps the
// name reserved until every open handle closes, so a follow-up CreateService can
// briefly fail with ERROR_SERVICE_MARKED_FOR_DELETE — createService retries
// around exactly that.
func stopAndDelete(s *mgr.Service) error {
	if st, err := s.Control(svc.Stop); err == nil {
		deadline := time.Now().Add(15 * time.Second)
		for st.State != svc.Stopped && time.Now().Before(deadline) {
			time.Sleep(300 * time.Millisecond)
			if st, err = s.Query(); err != nil {
				break
			}
		}
	}
	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// createService creates the SCM service, retrying while the name is still
// "marked for deletion" from a just-deleted previous instance (the SCM frees the
// name only once all handles to the old service close, which can lag the Delete
// call by a moment). Any other error is returned immediately.
func createService(m *mgr.Mgr, cfg Config) (*mgr.Service, error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		s, err := m.CreateService(Name, cfg.ExePath, mgr.Config{
			DisplayName:  DisplayName,
			Description:  Description,
			StartType:    mgr.StartAutomatic,
			ErrorControl: mgr.ErrorNormal,
		}, cfg.Args...)
		if err == nil {
			return s, nil
		}
		if errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) && time.Now().Before(deadline) {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		return nil, fmt.Errorf("create service: %w", err)
	}
}

// Status reports the SCM service state. It connects with read-only rights
// (SC_MANAGER_CONNECT + SERVICE_QUERY_STATUS) so an unelevated user can query
// status — unlike Install/Uninstall, which need admin.
func Status() (string, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return "", fmt.Errorf("open service manager: %w", err)
	}
	defer windows.CloseServiceHandle(scm)

	namePtr, err := windows.UTF16PtrFromString(Name)
	if err != nil {
		return "", err
	}
	h, err := windows.OpenService(scm, namePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return "not installed", nil //nolint:nilerr // absence is a status, not an error
	}
	s := &mgr.Service{Name: Name, Handle: h}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return "", fmt.Errorf("query service: %w", err)
	}
	return stateString(st.State), nil
}

// RunIfService detects whether the process was started by the SCM and, if so,
// runs the agent loop under the service control handler. It returns
// (false, nil) for an ordinary foreground invocation so the caller runs the
// normal loop instead.
func RunIfService(opts runner.Options) (bool, error) {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return false, fmt.Errorf("detect service context: %w", err)
	}
	if !isSvc {
		return false, nil
	}
	logw := serviceLog()
	opts.Out = logw
	opts.Err = logw
	return true, svc.Run(Name, &agentService{opts: opts})
}

// agentService adapts runner.Run to the Windows service control protocol.
type agentService struct {
	opts runner.Options
}

func (a *agentService) Execute(
	_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status,
) (ssec bool, errno uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		runErr = runner.Run(ctx, a.opts)
	}()

	status <- svc.Status{State: svc.Running, Accepts: accepts}
	for {
		select {
		case <-done: // loop exited on its own
			status <- svc.Status{State: svc.Stopped}
			if errors.Is(runErr, runner.ErrRestartRequired) {
				// Exit non-zero so the SCM "restart on failure" recovery action
				// relaunches the (now updated) binary.
				return false, 1
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				status <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		}
	}
}

// serviceLog opens the agent log file for the service (no console attached);
// falls back to discarding output if the file cannot be opened.
func serviceLog() io.Writer {
	dir, err := config.Dir()
	if err != nil {
		return io.Discard
	}
	f, err := os.OpenFile( //nolint:gosec // path derived from config.Dir
		filepath.Join(dir, "agent.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600,
	)
	if err != nil {
		return io.Discard
	}
	return f
}

func stateString(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start pending"
	case svc.StopPending:
		return "stop pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue pending"
	case svc.PausePending:
		return "pause pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown (%d)", s)
	}
}
