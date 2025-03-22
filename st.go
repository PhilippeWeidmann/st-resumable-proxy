package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/uuid"
)

func main() {
	http.HandleFunc("/upload", uploadHandler)
	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	containerUUID := uuid.New().String()
	containerPath := fmt.Sprintf("containers/%s", containerUUID)

	err := os.MkdirAll(containerPath, os.ModePerm)
	if err != nil {
		http.Error(w, "Error creating directory", http.StatusInternalServerError)
		return
	}

	chunkSize := 50 * 1024 * 1024
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
			chunkFilePath := fmt.Sprintf("%s/%d", containerPath, chunkIndex)
			if err := writeChunk(chunkFilePath, chunkBuffer[:chunkSize]); err != nil {
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
		chunkFilePath := fmt.Sprintf("%s/%d", containerPath, chunkIndex)
		if err := writeChunk(chunkFilePath, chunkBuffer); err != nil {
			http.Error(w, "Error writing final chunk", http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprintln(w, "Upload complete.")
}

func writeChunk(path string, data []byte) error {
	chunkFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer chunkFile.Close()

	_, err = chunkFile.Write(data)
	return err
}
