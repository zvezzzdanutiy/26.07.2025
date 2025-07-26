package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Ограничение размера файлов во избежание DDOS
const maxUploadSize = 20 * 1024 * 1024

func PostZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONerror(w, "Метод должен быть POST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Ошибка чтения тела запроса", err)
		JSONerror(w, "Ошибка чтения тела запроса")
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var req MyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Косяк с JSON", err)
		JSONerror(w, "Проблема с JSON")
		return
	}

	files, err := os.ReadDir("temp")
	if err != nil {
		log.Printf("Не могу прочитать папку temp", err)
		JSONerror(w, "Не могу прочитать папку temp")
		return
	}
	used := make(map[int]bool)
	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, "file") && strings.HasSuffix(name, ".zip") {
			var n int
			fmt.Sscanf(name, "file%d.zip", &n)
			if n > 0 {
				used[n] = true
			}
		}
	}

	// Находим первый свободный номер
	n := 1
	for ; n <= 100; n++ {
		if !used[n] {
			break
		}
	}

	archiveName := fmt.Sprintf("file%d.zip", n)
	archivePath := filepath.Join("temp", archiveName)

	// Создаём пустой архив
	err = CreateEmptyZip(archivePath)
	if err != nil {
		log.Printf("Проблема с созданием пустого архива", err)
		JSONerror(w, "Проблема с созданием пустого архива")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode("Файл " + archiveName + " добавлен")
}

func DownloadZip(w http.ResponseWriter, r *http.Request) {
	//tasks/123/archive
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[1] != "tasks" || parts[3] != "archive" {
		http.NotFound(w, r)
		return
	}
	taskID := parts[2]

	archivePath := filepath.Join("temp", taskID+".zip")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		log.Printf("Архив не найден", err)
		JSONerror(w, "Архив не найден")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+taskID+".zip\"")
	http.ServeFile(w, r, archivePath)
}

// Handler для добавления файла по ссылке в существующий архив
func AddToZipHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONerror(w, "Метод должен быть POST")
		return
	}

	// Пример: /tasks/file1/addtozip
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] != "addtozip" {
		log.Printf("Не тот path")
		JSONerror(w, "Не тот path")
		return
	}
	archiveName := parts[2] + ".zip"
	archivePath := filepath.Join("temp", archiveName)

	// Проверяем, что архив существует
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		log.Printf("Архив не найден", http.StatusNotFound)
		JSONerror(w, "Архив не найден")
		return
	}

	// Читаем ссылку из тела запроса
	var req MyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Косяк с JSON", err)
		JSONerror(w, "Проблема с JSON")
		return
	}
	if err := validateFileURL(req.URL); err != nil {
		log.Printf("Проблема с валидацией", err)
		JSONerror(w, "Проблема с валидацией")
		return
	}

	// Проверяем количество файлов в архиве
	fileCount := countFilesInZip(archivePath)

	if fileCount == 3 {
		log.Printf("Архив содержит 3 файла. Больше добавлять нельзя.", http.StatusBadRequest)
		JSONerror(w, "Архив содержит 3 файла. Больше добавлять нельзя.")
		resp := ZipResponse{
			ArchiveURL: "http://localhost:8080/temp/" + archiveName,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	} else {
		err := AddToZip(archivePath, req.URL)
		if err != nil {
			return
		}
		json.NewEncoder(w).Encode("Файл добавлен в архив")
		return
	}

}

// Хендлер получения статуса
func GetZipStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Метод getstatus - прежде всего GET")
		JSONerror(w, "Метод должен быть GET")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[1] != "getstatus" {
		log.Printf("Не тот path", http.StatusBadRequest)
		JSONerror(w, "Не тот path")
		return
	}
	archiveName := parts[2] + ".zip"
	archivePath := filepath.Join("temp", archiveName)

	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		log.Printf("Архив не найден", http.StatusNotFound)
		JSONerror(w, "Архив не найден")
		return
	}

	fileCount := countFilesInZip(archivePath)
	status := ""
	switch {
	case fileCount == 0:
		status = "архив создан (пустой)"
	case fileCount < 3:
		status = "архив содержит " + strconv.Itoa(fileCount) + " файлов"
	case fileCount == 3:
		status = "архив заполнен"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"archive_url": "http://localhost:8080/temp/" + archiveName},
		)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}
