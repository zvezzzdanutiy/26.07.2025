package internal

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type AddFileRequest struct {
	URL string `json:"url"`
}

type AddFileResponse struct {
	ArchiveURL string `json:"archive_url,omitempty"`
	Error      string `json:"error,omitempty"`
}

func CreateArchive(taskID string, urls []string) error {
	archivePath := filepath.Join("temp", taskID+".zip")
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	for _, urlStr := range urls {
		resp, err := http.Get(urlStr)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			return err
		}
		fileName := filepath.Base(parsedURL.Path)
		if fileName == "" || !strings.Contains(fileName, ".") {
			// Попробуем определить расширение по Content-Type
			contentType := resp.Header.Get("Content-Type")
			ext := ""
			switch contentType {
			case "image/jpeg":
				ext = ".jpeg"
			case "application/pdf":
				ext = ".pdf"
			}
			fileName = "file" + ext
		}

		f, err := zipWriter.Create(fileName)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddFileToTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод должен быть POST", http.StatusMethodNotAllowed)
		return
	}

	// Пример: /tasks/123/files
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Не тот path", http.StatusBadRequest)
		return
	}
	taskID := parts[2]

	// Диагностика: выводим тело запроса
	body, _ := io.ReadAll(r.Body)
	fmt.Println("Тело запроса:", string(body))
	r.Body = io.NopCloser(bytes.NewBuffer(body)) // чтобы Decode сработал дальше

	var req AddFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Косяк с JSON", http.StatusBadRequest)
		return
	}

	// Создаём архив с одной картинкой
	err := CreateArchive(taskID, []string{req.URL})
	if err != nil {
		resp := AddFileResponse{Error: err.Error()}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := AddFileResponse{ArchiveURL: "http://localhost:8080/tasks/" + taskID + "/archive"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func ArchiveDownloadHandler(w http.ResponseWriter, r *http.Request) {
	// Пример: /tasks/123/archive
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[1] != "tasks" || parts[3] != "archive" {
		http.NotFound(w, r)
		return
	}
	taskID := parts[2]

	archivePath := filepath.Join("temp", taskID+".zip")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		http.Error(w, "Archive not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+taskID+".zip\"")
	http.ServeFile(w, r, archivePath)
}
