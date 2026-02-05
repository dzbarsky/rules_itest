# Plan: Fix Context Cancellation Sending SIGKILL Before Graceful Shutdown

## Problem Statement

When a service is stopped (e.g., via SIGINT to svcinit), the configured `shutdown_signal` (SIGTERM) and `shutdown_timeout` (grace period) are bypassed because `exec.CommandContext` automatically sends SIGKILL when the context is cancelled.

### Root Cause

In `runner/runner.go:258`:
```go
cmd := exec.CommandContext(ctx, s.Exe, s.Args...)
```

When the signal handler in `cmd/svcinit/main.go` calls `cancelFunc()`, Go's exec package immediately sends SIGKILL to the child process. This happens BEFORE `StopAll()` can send SIGTERM with the configured grace period.

### Timeline of Current Broken Behavior

1. SIGINT received → signal handler calls `cancelFunc()`
2. **Go's exec.CommandContext immediately sends SIGKILL** to child process
3. Child dies instantly (no graceful shutdown)
4. `StopAll()` runs but process is already dead
5. Configured `shutdown_signal=SIGTERM` and `shutdown_timeout=5s` are never used

## Solution

Set `cmd.Cancel` (Go 1.20+) to customize context cancellation behavior. Instead of SIGKILL, send the configured shutdown signal and use WaitDelay for the grace period.

## Implementation Plan

### Step 1: Modify `initializeServiceCmd` in `runner/runner.go`

Replace lines 255-298 with logic that:

1. Uses `exec.Command` instead of `exec.CommandContext` to avoid automatic SIGKILL
2. OR sets `cmd.Cancel` to send the configured `ShutdownSignal` instead of SIGKILL
3. Sets `cmd.WaitDelay` to the configured `shutdown_timeout` for services

**Option A: Custom Cancel function (Preferred)**

```go
func initializeServiceCmd(ctx context.Context, instance *ServiceInstance) error {
    s := instance.VersionedServiceSpec

    cmd := exec.CommandContext(ctx, s.Exe, s.Args...)

    // ... existing env setup ...

    if shouldUseProcessGroups {
        setPgid(cmd)
    }

    // Configure graceful shutdown on context cancellation
    if s.Type == "service" && !s.Deferred {
        shutdownTimeout, err := time.ParseDuration(s.ShutdownTimeout)
        if err != nil {
            shutdownTimeout = 50 * time.Millisecond
        }
        cmd.WaitDelay = shutdownTimeout

        // Send configured signal instead of SIGKILL on context cancel
        cmd.Cancel = func() error {
            var sig syscall.Signal
            switch s.ShutdownSignal {
            case "SIGTERM":
                sig = syscall.SIGTERM
            default:
                sig = syscall.SIGKILL
            }
            return killGroup(cmd, sig)
        }
    }

    // ... rest of function ...
}
```

**Option B: Don't use CommandContext**

```go
cmd := exec.Command(s.Exe, s.Args...)
// Handle context cancellation manually in a goroutine
go func() {
    <-ctx.Done()
    // Let StopAll() handle graceful shutdown
}()
```

### Step 2: Export `killGroup` or refactor signal sending

The `killGroup` function in `runner/pgroup_unix.go` needs to be accessible from the Cancel callback. Options:
- Make it a method on ServiceInstance
- Pass it as a closure
- Export it (if appropriate)

### Step 3: Update `StopWithSignal` to handle already-exited processes

In `runner/service_instance.go`, the `StopWithSignal` function should gracefully handle the case where the process was already killed by context cancellation:

```go
func (s *ServiceInstance) StopWithSignal(signal syscall.Signal) error {
    if s.cmd.Process == nil {
        return nil
    }

    // Check if already done (killed by context cancellation)
    if s.isDone() {
        return nil
    }

    // ... rest of function ...
}
```

### Step 4: Add tests

Create `runner/shutdown_test.go`:

1. **Test: SIGTERM grace period is respected**
   - Start a service that takes 2s to shut down gracefully
   - Send shutdown signal
   - Verify service receives SIGTERM (not SIGKILL)
   - Verify service has time to clean up

2. **Test: SIGKILL after timeout**
   - Start a service that ignores SIGTERM
   - Configure 1s shutdown_timeout
   - Verify SIGKILL is sent after timeout

3. **Test: Context cancellation uses configured signal**
   - Start a service
   - Cancel the context directly
   - Verify configured shutdown_signal is sent (not SIGKILL)

### Step 5: Update documentation

Update `private/itest.bzl` docstrings to clarify:
- `shutdown_signal`: Signal sent on graceful shutdown AND context cancellation
- `shutdown_timeout`: Grace period before SIGKILL, also used as WaitDelay

## Files to Modify

| File | Changes |
|------|---------|
| `runner/runner.go` | Set `cmd.Cancel` and `cmd.WaitDelay` based on service config |
| `runner/service_instance.go` | Handle already-exited processes in `StopWithSignal` |
| `runner/pgroup_unix.go` | Possibly export `killGroup` or refactor |
| `runner/pgroup_windows.go` | Corresponding Windows changes if needed |
| `runner/shutdown_test.go` | New test file for shutdown behavior |
| `private/itest.bzl` | Documentation updates |

## Testing the Fix

1. Build rules_itest with changes
2. In discord repo, run:
   ```bash
   bazel run //:itest_service
   # Wait for services to start
   # Press Ctrl+C
   # Verify "Sent SIGTERM" appears BEFORE processes die
   # Verify docker compose has time to gracefully stop containers
   ```

## Compatibility Notes

- Requires Go 1.20+ for `cmd.Cancel` support
- The `cmd.Cancel` field was added in Go 1.20
- Check `go.mod` for minimum Go version requirement

## References

- Go exec.Cmd.Cancel: https://pkg.go.dev/os/exec#Cmd
- Original investigation: discord repo strace analysis showing SIGKILL via `pidfd_send_signal` before SIGTERM
