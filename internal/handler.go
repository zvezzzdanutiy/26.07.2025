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
	"strconv"
	"strings"
)

type MyRequest struct {
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type ZipResponse struct {
	ArchiveURL string `json:"archive_link,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Функция получения ссылок на существующие архивы
func GetReadyLinks() []string {
	var readyLinks []string
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("file%d.zip", i)
		path := filepath.Join("temp", name)
		if _, err := os.Stat(path); err == nil {
			readyLinks = append(readyLinks, "http://localhost:8080/temp/"+name)
		}
	}
	return readyLinks
}

func PostZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод должен быть POST", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	fmt.Println("Тело запроса:", string(body))
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var req MyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Косяк с JSON", http.StatusBadRequest)
		return
	}

	files, err := os.ReadDir("temp")
	if err != nil {
		http.Error(w, "Не могу прочитать папку temp", http.StatusInternalServerError)
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
		resp := ZipResponse{Error: err.Error()}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode("Файл " + archiveName + " добавлен")
}

// Функция для создания архива по заданному пути
func CreateZip(archivePath, urlStr string) error {
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	resp, err := http.Get(urlStr)
	if err != nil {
		return err
	}
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fileName := filepath.Base(parsedURL.Path)
	if fileName == "" || !strings.Contains(fileName, ".") {
		// Попробуем определить расширение по Content-Type
		contentType := resp.Header.Get("Content-Type")
		ext := ""
		switch contentType {
		case "image/jpeg":
			ext = ".jpeg"
		case "image/png":
			ext = ".png"
		case "application/pdf":
			ext = ".pdf"
		}
		fileName = fileName[:len(fileName)-1] + ext
	}

	// Имя файла внутри архива — любое, например, file
	f, err := zipWriter.Create(fileName)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func DownloadZip(w http.ResponseWriter, r *http.Request) {
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

// Handler для добавления файла по ссылке в существующий архив
func AddToZipHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод должен быть POST", http.StatusMethodNotAllowed)
		return
	}

	// Пример: /tasks/file1/addtozip
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] != "addtozip" {
		http.Error(w, "Не тот path", http.StatusBadRequest)
		return
	}
	archiveName := parts[2] + ".zip"
	archivePath := filepath.Join("temp", archiveName)

	// Проверяем, что архив существует
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		http.Error(w, "Архив не найден", http.StatusNotFound)
		return
	}

	// Читаем ссылку из тела запроса
	var req MyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Косяк с JSON", http.StatusBadRequest)
		return
	}

	// Добавляем файл в архив

	// Проверяем количество файлов в архиве
	fileCount := countFilesInZip(archivePath)

	if fileCount == 3 {
		http.Error(w, "Архив содержит 3 файла. Больше добавлять нельзя.", http.StatusBadRequest)
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

// Функция для добавления файла в существующий архив
func AddToZip(archivePath, urlStr string) error {
	zipFile, err := os.OpenFile(archivePath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	stat, err := zipFile.Stat()
	if err != nil {
		return err
	}
	zipBytes := make([]byte, stat.Size())
	_, err = zipFile.Read(zipBytes)
	if err != nil && err != io.EOF {
		return err
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Пересоздание архива с копированием старых файлов
	oldZipReader, err := zip.NewReader(bytes.NewReader(zipBytes), stat.Size())
	if err != nil {
		return err
	}
	for _, f := range oldZipReader.File {
		w, err := zipWriter.Create(f.Name)
		if err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		r.Close()
		if err != nil {
			return err
		}
	}

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
		fileName = "file"
	}

	w, err := zipWriter.Create(fileName)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}

	zipWriter.Close()

	return os.WriteFile(archivePath, buf.Bytes(), 0644)
}

// Функция для создания пустого архива
func CreateEmptyZip(archivePath string) error {
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	return nil
}

func countFilesInZip(archivePath string) int {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0
	}
	defer reader.Close()
	return len(reader.File)
}

// Хендлер получения статуса
func GetZipStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод должен быть GET", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[1] != "getstatus" {
		http.Error(w, "Не тот path", http.StatusBadRequest)
		return
	}
	archiveName := parts[2] + ".zip"
	archivePath := filepath.Join("temp", archiveName)

	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		http.Error(w, "Архив не найден", http.StatusNotFound)
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
