package pprof

import (
	"net/http/pprof"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Enabled reports whether PPROF_ENABLED=true.
func Enabled() bool {
	raw := os.Getenv("PPROF_ENABLED")
	if raw == "" {
		return false
	}
	enabled, err := strconv.ParseBool(raw)
	return err == nil && enabled
}

// Mount registers /debug/pprof/* handlers when PPROF_ENABLED=true.
// Disabled by default; do not enable in production without access control.
func Mount(r chi.Router) {
	if !Enabled() {
		return
	}

	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
}
