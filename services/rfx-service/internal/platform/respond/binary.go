package respond

import (
	"fmt"
	"net/http"
)

func BinaryAttachment(w http.ResponseWriter, status int, contentType, filename string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if len(data) > 0 {
		_, _ = w.Write(data)
	}
}
