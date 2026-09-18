package studio

import "testing"

func TestEvaluateNuxtDevBootLog_nitroBuilt(t *testing.T) {
	state := evaluateNuxtDevBootLog("Nuxt Nitro server built", "3022", func(string) bool { return true })
	if !state.ready || state.fatal {
		t.Fatalf("expected ready without fatal, got %+v", state)
	}
}

func TestEvaluateNuxtDevBootLog_pnpmExitWithListenerReady(t *testing.T) {
	log := "Local: http://127.0.0.1:3022/\nVite server built in 300ms\nERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command \"nuxt\" not found"
	state := evaluateNuxtDevBootLog(log, "3022", func(string) bool { return true })
	if !state.ready || state.fatal {
		t.Fatalf("expected ready when port is listening, got %+v", state)
	}
}

func TestEvaluateNuxtDevBootLog_pnpmErrorWithoutListenerWaits(t *testing.T) {
	log := "ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command \"nuxt\" not found"
	state := evaluateNuxtDevBootLog(log, "3022", func(string) bool { return false })
	if state.fatal || state.ready || state.reason == "" {
		t.Fatalf("expected non-fatal wait when port is free, got %+v", state)
	}
}

func TestEvaluateNuxtDevBootLog_viteBuiltWithPortReadyWithoutPortInUse(t *testing.T) {
	log := "Local: http://127.0.0.1:3022/\nVite server built in 324ms\nERROR EBUSY: resource busy"
	state := evaluateNuxtDevBootLog(log, "3022", func(string) bool { return false })
	if state.ready || state.fatal || state.reason == "" {
		t.Fatalf("expected wait while listener binds, got %+v", state)
	}
}

func TestEvaluateNuxtDevBootLog_viteBuiltWithPortReadyWhenListenerBound(t *testing.T) {
	log := "Local: http://127.0.0.1:3022/\nVite server built in 324ms\nERROR EBUSY: resource busy"
	state := evaluateNuxtDevBootLog(log, "3022", func(string) bool { return true })
	if !state.ready || state.fatal {
		t.Fatalf("expected ready once listener is bound, got %+v", state)
	}
}

func TestEvaluateNuxtDevBootLog_pnpmExitAfterViteWithoutListenerWaits(t *testing.T) {
	log := "Local: http://127.0.0.1:3020/\nVite server built in 421ms\nERROR EBUSY: resource busy\nERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command \"nuxt\" not found"
	state := evaluateNuxtDevBootLog(log, "3020", func(string) bool { return false })
	if state.ready || state.fatal || state.reason == "" {
		t.Fatalf("expected wait when pnpm exits without listener, got %+v", state)
	}
}

func TestEvaluateNuxtDevBootLog_staleLockFatal(t *testing.T) {
	state := evaluateNuxtDevBootLog("Another Nuxt dev server is already running", "3022", func(string) bool { return true })
	if !state.fatal {
		t.Fatalf("expected stale lock fatal, got %+v", state)
	}
}
