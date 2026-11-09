package document

import (
	"net/http"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

// RejectSigningRecovery closes the public mutation. RequireOperatorShipmentRead
// is a shipment read guard and is not a document write-authority contract.
func RejectSigningRecovery(w http.ResponseWriter, _ *http.Request) {
	respond.Error(w, apperrors.Forbidden("signing recovery has no public write authority"))
}
