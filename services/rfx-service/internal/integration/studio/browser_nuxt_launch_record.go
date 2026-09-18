package studio

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	nuxtGroupTermWait  = 10 * time.Second
	nuxtGroupKillWait  = 10 * time.Second
	nuxtGroupPollEvery = 200 * time.Millisecond
)

// nuxtProcessLaunchRecord captures immutable task-owned Unix process group identity
// created when a harness Nuxt dev launcher starts.
type nuxtProcessLaunchRecord struct {
	LauncherPID       int
	PGID              int
	LauncherStarttime uint64
	AppLabel          string
	ExpectedPort      string
	WorktreeRoot      string
	AppRoot           string
	BuildDir          string
	RunToken          string
	CreatedAt         time.Time
}

func newNuxtRunToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (r *nuxtProcessLaunchRecord) complete() bool {
	if r == nil {
		return false
	}
	return strings.TrimSpace(r.RunToken) != "" &&
		r.LauncherPID > 0 &&
		r.PGID > 0 &&
		r.LauncherStarttime > 0 &&
		strings.TrimSpace(r.AppLabel) != "" &&
		isScopedDevPort(r.ExpectedPort) &&
		strings.TrimSpace(r.WorktreeRoot) != "" &&
		strings.TrimSpace(r.AppRoot) != "" &&
		strings.TrimSpace(r.BuildDir) != ""
}

func (r *nuxtProcessLaunchRecord) validateForShutdown(port string) error {
	if r == nil {
		return fmt.Errorf("missing launch record")
	}
	if !r.complete() {
		return fmt.Errorf("malformed launch record")
	}
	if r.ExpectedPort != port {
		return fmt.Errorf("port mismatch record=%s requested=%s", r.ExpectedPort, port)
	}
	if invalidHarnessPGID(r.PGID) {
		return fmt.Errorf("pgid %d is not task-owned", r.PGID)
	}
	return nil
}

func invalidHarnessPGID(pgid int) bool {
	if pgid <= 1 {
		return true
	}
	harnessPGID, err := currentProcessGroupID()
	if err != nil {
		return false
	}
	return pgid == harnessPGID
}

func captureNuxtProcessLaunchRecord(cmd *exec.Cmd, appLabel, port, worktreeRoot, appRoot, buildDir, runToken string) (*nuxtProcessLaunchRecord, error) {
	if runtime.GOOS == "windows" {
		return nil, nil
	}
	if cmd == nil || cmd.Process == nil {
		return nil, fmt.Errorf("launcher process unavailable")
	}
	launcherPID := cmd.Process.Pid
	pgid, err := processGroupID(launcherPID)
	if err != nil {
		return nil, fmt.Errorf("read launcher pgid pid=%d: %w", launcherPID, err)
	}
	starttime, err := processStarttime(launcherPID)
	if err != nil {
		return nil, fmt.Errorf("read launcher starttime pid=%d: %w", launcherPID, err)
	}
	record := &nuxtProcessLaunchRecord{
		LauncherPID:       launcherPID,
		PGID:              pgid,
		LauncherStarttime: starttime,
		AppLabel:          appLabel,
		ExpectedPort:      port,
		WorktreeRoot:      worktreeRoot,
		AppRoot:           appRoot,
		BuildDir:          buildDir,
		RunToken:          runToken,
		CreatedAt:         time.Now().UTC(),
	}
	if err := record.validateForShutdown(port); err != nil {
		return nil, err
	}
	if pgid != launcherPID {
		return nil, fmt.Errorf("launcher pgid mismatch pid=%d pgid=%d", launcherPID, pgid)
	}
	return record, nil
}

func launcherIdentityMatches(record *nuxtProcessLaunchRecord) (bool, error) {
	if record == nil || record.LauncherPID <= 0 {
		return false, fmt.Errorf("missing launcher pid")
	}
	if !processStillRunning(record.LauncherPID) {
		return true, nil
	}
	starttime, err := processStarttime(record.LauncherPID)
	if err != nil {
		return false, err
	}
	if starttime != record.LauncherStarttime {
		return false, fmt.Errorf("launcher starttime mismatch recorded=%d live=%d", record.LauncherStarttime, starttime)
	}
	pgid, err := processGroupID(record.LauncherPID)
	if err != nil {
		return false, err
	}
	if pgid != record.PGID {
		return false, fmt.Errorf("launcher pgid mismatch recorded=%d live=%d", record.PGID, pgid)
	}
	return true, nil
}
