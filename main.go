package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const chunkSize = 50 * 1024 * 1024

func main() {
	http.HandleFunc("/upload", uploadHandler)
	http.ListenAndServe(":8080", nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uploadHost := r.Header.Get("x-upload-host")

	q := r.URL.Query()
	containerUUID := q.Get("containerUUID")
	uploadFileUUID := q.Get("uploadFileUUID")

	if uploadHost == "" || containerUUID == "" || uploadFileUUID == "" {
		http.Error(w, "Missing required query parameters", http.StatusBadRequest)
		return
	}

	chunkIndex := 0
	chunkBuffer := make([]byte, 0, chunkSize)

	for {
		buf := make([]byte, 4096)
		n, readErr := r.Body.Read(buf)
		if readErr != nil && readErr != io.EOF {
			http.Error(w, "Error reading body", http.StatusInternalServerError)
			return
		}
		chunkBuffer = append(chunkBuffer, buf[:n]...)

		// Flush a full chunk
		for len(chunkBuffer) >= chunkSize {
			if err := writeRemoteChunk(uploadHost, containerUUID, uploadFileUUID, chunkIndex, false, chunkBuffer[:chunkSize]); err != nil {
				http.Error(w, "Error writing chunk", http.StatusInternalServerError)
				return
			}
			chunkBuffer = chunkBuffer[chunkSize:]
			chunkIndex++
		}

		if readErr == io.EOF || n == 0 {
			break
		}
	}

	// Write any leftover chunk
	if len(chunkBuffer) > 0 {
		if err := writeRemoteChunk(uploadHost, containerUUID, uploadFileUUID, chunkIndex, true, chunkBuffer); err != nil {
			http.Error(w, "Error writing final chunk", http.StatusInternalServerError)
			return
		}
	}
}

func writeRemoteChunk(uploadHost string, containerUUID string, uploadFileUUID string, chunkIndex int, isLastChunk bool, data []byte) error {
	rawLastChunk := 0
	if isLastChunk {
		rawLastChunk = 1
	}

	uploadURL := fmt.Sprintf("https://%s/api/uploadChunk/%s/%s/%d/%d", uploadHost, containerUUID, uploadFileUUID, chunkIndex, rawLastChunk)

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "ST-Resumable-Proxy/1.0")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed, got status: %d", resp.StatusCode)
	}

	return nil
}

func checkChunkExists(uploadHost string, containerUUID string, uploadFileUUID string, chunkIndex int) bool {
	// We always check for a size of 50mo because we don't know the size of the last chunk, if it's less than 50mo we will get a 404 and client will retry from scratch
	chunkExistsURL := fmt.Sprintf("https://%s/api/mobile/containers/%s/files/%s/chunks/%d/exists?chunk_size=%d", uploadHost, containerUUID, uploadFileUUID, chunkIndex, chunkSize)

	req, err := http.NewRequest("GET", chunkExistsURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "ST-Resumable-Proxy/1.0")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)

	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return false
	}

	return string(body) == "true"
}
