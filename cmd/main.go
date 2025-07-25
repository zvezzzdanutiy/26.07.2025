package main

import (
	"net/http"
	"workmate/internal"
)

func main() {
	http.HandleFunc("/tasks/123/files", internal.AddFileToTaskHandler)
	http.HandleFunc("/tasks/123/archive", internal.ArchiveDownloadHandler)
	http.ListenAndServe(":8080", nil)
}
