package studio

import "testing"

const testWorktree = `D:\Projects\freight-platform-wt\rfx-scoring-browser-startup-race-v3.0d`

func TestClassifyDevPortListener_taskOwnedStaleNuxt(t *testing.T) {
	proc := portProcessInfo{
		PID:         4242,
		ProcessName: "node.exe",
		CommandLine: `node "D:\Projects\freight-platform-wt\rfx-scoring-browser-startup-race-v3.0d\apps\web-admin\node_modules\nuxt\bin\nuxt.mjs" "dev" "--port" "3022" "--host" "127.0.0.1"`,
	}
	kill, blocked, reason := classifyDevPortListener("3022", proc, testWorktree)
	if !kill || blocked || reason == "" {
		t.Fatalf("want kill=true blocked=false, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestClassifyDevPortListener_unknownListenerBlocked(t *testing.T) {
	proc := portProcessInfo{
		PID:         9999,
		ProcessName: "python.exe",
		CommandLine: `python -m http.server 3022`,
	}
	kill, blocked, reason := classifyDevPortListener("3022", proc, testWorktree)
	if kill || !blocked {
		t.Fatalf("want kill=false blocked=true, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestClassifyDevPortListener_protectedProcessBlocked(t *testing.T) {
	proc := portProcessInfo{
		PID:         1111,
		ProcessName: "powershell.exe",
		CommandLine: `powershell -Command "Start-Process node"`,
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktree)
	if kill || !blocked {
		t.Fatalf("want kill=false blocked=true, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_nuxtWrongPortNotKillable(t *testing.T) {
	proc := portProcessInfo{
		PID:         5555,
		ProcessName: "node.exe",
		CommandLine: `nuxt dev --port 3000 --host 127.0.0.1`,
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktree)
	if kill || !blocked {
		t.Fatalf("want kill=false blocked=true, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_taskOwnedStalePnpmLauncher(t *testing.T) {
	proc := portProcessInfo{
		PID:         27812,
		ProcessName: "node.exe",
		CommandLine: `"C:\Program Files\nodejs\node.exe" "C:\Program Files\nodejs\node_modules\corepack\dist\pnpm.js" --filter @freight-platform/web-admin exec nuxt dev --port 3022 --host 127.0.0.1`,
	}
	kill, blocked, reason := classifyDevPortListener("3022", proc, testWorktree)
	if !kill || blocked || reason == "" {
		t.Fatalf("want kill=true blocked=false, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestIsTaskOwnedStalePnpmNuxtLauncher_wrongPortNotKillable(t *testing.T) {
	proc := portProcessInfo{
		PID:         8888,
		ProcessName: "node.exe",
		CommandLine: `node pnpm.js --filter @freight-platform/web-admin exec nuxt dev --port 3000 --host 127.0.0.1`,
	}
	if isTaskOwnedStalePnpmNuxtLauncher("3022", proc.ProcessName, proc.CommandLine, testWorktree) {
		t.Fatal("expected pnpm launcher on wrong port to not be killable")
	}
}

func TestCommandLineMatchesNuxtDevPort(t *testing.T) {
	cases := []struct {
		cmd  string
		port string
		want bool
	}{
		{`nuxt dev --port 3023 --host 127.0.0.1`, "3023", true},
		{`nuxt dev --port=3022`, "3022", true},
		{`nuxt dev --port 3022`, "3023", false},
		{
			`node "D:\Projects\freight-platform-wt\rfx-scoring-browser-startup-race-v3.0d\apps\web-admin\node_modules\nuxt\bin\nuxt.mjs" "dev" "--port" "3022" "--host" "127.0.0.1"`,
			"3022",
			true,
		},
	}
	for _, tc := range cases {
		if got := commandLineMatchesNuxtDevPort(tc.cmd, tc.port); got != tc.want {
			t.Fatalf("commandLineMatchesNuxtDevPort(%q, %q)=%v want %v", tc.cmd, tc.port, got, tc.want)
		}
	}
}
