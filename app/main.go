package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://speech.googleapis.com/v1/speech:recognize"
	usageText       = `Usage:
  transcribe-google -input <path> -language <code> -sample-rate <hz> [-api-key <key>]
  transcribe-google <path_to_audio_file> <language_code> <sample_rate>
`
)

var errShowUsage = errors.New("show usage")

type cliConfig struct {
	inputFile    string
	languageCode string
	sampleRate   int
	apiKey       string
	endpoint     string
}

type recognizeRequest struct {
	Config recognitionConfig `json:"config"`
	Audio  recognitionAudio  `json:"audio"`
}

type recognitionConfig struct {
	Encoding        string `json:"encoding"`
	SampleRateHertz int    `json:"sampleRateHertz"`
	LanguageCode    string `json:"languageCode"`
}

type recognitionAudio struct {
	Content string `json:"content"`
}

type recognizeResponse struct {
	Results []recognitionResult `json:"results"`
	Error   *apiError           `json:"error,omitempty"`
}

type recognitionResult struct {
	Alternatives []recognitionAlternative `json:"alternatives"`
}

type recognitionAlternative struct {
	Transcript string `json:"transcript"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func main() {
	log.SetFlags(0)

	if err := run(os.Args[1:], os.Stdout); err != nil {
		if errors.Is(err, errShowUsage) {
			_, _ = fmt.Fprint(os.Stdout, usageText)
			return
		}
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}

	audioData, err := convertAudio(cfg.inputFile, cfg.sampleRate)
	if err != nil {
		return err
	}

	transcript, err := transcribe(cfg, audioData)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(stdout, transcript)
	return err
}

func parseArgs(args []string) (cliConfig, error) {
	if len(args) == 3 && !strings.HasPrefix(args[0], "-") {
		sampleRate, err := strconv.Atoi(args[2])
		if err != nil {
			return cliConfig{}, fmt.Errorf("invalid sample rate %q: %w", args[2], err)
		}

		cfg := cliConfig{
			inputFile:    args[0],
			languageCode: args[1],
			sampleRate:   sampleRate,
			apiKey:       os.Getenv("GOOGLE_API_KEY"),
			endpoint:     defaultEndpoint,
		}

		return cfg, validateConfig(cfg)
	}

	var cfg cliConfig

	fs := flag.NewFlagSet("transcribe-google", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.inputFile, "input", "", "path to input audio file")
	fs.StringVar(&cfg.languageCode, "language", "", "BCP-47 language code, for example en-US")
	fs.IntVar(&cfg.sampleRate, "sample-rate", 0, "audio sample rate in Hz")
	fs.StringVar(&cfg.apiKey, "api-key", os.Getenv("GOOGLE_API_KEY"), "Google Cloud API key")
	fs.StringVar(&cfg.endpoint, "endpoint", defaultEndpoint, "Google Speech-to-Text REST endpoint")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return cliConfig{}, errShowUsage
		}
		return cliConfig{}, fmt.Errorf("%w\n\n%s", err, usageText)
	}

	if len(fs.Args()) != 0 {
		return cliConfig{}, fmt.Errorf("unexpected positional arguments: %s\n\n%s", strings.Join(fs.Args(), " "), usageText)
	}

	if err := validateConfig(cfg); err != nil {
		return cliConfig{}, err
	}

	return cfg, nil
}

func validateConfig(cfg cliConfig) error {
	if cfg.inputFile == "" {
		return errors.New("missing input audio file\n\n" + usageText)
	}

	if _, err := os.Stat(cfg.inputFile); err != nil {
		return fmt.Errorf("cannot access input file %q: %w", cfg.inputFile, err)
	}

	if cfg.languageCode == "" {
		return errors.New("missing language code\n\n" + usageText)
	}

	if cfg.sampleRate <= 0 {
		return errors.New("sample rate must be greater than zero\n\n" + usageText)
	}

	if cfg.apiKey == "" {
		return errors.New("missing API key: set GOOGLE_API_KEY or pass -api-key")
	}

	parsedEndpoint, err := url.Parse(cfg.endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint %q: %w", cfg.endpoint, err)
	}
	if parsedEndpoint.Host == "" || (parsedEndpoint.Scheme != "https" && parsedEndpoint.Scheme != "http") {
		return fmt.Errorf("endpoint must include http:// or https:// and a host: %q", cfg.endpoint)
	}

	return nil
}

func convertAudio(audioFilePath string, sampleRate int) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, errors.New("ffmpeg is required in PATH")
	}

	cmd := exec.Command(
		"ffmpeg",
		"-v", "error",
		"-i", audioFilePath,
		"-ar", strconv.Itoa(sampleRate),
		"-ac", "1",
		"-f", "s16le",
		"-",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ffmpeg conversion failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	if stdout.Len() == 0 {
		return nil, errors.New("ffmpeg returned empty audio output")
	}

	maxSyncBytes := cfgMaxSyncBytes(sampleRate)
	if stdout.Len() > maxSyncBytes {
		return nil, fmt.Errorf(
			"audio appears longer than 60 seconds after conversion; synchronous recognize is intended for clips up to 1 minute",
		)
	}

	return stdout.Bytes(), nil
}

func cfgMaxSyncBytes(sampleRate int) int {
	return sampleRate * 2 * 60
}

func transcribe(cfg cliConfig, audioData []byte) (string, error) {
	requestBody, err := json.Marshal(recognizeRequest{
		Config: recognitionConfig{
			Encoding:        "LINEAR16",
			SampleRateHertz: cfg.sampleRate,
			LanguageCode:    cfg.languageCode,
		},
		Audio: recognitionAudio{
			Content: base64.StdEncoding.EncodeToString(audioData),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode request body: %w", err)
	}

	endpoint, err := url.Parse(cfg.endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint %q: %w", cfg.endpoint, err)
	}
	query := endpoint.Query()
	query.Set("key", cfg.apiKey)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodPost, endpoint.String(), bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Google Cloud Speech-to-Text: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var response recognizeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response %q: %w", string(body), err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if response.Error != nil {
			return "", fmt.Errorf(
				"google speech api error %d %s: %s",
				response.Error.Code,
				response.Error.Status,
				response.Error.Message,
			)
		}
		return "", fmt.Errorf("google speech api returned HTTP %s", resp.Status)
	}

	if response.Error != nil {
		return "", fmt.Errorf(
			"google speech api error %d %s: %s",
			response.Error.Code,
			response.Error.Status,
			response.Error.Message,
		)
	}

	transcript := collectTranscript(response.Results)
	if transcript == "" {
		return "", errors.New("no transcription returned")
	}

	return transcript, nil
}

func collectTranscript(results []recognitionResult) string {
	transcripts := make([]string, 0, len(results))

	for _, result := range results {
		if len(result.Alternatives) == 0 {
			continue
		}
		text := strings.TrimSpace(result.Alternatives[0].Transcript)
		if text == "" {
			continue
		}
		transcripts = append(transcripts, text)
	}

	return strings.Join(transcripts, " ")
}
