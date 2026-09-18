//go:build unix

package studio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestVerifyListenerGroupMembership_rejectsForeignPGID(t *testing.T) {
	listenerPID, pgid, cleanup := startSyntheticUnixListener(t, pickFreeScopedPort(t))
	defer cleanup()
	ok, reason := verifyListenerGroupMembership(listenerPID, pgid+9999)
	if ok || !strings.Contains(reason, "pgid mismatch") {
		t.Fatalf("want foreign pgid blocked, got ok=%v reason=%q", ok, reason)
	}
}

func TestShutdownTaskOwnedProcessGroup_idempotentWhenPortFree(t *testing.T) {
	record := validUnixLaunchRecord(t, "3022")
	if err := shutdownTaskOwnedProcessGroup(t, record, "3022"); err != nil {
		t.Fatalf("expected idempotent free-port cleanup, got %v", err)
	}
}

func TestShutdownTaskOwnedProcessGroup_blocksUnknownPGID(t *testing.T) {
	port := pickFreeScopedPort(t)
	record := validUnixLaunchRecord(t, port)
	listenerPID, _, cleanup := startSyntheticUnixListener(t, port)
	defer cleanup()
	record.PGID = processGroupIDOrFail(t, listenerPID) + 5000
	err := shutdownTaskOwnedProcessGroup(t, record, port)
	if err == nil || !strings.Contains(err.Error(), "UNKNOWN_PORT_OWNER_BLOCKED=YES") {
		t.Fatalf("expected unknown owner block, got %v", err)
	}
}

func TestShutdownTaskOwnedProcessGroup_terminatesOrphanedListener(t *testing.T) {
	for _, port := range []string{"3020", "3022", "3023"} {
		port := pickFreePortPrefer(t, port)
		record, cleanupLaunch := startSyntheticUnixLauncherGroup(t, port)
		defer cleanupLaunch()
		if !devPortInUse(port) {
			t.Fatalf("port %s expected in use before cleanup", port)
		}
		if err := shutdownTaskOwnedProcessGroup(t, record, port); err != nil {
			t.Fatalf("port %s cleanup: %v", port, err)
		}
		if devPortInUse(port) {
			t.Fatalf("port %s still in use after group cleanup", port)
		}
	}
}

func TestLinuxGroupCleanupLoop(t *testing.T) {
	const cycles = 10
	for i := 1; i <= cycles; i++ {
		port := pickFreeScopedPort(t)
		record, cleanupLaunch := startSyntheticUnixLauncherGroup(t, port)
		if err := shutdownTaskOwnedProcessGroup(t, record, port); err != nil {
			cleanupLaunch()
			t.Fatalf("cycle %d cleanup failed: %v", i, err)
		}
		cleanupLaunch()
		if devPortInUse(port) {
			t.Fatalf("cycle %d port %s still in use", i, port)
		}
	}
}

func TestShutdownTaskOwnedProcessGroup_foreignControlSurvives(t *testing.T) {
	targetPort := pickFreeScopedPortExcluding(t)
	foreignPort := pickFreeScopedPortExcluding(t, targetPort)
	record, cleanupLaunch := startSyntheticUnixDirectLaunchGroup(t, targetPort)
	defer cleanupLaunch()
	foreignPID, _, cleanupForeign := startSyntheticUnixListener(t, foreignPort)
	defer cleanupForeign()
	if err := shutdownTaskOwnedProcessGroup(t, record, targetPort); err != nil {
		t.Fatalf("cleanup target group: %v", err)
	}
	if !processStillRunning(foreignPID) {
		t.Fatalf("foreign control listener pid=%d should remain alive", foreignPID)
	}
}

func validUnixLaunchRecord(t *testing.T, port string) *nuxtProcessLaunchRecord {
	t.Helper()
	root := testWorktreeRoot(t)
	return &nuxtProcessLaunchRecord{
		LauncherPID:       999999,
		PGID:              999999,
		LauncherStarttime: 1,
		AppLabel:          "web-admin",
		ExpectedPort:      port,
		WorktreeRoot:      root,
		AppRoot:           filepath.Join(root, "apps", "web-admin"),
		BuildDir:          t.TempDir(),
		RunToken:          "test-token",
		CreatedAt:         time.Now().UTC(),
	}
}

func pickFreeScopedPort(t *testing.T) string {
	t.Helper()
	return pickFreeScopedPortExcluding(t)
}

func pickFreeScopedPortExcluding(t *testing.T, excluded ...string) string {
	t.Helper()
	blocked := map[string]struct{}{}
	for _, port := range excluded {
		blocked[port] = struct{}{}
	}
	for _, port := range []string{"3020", "3022", "3023", "3120", "3122", "3123"} {
		if _, skip := blocked[port]; skip {
			continue
		}
		if !devPortInUse(port) {
			return port
		}
	}
	t.Fatal("no free scoped port for synthetic fixture")
	return ""
}

func pickFreePortPrefer(t *testing.T, preferred string) string {
	t.Helper()
	if !devPortInUse(preferred) {
		return preferred
	}
	return pickFreeScopedPort(t)
}

func startSyntheticUnixListener(t *testing.T, port string) (listenerPID int, pgid int, cleanup func()) {
	t.Helper()
	cmd := exec.Command(nodeBinary(t), "-e", fmt.Sprintf(
		`const http=require('http');http.createServer((req,res)=>res.end('ok')).listen(%s,'127.0.0.1');`,
		port,
	))
	applyTaskOwnedNuxtProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start synthetic listener: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if devPortInUse(port) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !devPortInUse(port) {
		cleanupPartial(t, cmd, nil)
		t.Fatalf("synthetic listener did not bind port %s", port)
	}
	pgid, pgidErr := processGroupID(cmd.Process.Pid)
	if pgidErr != nil {
		cleanupPartial(t, cmd, nil)
		t.Fatalf("read listener pgid: %v", pgidErr)
	}
	return cmd.Process.Pid, pgid, func() {
		_ = signalProcessGroup(pgid, syscall.SIGKILL)
		_ = cmd.Wait()
	}
}

func syntheticWorktreeRoot(t *testing.T) string {
	t.Helper()
	if root, err := findRepoRoot(); err == nil {
		return root
	}
	return testWorktreeRoot(t)
}

func startSyntheticUnixDirectLaunchGroup(t *testing.T, port string) (*nuxtProcessLaunchRecord, func()) {
	t.Helper()
	root := syntheticWorktreeRoot(t)
	buildDir := t.TempDir()
	nodePath := nodeBinary(t)
	cmd := exec.Command(nodePath, "-e", fmt.Sprintf(
		`const http=require('http');http.createServer((req,res)=>res.end('ok')).listen(%s,'127.0.0.1');`,
		port,
	))
	cmd.Dir = root
	applyTaskOwnedNuxtProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start synthetic direct listener: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if devPortInUse(port) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	pgid, err := processGroupID(cmd.Process.Pid)
	if err != nil {
		cleanupSyntheticLaunch(t, cmd, 0)
		t.Fatalf("listener pgid: %v", err)
	}
	listeners, err := listenerPIDsForPort(port)
	if err != nil || len(listeners) == 0 {
		cleanupSyntheticLaunch(t, cmd, pgid)
		t.Fatalf("direct listener lookup port=%s: %v listeners=%v", port, err, listeners)
	}
	ok, reason := verifyListenerGroupMembership(listeners[0], pgid)
	if !ok {
		cleanupSyntheticLaunch(t, cmd, pgid)
		t.Fatalf("direct listener membership: %s", reason)
	}
	starttime, err := processStarttime(cmd.Process.Pid)
	if err != nil {
		cleanupSyntheticLaunch(t, cmd, pgid)
		t.Fatalf("listener starttime: %v", err)
	}
	record := &nuxtProcessLaunchRecord{
		LauncherPID:       cmd.Process.Pid,
		PGID:              pgid,
		LauncherStarttime: starttime,
		AppLabel:          "web-admin",
		ExpectedPort:      port,
		WorktreeRoot:      root,
		AppRoot:           filepath.Join(root, "apps", "web-admin"),
		BuildDir:          buildDir,
		RunToken:          "direct-listener",
		CreatedAt:         time.Now().UTC(),
	}
	return record, func() {
		_ = shutdownTaskOwnedProcessGroup(t, record, port)
		_ = cmd.Wait()
	}
}

func startSyntheticUnixLauncherGroup(t *testing.T, port string) (*nuxtProcessLaunchRecord, func()) {
	t.Helper()
	root := syntheticWorktreeRoot(t)
	buildDir := t.TempDir()
	wrapperDone := make(chan struct{})
	nodePath := nodeBinary(t)
	wrapperCmd := exec.CommandContext(context.Background(), nodePath, "-e", fmt.Sprintf(
		`const {spawn}=require('child_process');const c=spawn(%q,['-e','const http=require(\"http\");http.createServer((req,res)=>res.end(\"ok\")).listen(%s,\"127.0.0.1\");'],{detached:false,stdio:'ignore'});c.unref();`,
		nodePath,
		port,
	))
	wrapperCmd.Dir = root
	applyTaskOwnedNuxtProcessGroup(wrapperCmd)
	if err := wrapperCmd.Start(); err != nil {
		t.Fatalf("start synthetic wrapper launcher: %v", err)
	}
	launcherPID := wrapperCmd.Process.Pid
	pgid, err := processGroupID(launcherPID)
	if err != nil {
		t.Fatalf("launcher pgid: %v", err)
	}
	starttime, err := processStarttime(launcherPID)
	if err != nil {
		t.Fatalf("launcher starttime: %v", err)
	}
	go func() {
		_ = wrapperCmd.Wait()
		close(wrapperDone)
	}()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if devPortInUse(port) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	select {
	case <-wrapperDone:
	default:
	}
	if !devPortInUse(port) {
		cleanupSyntheticLaunch(t, wrapperCmd, pgid)
		t.Fatalf("expected listener on port %s", port)
	}
	listeners, err := listenerPIDsForPort(port)
	if err != nil || len(listeners) == 0 {
		cleanupSyntheticLaunch(t, wrapperCmd, pgid)
		t.Fatalf("listener lookup port=%s: %v listeners=%v", port, err, listeners)
	}
	ok, reason := verifyListenerGroupMembership(listeners[0], pgid)
	if !ok {
		cleanupSyntheticLaunch(t, wrapperCmd, pgid)
		t.Fatalf("listener membership: %s", reason)
	}
	record := &nuxtProcessLaunchRecord{
		LauncherPID:       launcherPID,
		PGID:              pgid,
		LauncherStarttime: starttime,
		AppLabel:          "web-admin",
		ExpectedPort:      port,
		WorktreeRoot:      root,
		AppRoot:           filepath.Join(root, "apps", "web-admin"),
		BuildDir:          buildDir,
		RunToken:          "live-loop",
		CreatedAt:         time.Now().UTC(),
	}
	return record, func() {
		_ = shutdownTaskOwnedProcessGroup(t, record, port)
		_ = wrapperCmd.Wait()
	}
}

func cleanupSyntheticLaunch(t *testing.T, cmd *exec.Cmd, pgid int) {
	t.Helper()
	if pgid > 1 {
		_ = signalProcessGroup(pgid, syscall.SIGKILL)
	} else if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
	}
	if cmd != nil {
		_ = cmd.Wait()
	}
}

func processGroupIDOrFail(t *testing.T, pid int) int {
	t.Helper()
	pgid, err := processGroupID(pid)
	if err != nil {
		t.Fatalf("processGroupID pid=%d: %v", pid, err)
	}
	return pgid
}

func cleanupPartial(t *testing.T, cmd *exec.Cmd, _ any) {
	t.Helper()
	if cmd != nil && cmd.Process != nil {
		pgid, err := processGroupID(cmd.Process.Pid)
		if err == nil {
			_ = signalProcessGroup(pgid, syscall.SIGKILL)
		} else {
			_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = cmd.Wait()
	}
}

func nodeBinary(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{"node", "nodejs"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	t.Skip("node binary not found in PATH; skipping live unix process-group fixture")
	return ""
}

func TestParseProcStatFields_currentProcess(t *testing.T) {
	pid := os.Getpid()
	fields, ok := parseProcStatFields(pid)
	if !ok || len(fields) < 22 {
		t.Fatalf("parse proc stat pid=%d ok=%v fields=%d", pid, ok, len(fields))
	}
	if _, err := strconv.Atoi(fields[0]); err != nil {
		t.Fatalf("pid field invalid: %q", fields[0])
	}
}
