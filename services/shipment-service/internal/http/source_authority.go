package http

import (
	"net/http"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/http/handlers"
	"github.com/freight-platform/shipment-service/internal/http/sourceauth"
	"github.com/freight-platform/shipment-service/internal/service"
)

// WithShipmentSourceAuthority mounts the shipment owner callback in front of the service router.
// The dedicated revalidation credential is the only authority for these two routes.
func WithShipmentSourceAuthority(next http.Handler, token string, authority *service.ShipmentSourceAuthority) http.Handler {
	handler := handlers.NewShipmentSourceAuthorityHandler(authority)
	protect := sourceauth.Middleware(token)
	confirm := protect(http.HandlerFunc(handler.Confirm))
	revalidate := protect(http.HandlerFunc(handler.Revalidate))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == domain.ShipmentSourceConfirmPath:
			confirm.ServeHTTP(w, r)
			return
		case r.Method == http.MethodPost && r.URL.Path == domain.ShipmentSourceRevalidatePath:
			revalidate.ServeHTTP(w, r)
			return
		default:
			if next != nil {
				next.ServeHTTP(w, r)
			}
		}
	})
}
