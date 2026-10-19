package http

import (
	"net/http"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/http/handlers"
	"github.com/freight-platform/rfx-service/internal/http/sourceauth"
	"github.com/freight-platform/rfx-service/internal/service"
)

// WithRfxSourceAuthority mounts the RFx owner callback in front of the service router.
// The dedicated revalidation credential is the only authority for these two routes.
func WithRfxSourceAuthority(next http.Handler, token string, authority *service.RfxSourceAuthority) http.Handler {
	handler := handlers.NewRfxSourceAuthorityHandler(authority)
	protect := sourceauth.Middleware(token)
	confirm := protect(http.HandlerFunc(handler.Confirm))
	revalidate := protect(http.HandlerFunc(handler.Revalidate))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == domain.RfxSourceConfirmPath:
			confirm.ServeHTTP(w, r)
			return
		case r.Method == http.MethodPost && r.URL.Path == domain.RfxSourceRevalidatePath:
			revalidate.ServeHTTP(w, r)
			return
		default:
			if next != nil {
				next.ServeHTTP(w, r)
			}
		}
	})
}
