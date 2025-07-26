package main

import (
	"fmt"
	"net/http"
	"workmate/internal"
)

func main() {
	http.HandleFunc("/tasks/files", internal.PostZip)
	http.HandleFunc("/tasks/archive", internal.DownloadZip)
	http.HandleFunc("/tasks/", internal.AddToZipHandler)
	http.HandleFunc("/getstatus/", internal.GetZipStatus)
	fmt.Println("Сервер запущен на http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
