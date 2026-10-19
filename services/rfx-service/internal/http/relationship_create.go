package http

import (
	"net/http"

	"github.com/freight-platform/rfx-service/internal/http/handlers"
	"github.com/freight-platform/rfx-service/internal/http/userauth"
)

// WithRfxRelationshipCreate mounts the authenticated RFx relationship create command.
// Other routes keep the handler passed in. The route fails closed when the JWT secret is empty.
func WithRfxRelationshipCreate(next http.Handler, jwtSecret string, creator handlers.RfxRelationshipCreator) http.Handler {
	handler := handlers.NewRfxRelationshipCreateHandler(creator)
	protected := userauth.RequireJWT(jwtSecret)(http.HandlerFunc(handler.Create))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if _, _, ok := handlers.RfxRelationshipCreateTarget(r.URL.Path); ok {
				protected.ServeHTTP(w, r)
				return
			}
		}
		if next != nil {
			next.ServeHTTP(w, r)
		}
	})
}
