package studio

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestClassifyDevPortListener_taskOwnedStaleNuxt(t *testing.T) {
	adminApp := testAdminAppDir(t)
	nuxtPath := filepath.Join(adminApp, "node_modules", "nuxt", "bin", "nuxt.mjs")
	proc := portProcessInfo{
		PID:         4242,
		ProcessName: "node",
		CommandLine: `node "` + nuxtPath + `" dev --port 3022 --host 127.0.0.1`,
	}
	if runtime.GOOS == "windows" {
		proc.ProcessName = "node.exe"
	}
	kill, blocked, reason := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
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
	kill, blocked, reason := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
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
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
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
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
	if kill || !blocked {
		t.Fatalf("want kill=false blocked=true, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_taskOwnedStalePnpmLauncher(t *testing.T) {
	worktree := testWorktreeRoot(t)
	proc := portProcessInfo{
		PID:         27812,
		ProcessName: "node.exe",
		CommandLine: `"C:\Program Files\nodejs\node.exe" "C:\Program Files\nodejs\node_modules\corepack\dist\pnpm.js" --filter @freight-platform/web-admin exec nuxt dev --port 3022 --host 127.0.0.1 ` + worktree,
		Cwd:         worktree,
	}
	kill, blocked, reason := classifyDevPortListener("3022", proc, worktree)
	if !kill || blocked || reason == "" {
		t.Fatalf("want kill=true blocked=false, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestClassifyDevPortListener_taskOwnedStalePnpmLauncherWithoutWorktreeCwd(t *testing.T) {
	proc := portProcessInfo{
		PID:         12648,
		ProcessName: "node.exe",
		CommandLine: `"C:\Program Files\nodejs\node.exe" "C:\Program Files\nodejs\node_modules\corepack\dist\pnpm.js" --filter @freight-platform/web-admin exec nuxt dev --port 3020 --host 127.0.0.1`,
		Cwd:         `C:\Program Files\nodejs`,
	}
	kill, blocked, reason := classifyDevPortListener("3020", proc, testWorktreeRoot(t))
	if !kill || blocked || reason == "" {
		t.Fatalf("want kill=true blocked=false, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestClassifyDevPortListener_taskOwnedStaleNuxtCliDev(t *testing.T) {
	worktree := testWorktreeRoot(t)
	cliPath := filepath.Join(worktree, "node_modules", ".pnpm", "@nuxt+cli@3.37.0", "node_modules", "@nuxt", "cli", "dist", "dev", "index.mjs")
	proc := portProcessInfo{
		PID:         21732,
		ProcessName: "node.exe",
		CommandLine: `"C:\Program Files\nodejs\node.exe" --enable-source-maps ` + cliPath + ` --port 3020 --host 127.0.0.1`,
	}
	kill, blocked, reason := classifyDevPortListener("3020", proc, worktree)
	if !kill || blocked || reason == "" {
		t.Fatalf("want kill=true blocked=false, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestIsTaskOwnedStalePnpmNuxtLauncher_wrongPortNotKillable(t *testing.T) {
	worktree := testWorktreeRoot(t)
	proc := portProcessInfo{
		PID:         8888,
		ProcessName: "node.exe",
		CommandLine: `node pnpm.js --filter @freight-platform/web-admin exec nuxt dev --port 3000 --host 127.0.0.1`,
		Cwd:         worktree,
	}
	if isTaskOwnedStalePnpmNuxtLauncher("3022", proc.ProcessName, proc.CommandLine, worktree) {
		t.Fatal("expected pnpm launcher on wrong port to not be killable")
	}
}

func TestClassifyDevPortListener_linuxRelativeNuxtListener(t *testing.T) {
	procApp := testProcurementAppDir(t)
	proc := portProcessInfo{
		PID:         6180,
		ProcessName: "node",
		CommandLine: `node ./node_modules/.bin/../nuxt/bin/nuxt.mjs dev --port 3023 --host 127.0.0.1`,
		Cwd:         procApp,
	}
	kill, blocked, reason := classifyDevPortListener("3023", proc, testWorktreeRoot(t))
	if !kill || blocked || reason == "" {
		t.Fatalf("want kill=true blocked=false, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestClassifyDevPortListener_linuxRelativeNuxtFromWorktreeRoot(t *testing.T) {
	worktree := testWorktreeRoot(t)
	proc := portProcessInfo{
		PID:         6295,
		ProcessName: "node",
		CommandLine: `node ./node_modules/.bin/../nuxt/bin/nuxt.mjs dev --port 3023 --host 127.0.0.1`,
		Cwd:         worktree,
	}
	kill, blocked, reason := classifyDevPortListener("3023", proc, worktree)
	if !kill || blocked || reason == "" {
		t.Fatalf("want kill=true blocked=false, got kill=%v blocked=%v reason=%q", kill, blocked, reason)
	}
}

func TestClassifyDevPortListener_linuxRelativeNuxtWrongPortAppBlocked(t *testing.T) {
	procApp := testProcurementAppDir(t)
	proc := portProcessInfo{
		PID:         6181,
		ProcessName: "node",
		CommandLine: `node ./node_modules/.bin/../nuxt/bin/nuxt.mjs dev --port 3022 --host 127.0.0.1`,
		Cwd:         procApp,
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
	if kill || !blocked {
		t.Fatalf("procurement cwd on admin port must be blocked, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_relativeWithoutCwdBlocked(t *testing.T) {
	proc := portProcessInfo{
		PID:         7001,
		ProcessName: "node",
		CommandLine: `node ./node_modules/nuxt/bin/nuxt.mjs dev --port 3022 --host 127.0.0.1`,
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
	if kill || !blocked {
		t.Fatalf("relative argv without cwd must be blocked, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_relativeWithForeignCwdBlocked(t *testing.T) {
	proc := portProcessInfo{
		PID:         7002,
		ProcessName: "node",
		CommandLine: `node ./node_modules/nuxt/bin/nuxt.mjs dev --port 3022 --host 127.0.0.1`,
		Cwd:         filepath.Join(testWorktreeRoot(t), "..", "other-worktree", "apps", "web-admin"),
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
	if kill || !blocked {
		t.Fatalf("relative argv with foreign cwd must be blocked, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_siblingAppPathBlocked(t *testing.T) {
	sibling := filepath.Join(testWorktreeRoot(t), "apps", "web-admin-old", "node_modules", "nuxt", "bin", "nuxt.mjs")
	proc := portProcessInfo{
		PID:         7003,
		ProcessName: "node.exe",
		CommandLine: `node "` + sibling + `" dev --port 3022 --host 127.0.0.1`,
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
	if kill || !blocked {
		t.Fatalf("sibling app path must be blocked, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_otherWorktreeAbsoluteBlocked(t *testing.T) {
	other := filepath.Join(testWorktreeRoot(t), "..", "other-rfx", "apps", "web-admin", "node_modules", "nuxt", "bin", "nuxt.mjs")
	proc := portProcessInfo{
		PID:         7004,
		ProcessName: "node.exe",
		CommandLine: `node "` + other + `" dev --port 3022 --host 127.0.0.1`,
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
	if kill || !blocked {
		t.Fatalf("other worktree absolute path must be blocked, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestClassifyDevPortListener_webAdminSubstringInArgumentBlocked(t *testing.T) {
	proc := portProcessInfo{
		PID:         7005,
		ProcessName: "node.exe",
		CommandLine: `node C:\tmp\scripts\web-admin-helper.js dev --port 3022 --host 127.0.0.1`,
	}
	kill, blocked, _ := classifyDevPortListener("3022", proc, testWorktreeRoot(t))
	if kill || !blocked {
		t.Fatalf("substring web-admin without nuxt ownership must be blocked, got kill=%v blocked=%v", kill, blocked)
	}
}

func TestPathWithinRoot_rejectsSiblingPrefix(t *testing.T) {
	root := filepath.Join(testWorktreeRoot(t), "apps", "web-admin")
	sibling := filepath.Join(testWorktreeRoot(t), "apps", "web-admin-old", "node_modules", "nuxt.mjs")
	if pathWithinRoot(sibling, root) {
		t.Fatalf("expected sibling path %q to be outside %q", sibling, root)
	}
}

func TestCommandLineMatchesNuxtDevPort(t *testing.T) {
	adminApp := testAdminAppDir(t)
	nuxtPath := filepath.Join(adminApp, "node_modules", "nuxt", "bin", "nuxt.mjs")
	cases := []struct {
		cmd  string
		port string
		want bool
	}{
		{`nuxt dev --port 3023 --host 127.0.0.1`, "3023", true},
		{`nuxt dev --port=3022`, "3022", true},
		{`nuxt dev --port 3022`, "3023", false},
		{`node "` + nuxtPath + `" dev --port 3022 --host 127.0.0.1`, "3022", true},
	}
	for _, tc := range cases {
		if got := commandLineMatchesNuxtDevPort(tc.cmd, tc.port); got != tc.want {
			t.Fatalf("commandLineMatchesNuxtDevPort(%q, %q)=%v want %v", tc.cmd, tc.port, got, tc.want)
		}
	}
}

func TestReadProcCmdline_nullSeparated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("proc cmdline test is linux-only")
	}
	joined := joinNullSeparatedArgv([]string{"node", "./node_modules/.bin/../nuxt/bin/nuxt.mjs", "dev", "--port", "3023"})
	if got := readProcCmdlineFromBytes(joined); got != "node ./node_modules/.bin/../nuxt/bin/nuxt.mjs dev --port 3023" {
		t.Fatalf("unexpected parsed cmdline %q", got)
	}
}

func joinNullSeparatedArgv(args []string) []byte {
	var buf []byte
	for i, arg := range args {
		if i > 0 {
			buf = append(buf, 0)
		}
		buf = append(buf, arg...)
	}
	buf = append(buf, 0)
	return buf
}
