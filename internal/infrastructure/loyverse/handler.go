package loyverse

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
)

// ServeHTTP implementa http.Handler para el endpoint del webhook.
func (h *LoyverseWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.logger.log("payload recibido (%d bytes): %s", len(body), string(body))

	if h.secret != "" {
		signature := r.Header.Get("X-Loyverse-Signature")
		if !h.verifySignature(body, signature) {
			h.logger.log("firma inválida — petición rechazada")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.log("error parseando JSON: %v", err)
		// 200 para evitar que Loyverse reintente indefinidamente por errores de formato.
		w.WriteHeader(http.StatusOK)
		return
	}

	h.processPayload(payload)
	w.WriteHeader(http.StatusOK)
}

// verifySignature verifica la firma HMAC-SHA256 del payload.
func (h *LoyverseWebhook) verifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	expectedSignature := base64.StdEncoding.EncodeToString(expectedMAC)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
