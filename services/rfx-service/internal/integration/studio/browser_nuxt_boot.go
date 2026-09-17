package studio

import "strings"

type nuxtBootState struct {
	ready  bool
	fatal  bool
	reason string
}

func evaluateNuxtDevBootLog(text string, port string, portInUse func(string) bool) nuxtBootState {
	if strings.Contains(text, "Another Nuxt dev server is already running") {
		return nuxtBootState{fatal: true, reason: "stale nuxt lock detected"}
	}
	if strings.Contains(text, "EBUSY") && strings.Contains(text, ".nuxt") {
		return nuxtBootState{fatal: true, reason: ".nuxt/dev locked"}
	}
	if strings.Contains(text, "Nuxt Nitro server built") {
		return nuxtBootState{ready: true}
	}
	if portInUse(port) &&
		strings.Contains(text, "Local:") &&
		strings.Contains(text, "Vite server built") {
		// pnpm launcher may exit after spawning the real nuxt listener on Windows.
		return nuxtBootState{ready: true}
	}
	if strings.Contains(text, "ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL") {
		if portInUse(port) {
			return nuxtBootState{reason: "pnpm wrapper exited; waiting for nuxt listener"}
		}
		return nuxtBootState{fatal: true, reason: "pnpm exec resolution error"}
	}
	return nuxtBootState{}
}
