package http

import (
	"net/http"

	"github.com/freight-platform/shipment-service/internal/http/handlers"
	"github.com/freight-platform/shipment-service/internal/http/userauth"
)

// WithShipmentRelationshipCreate mounts the authenticated shipment relationship create command.
// Other routes keep the handler passed in. The route fails closed when the JWT secret is empty.
func WithShipmentRelationshipCreate(next http.Handler, jwtSecret string, creator handlers.RelationshipCreator) http.Handler {
	handler := handlers.NewRelationshipCreateHandler(creator)
	protected := userauth.RequireJWT(jwtSecret)(http.HandlerFunc(handler.Create))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if _, ok := handlers.ShipmentRelationshipCreateID(r.URL.Path); ok {
				protected.ServeHTTP(w, r)
				return
			}
		}
		if next != nil {
			next.ServeHTTP(w, r)
		}
	})
}
