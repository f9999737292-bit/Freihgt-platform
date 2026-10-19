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
	jwt := userauth.RequireJWT(jwtSecret)
	protectedCreate := jwt(http.HandlerFunc(handler.Create))
	protectedStatus := jwt(http.HandlerFunc(handler.Status))
	protectedCancel := jwt(http.HandlerFunc(handler.Cancel))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if _, _, ok := handlers.ShipmentRelationshipCancelTarget(r.URL.Path); ok {
				protectedCancel.ServeHTTP(w, r)
				return
			}
			if _, ok := handlers.ShipmentRelationshipCreateID(r.URL.Path); ok {
				protectedCreate.ServeHTTP(w, r)
				return
			}
		case http.MethodGet:
			if _, ok := handlers.ShipmentRelationshipCreateID(r.URL.Path); ok {
				protectedStatus.ServeHTTP(w, r)
				return
			}
		}
		if next != nil {
			next.ServeHTTP(w, r)
		}
	})
}
