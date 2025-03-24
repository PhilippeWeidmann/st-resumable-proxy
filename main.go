package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const chunkSize = 50 * 1024 * 1024

func main() {
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/upload/resume", uploadResumableHandler)
	http.ListenAndServe(":8080", nil)
}

func uploadResumableHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uploadHost := r.Header.Get("x-upload-host")

	q := r.URL.Query()
	containerUUID := q.Get("containerUUID")
	uploadFileUUID := q.Get("uploadFileUUID")

	if uploadHost == "" || containerUUID == "" || uploadFileUUID == "" {
		http.Error(w, "Missing required query parameters", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodHead {
		uploadOffset := getServerOffset(uploadHost, containerUUID, uploadFileUUID)

		w.Header().Set("Upload-Complete", "?0")
		w.Header().Set("Upload-Offset", strconv.FormatInt(uploadOffset, 10))
		w.WriteHeader(200)
	} else if r.Method == http.MethodPatch {
		uploadOffset, ok := getClientOffset(r)
		if !ok {
			w.WriteHeader(400)
			w.Write([]byte("invalid or missing Upload-Offset header\n"))
			return
		}

		chunkIndex := int(uploadOffset / chunkSize)

		ingestError := ingestChunks(chunkIndex, r, w, uploadHost, containerUUID, uploadFileUUID)
		if ingestError != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Upload-Complete", "?1")
		w.WriteHeader(200)
	} else {
		fmt.Println("Invalid request method")
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

func getClientOffset(r *http.Request) (int64, bool) {
	offset, err := strconv.Atoi(r.Header.Get("Upload-Offset"))
	if err != nil {
		return 0, false
	}
	return int64(offset), true
}

func getServerOffset(uploadHost string, containerUUID string, uploadFileUUID string) int64 {
	var offset int64
	chunkIndex := 0

	for {
		if !checkChunkExists(uploadHost, containerUUID, uploadFileUUID, chunkIndex) {
			break
		}
		offset += chunkSize
		chunkIndex++
	}

	return offset
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uploadHost := r.Header.Get("x-upload-host")
	rawContentLength := r.Header.Get("content-length")

	q := r.URL.Query()
	containerUUID := q.Get("containerUUID")
	uploadFileUUID := q.Get("uploadFileUUID")

	contentLength, err := strconv.Atoi(rawContentLength)

	if uploadHost == "" || containerUUID == "" || uploadFileUUID == "" || rawContentLength == "" || err != nil || contentLength <= 0 {
		http.Error(w, "Missing required query parameters", http.StatusBadRequest)
		return
	}

	resumeUploadUrl := fmt.Sprintf("http://proxyman.debug:8080/upload/resume?containerUUID=%s&uploadFileUUID=%s", containerUUID, uploadFileUUID)

	w.Header().Set("Location", resumeUploadUrl)
	w.Header().Set("Upload-Draft-Interop-Version", "6")
	w.WriteHeader(104)
	w.Header().Del("Upload-Draft-Interop-Version")

	ingestError := ingestChunks(0, r, w, uploadHost, containerUUID, uploadFileUUID)
	if ingestError != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Upload-Complete", "?1")
	w.WriteHeader(201)
}

func ingestChunks(startIndex int, r *http.Request, w http.ResponseWriter, uploadHost string, containerUUID string, uploadFileUUID string) error {
	chunkIndex := startIndex
	chunkBuffer := make([]byte, 0, chunkSize)

	for {
		buf := make([]byte, 4096)
		n, readErr := r.Body.Read(buf)
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("error reading body: %v", readErr)
		}
		chunkBuffer = append(chunkBuffer, buf[:n]...)

		// Flush a full chunk
		for len(chunkBuffer) >= chunkSize {
			if err := writeRemoteChunk(uploadHost, containerUUID, uploadFileUUID, chunkIndex, false, chunkBuffer[:chunkSize]); err != nil {
				return fmt.Errorf("error writing chunk %v", err)
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
			return fmt.Errorf("error writing final chunk: %v", err)
		}
	}

	return nil
}

func writeRemoteChunk(uploadHost string, containerUUID string, uploadFileUUID string, chunkIndex int, isLastChunk bool, data []byte) error {
	chunkDirectoryPath := fmt.Sprintf("%s/%s", containerUUID, uploadFileUUID)
	if _, err := os.Stat(chunkDirectoryPath); os.IsNotExist(err) {
		os.MkdirAll(chunkDirectoryPath, 0755)
	}

	chunkFilePath := fmt.Sprintf("%s/%s/%d", containerUUID, uploadFileUUID, chunkIndex)

	chunkFile, err := os.Create(chunkFilePath)
	if err != nil {
		return err
	}
	defer chunkFile.Close()

	_, err = chunkFile.Write(data)
	time.Sleep(1 * time.Second)
	return err
}

func checkChunkExists(uploadHost string, containerUUID string, uploadFileUUID string, chunkIndex int) bool {
	chunkFilePath := fmt.Sprintf("%s/%s/%d", containerUUID, uploadFileUUID, chunkIndex)

	if _, err := os.Stat(chunkFilePath); os.IsNotExist(err) {
		return false
	}

	return true
}
