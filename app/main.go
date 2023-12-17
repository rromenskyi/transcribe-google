package main

import (
    "bytes"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
)

const apiKey = ""

func main() {
    if len(os.Args) < 4 {
	log.Fatal("Usage: go run main.go <path_to_audio_file> <language_code> <sample_rate>")
    }

    audioFilePath := os.Args[1]
    languageCode := os.Args[2]
    sampleRate := os.Args[3]

    // Рекодирование аудиофайла с помощью FFmpeg
    cmd := exec.Command("ffmpeg", "-i", audioFilePath, "-ar", sampleRate, "-ac", "1", "-f", "wav", "-")
    data, err := cmd.Output()
    if err != nil {
	log.Fatalf("Failed to re-encode audio file with FFmpeg: %v", err)
    }

    encodedData := base64.StdEncoding.EncodeToString(data)

    requestBody, err := json.Marshal(map[string]interface{}{
	"config": map[string]interface{}{
	    "encoding":        "LINEAR16",
	    "sampleRateHertz": sampleRate,
	    "languageCode":    languageCode,
	},
	"audio": map[string]interface{}{
	    "content": encodedData,
	},
    })

    if err != nil {
	log.Fatalf("Failed to encode audio data: %v", err)
    }

    resp, err := http.Post("https://speech.googleapis.com/v1/speech:recognize?key="+apiKey, "application/json", bytes.NewBuffer(requestBody))
    if err != nil {
	log.Fatalf("Failed to call Google Cloud API: %v", err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
	log.Fatalf("Failed to read response: %v", err)
    }

    var response struct {
	Results []struct {
	    Alternatives []struct {
		Transcript string `json:"transcript"`
	    } `json:"alternatives"`
	} `json:"results"`
    }

    if err := json.Unmarshal(body, &response); err != nil {
	log.Fatalf("Failed to parse response: %v", err)
    }

    for _, result := range response.Results {
	fmt.Printf("Transcription: %s\n", result.Alternatives[0].Transcript)
    }
}
