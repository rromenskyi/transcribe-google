# transcribe-google

Minimal Go CLI for transcribing short audio files through Google Cloud
Speech-to-Text.

It converts input audio with `ffmpeg`, sends a synchronous recognition request
to the Google Speech-to-Text REST API, and prints the final transcript.

## What Is Google Cloud Speech-to-Text

Google Cloud Speech-to-Text is Google's managed speech recognition service. It
accepts audio as input and returns text transcripts using Google's speech
models.

This repository is not a full SDK wrapper. It is a very small CLI utility for a
single job: send one short audio file to the REST API and print the result.

Official docs:

- Cloud Speech-to-Text basics: <https://docs.cloud.google.com/speech-to-text/docs/speech-to-text-requests>
- Authentication: <https://docs.cloud.google.com/speech-to-text/docs/authentication>
- REST reference: <https://docs.cloud.google.com/speech-to-text/docs/reference/rest>

## Why This Exists

This tool is useful when you already have recorded audio, for example from
Asterisk call recordings, and need a lightweight command-line helper to:

- normalize audio with `ffmpeg`
- send it to Google Speech-to-Text
- print back the recognized text

Google does the speech recognition itself. This repository is just the glue
around that API.

## What This CLI Does

1. Reads a source audio file supported by `ffmpeg`
2. Re-encodes it to mono 16-bit PCM
3. Base64-encodes the audio payload
4. Calls `speech:recognize`
5. Prints the merged transcript

This repo targets synchronous recognition for short clips. Google documents
that synchronous `recognize` is limited to audio of 1 minute or less; longer
audio should use asynchronous recognition instead.

## Requirements

- Go 1.22+
- `ffmpeg` in `PATH`
- A Google Cloud project with Cloud Speech-to-Text enabled
- A valid API key exported as `GOOGLE_API_KEY`

For a minimal REST-based CLI, this repo uses an API key. Google also documents
client libraries and Application Default Credentials for programmatic access,
which is the more typical production path.

## Quick Start

Export your API key:

```bash
export GOOGLE_API_KEY="your-api-key"
```

Build the CLI:

```bash
go build -o bin/transcribe-google ./app
```

Run it:

```bash
./bin/transcribe-google \
  -input /path/to/audio.wav \
  -language en-US \
  -sample-rate 8000
```

Example output:

```text
hello this is a test call
```

## Usage

Preferred flag-based form:

```bash
go run ./app \
  -input /path/to/audio.wav \
  -language en-US \
  -sample-rate 8000
```

Legacy positional form is also supported:

```bash
go run ./app /path/to/audio.wav en-US 8000
```

Arguments:

- `-input`: input audio file path
- `-language`: BCP-47 language code such as `en-US` or `uk-UA`
- `-sample-rate`: sample rate in Hz, often `8000` for telephony recordings
- `-api-key`: optional override for `GOOGLE_API_KEY`
- `-endpoint`: optional override for the REST endpoint

## Typical Telephony Example

```bash
GOOGLE_API_KEY="your-api-key" go run ./app \
  -input /var/spool/asterisk/monitor/call.wav \
  -language en-US \
  -sample-rate 8000
```

## Notes

- This tool is intentionally minimal and only handles synchronous REST
  transcription.
- It prints only the final merged transcript.
- Multi-minute recordings should be routed through asynchronous recognition,
  not this one-shot CLI.
- Recognition quality depends on language selection, audio quality, and whether
  the sample rate matches the source material.

## Repository Layout

```text
.
├── app/main.go
├── go.mod
├── LICENSE
└── README.md
```
