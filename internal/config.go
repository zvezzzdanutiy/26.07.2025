package internal

type MyRequest struct {
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type ZipResponse struct {
	ArchiveURL string `json:"archive_link,omitempty"`
	Error      string `json:"error,omitempty"`
}
