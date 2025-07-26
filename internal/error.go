package internal

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func JSONerror(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"error": err})
}

// Простенькая валидация (чтобы было)
func validateFileURL(url string) error {
	if url == "" {
		return fmt.Errorf("URL не может быть пустым")
	}

	validExt := []string{".jpg", ".jpeg", ".png", ".pdf"}
	isValid := false
	log.Printf("Проверяю", url)
	for _, ext := range validExt {
		if strings.HasSuffix(url, ext) {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("URL имеет неподдерживаемый формат")
	}

	return nil
}
