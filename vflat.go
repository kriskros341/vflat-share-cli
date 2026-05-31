package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const createURL = "https://send.vflat.com/api/create"

// httpClient is shared for the vFlat API and downloads. No timeout: scans can
// be large and downloads run concurrently.
var httpClient = &http.Client{Timeout: 0}

// fileInfo describes one file advertised by the vFlat server. The `id` field
// may be sent as a string or a number, so it is decoded raw and normalised.
type fileInfo struct {
	ID       json.RawMessage `json:"id"`
	Filename string          `json:"filename"`
}

func (f fileInfo) id() string {
	return strings.Trim(string(f.ID), `"`)
}

func getJSON(url string, out any) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s -> %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func createSession() (string, error) {
	var result struct {
		UUID string `json:"uuid"`
	}
	if err := getJSON(createURL, &result); err != nil {
		return "", err
	}
	if result.UUID == "" {
		return "", fmt.Errorf("server returned an empty uuid")
	}
	return result.UUID, nil
}

func fetchFileList(baseAddress string, port int, uuid string) ([]fileInfo, error) {
	url := fmt.Sprintf("http://%s:%d/%s/info", baseAddress, port, uuid)
	fmt.Println(url)
	var result struct {
		SendFiles []fileInfo `json:"sendFiles"`
	}
	if err := getJSON(url, &result); err != nil {
		return nil, err
	}
	return result.SendFiles, nil
}

func downloadFile(url, dest string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func downloadAll(baseAddress string, port int, uuid string, files []fileInfo, outDir string) {
	fmt.Printf("Starting %d concurrent downloads...\n", len(files))
	var wg sync.WaitGroup
	for _, fi := range files {
		wg.Add(1)
		go func(fi fileInfo) {
			defer wg.Done()
			url := fmt.Sprintf("http://%s:%d/%s/sendFile/%s/%s",
				baseAddress, port, uuid, fi.id(), fi.Filename)
			dest := filepath.Join(outDir, fi.Filename)
			if err := downloadFile(url, dest); err != nil {
				fmt.Printf("Failed to download %s: %v\n", fi.Filename, err)
				return
			}
			fmt.Printf("Downloaded: %s\n", fi.Filename)
		}(fi)
	}
	wg.Wait()
	fmt.Println("Downloads finished!")
}
