package studio

import "strings"

type nuxtBootState struct {
	ready  bool
	fatal  bool
	reason string
}

func evaluateNuxtDevBootLog(text string, port string, portInUse func(string) bool) nuxtBootState {
	if strings.Contains(text, "Nuxt Nitro server built") {
		return nuxtBootState{ready: true}
	}
	if strings.Contains(text, "Vite server built") &&
		strings.Contains(text, "Local:") &&
		strings.Contains(text, ":"+port) {
		if strings.Contains(text, "ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL") && !portInUse(port) {
			return nuxtBootState{reason: "pnpm wrapper exited before listener confirmed"}
		}
		if strings.Contains(text, "EBUSY") &&
			(strings.Contains(text, "dev") || strings.Contains(text, ".nuxt")) {
			if portInUse(port) {
				return nuxtBootState{ready: true}
			}
			return nuxtBootState{reason: "transient dev busy after vite built; waiting for listener"}
		}
		if portInUse(port) {
			return nuxtBootState{ready: true}
		}
		return nuxtBootState{reason: "vite built; waiting for listener bind"}
	}
	if strings.Contains(text, "Another Nuxt dev server is already running") {
		return nuxtBootState{fatal: true, reason: "stale nuxt lock detected"}
	}
	if strings.Contains(text, "EBUSY") && strings.Contains(text, ".nuxt") {
		return nuxtBootState{fatal: true, reason: ".nuxt/dev locked"}
	}
	if strings.Contains(text, "ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL") {
		if portInUse(port) {
			return nuxtBootState{reason: "pnpm wrapper exited; waiting for nuxt listener"}
		}
		// Wrapper may exit before the child listener binds; keep polling until timeout.
		return nuxtBootState{reason: "pnpm wrapper exited before listener confirmed"}
	}
	return nuxtBootState{}
}
