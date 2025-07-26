package internal

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

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
