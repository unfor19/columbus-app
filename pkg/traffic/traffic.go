package traffic

import (
	"io"
	"log"
	"net/http"
	"os"
)

type HttpHeader struct {
	Name  string
	Value string
}

type UrlResponse struct {
	StatusCode int
	Headers    []HttpHeader
}

type RequestUrlResponse struct {
	statusCode int
	body       string
	headers    http.Header
}

// Source: https://golangcode.com/download-a-file-from-a-url/
// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(filepath string, url string) error {
	log.Println("Downloading the file:", url)
	log.Println("Filepath:", filepath)
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func GetRequestUrlResponse(u string) http.Response {
	resp, err := http.Get(u)
	if err != nil {
		log.Println(err)
	}
	return *resp
}
