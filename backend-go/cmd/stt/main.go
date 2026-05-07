// stt is a command-line tool for testing the Voxtral STT API.
// It reads a 16-bit PCM WAV file and streams it through the realtime
// transcription endpoint, printing events to stdout.
//
// Usage:
//
//	MISTRAL_API_KEY=<key> go run ./cmd/stt <file.wav>
package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/svdx9/talki/backend-go/internal/tts"
	"github.com/svdx9/talki/backend-go/internal/tts/voxtral"
)

var (
	errNotRIFF        = errors.New("not a RIFF file")
	errNotWAVE        = errors.New("not a WAVE file")
	errFmtChunkTooSmall = errors.New("fmt chunk too small")
	errOnlyPCMSupported = errors.New("only PCM (format 1) supported")
	errDataBeforeFmt   = errors.New("data chunk before fmt chunk")
)

const (
	model     = "voxtral-mini-transcribe-realtime-2602"
	chunkSize = 4096 // bytes per send (~128ms at 16kHz mono 16-bit)
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: stt <file.wav>")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "MISTRAL_API_KEY is not set")
		os.Exit(1)
	}

	sampleRate, pcm, err := readWAV(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read wav: %v\n", err)
		os.Exit(1)
	}
	slog.Info("wav loaded", "sample_rate", sampleRate, "pcm_bytes", len(pcm))

	ctx := context.Background()
	af := tts.AudioFormat{Encoding: "pcm_s16le", SampleRate: sampleRate}
	client, err := voxtral.Dial(ctx, apiKey, model, af, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = client.Close() }()

	// Send PCM in chunks, then flush.
	for i := 0; i < len(pcm); i += chunkSize {
		end := min(i+chunkSize, len(pcm))
		err = client.SendAudio(ctx, pcm[i:end])
		if err != nil {
			fmt.Fprintf(os.Stderr, "send audio: %v\n", err)
			os.Exit(1)
		}
	}
	err = client.Flush(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flush: %v\n", err)
		os.Exit(1)
	}

	// Drain events until transcription.done.
	for ev := range client.Events() {
		switch ev.Type {
		case "transcription.text.delta":
			fmt.Print(ev.Text)
		case "transcription.done":
			fmt.Println()
			return
		case "error":
			fmt.Fprintf(os.Stderr, "\nSTT error: %s\n", string(ev.Raw))
			os.Exit(1)
		default:
			slog.Debug("event", "type", ev.Type)
		}
	}
}

// readWAV parses a standard 16-bit PCM WAV file and returns the sample rate
// and raw PCM bytes. Searches for the "data" chunk to handle files with
// extra metadata chunks (e.g. LIST, INFO).
func readWAV(path string) (sampleRate int, pcm []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = f.Close() }()

	// RIFF header
	var riff [4]byte
	_, err = io.ReadFull(f, riff[:])
	if err != nil {
		return 0, nil, fmt.Errorf("read RIFF: %w", err)
	}
	if string(riff[:]) != "RIFF" {
		return 0, nil, fmt.Errorf("not a RIFF file: %w", errNotRIFF)
	}
	_, err = io.ReadFull(f, make([]byte, 4)) // skip file size
	if err != nil {
		return 0, nil, fmt.Errorf("read file size: %w", err)
	}
	var wave [4]byte
	_, err = io.ReadFull(f, wave[:])
	if err != nil {
		return 0, nil, fmt.Errorf("read WAVE: %w", err)
	}
	if string(wave[:]) != "WAVE" {
		return 0, nil, fmt.Errorf("not a WAVE file: %w", errNotWAVE)
	}

	// Walk chunks until we find "fmt " and "data".
	var fmtFound bool
	for {
		var id [4]byte
		_, err = io.ReadFull(f, id[:])
		if err != nil {
			return 0, nil, fmt.Errorf("read chunk id: %w", err)
		}
		var size uint32
		err = binary.Read(f, binary.LittleEndian, &size)
		if err != nil {
			return 0, nil, fmt.Errorf("read chunk size: %w", err)
		}

		switch string(id[:]) {
		case "fmt ":
			if size < 16 {
				return 0, nil, fmt.Errorf("fmt chunk too small: %w", errFmtChunkTooSmall)
			}
			var audioFmt uint16
			err = binary.Read(f, binary.LittleEndian, &audioFmt)
			if err != nil {
				return 0, nil, fmt.Errorf("read audio format: %w", err)
			}
			if audioFmt != 1 {
				return 0, nil, fmt.Errorf("only PCM (format 1) supported: %w", errOnlyPCMSupported)
			}
			var channels uint16
			err = binary.Read(f, binary.LittleEndian, &channels)
			if err != nil {
				return 0, nil, fmt.Errorf("read channels: %w", err)
			}
			var sr uint32
			err = binary.Read(f, binary.LittleEndian, &sr)
			if err != nil {
				return 0, nil, fmt.Errorf("read sample rate: %w", err)
			}
			sampleRate = int(sr)
			skip := int(size) - 8
			_, err = io.ReadFull(f, make([]byte, skip))
			if err != nil {
				return 0, nil, fmt.Errorf("skip fmt tail: %w", err)
			}
			fmtFound = true

		case "data":
			if !fmtFound {
				return 0, nil, fmt.Errorf("data chunk before fmt chunk: %w", errDataBeforeFmt)
			}
			pcm = make([]byte, size)
			_, err = io.ReadFull(f, pcm)
			if err != nil {
				return 0, nil, fmt.Errorf("read pcm data: %w", err)
			}
			return sampleRate, pcm, nil

		default:
			skip := int64(size)
			if skip%2 != 0 {
				skip++
			}
			_, err = f.Seek(skip, io.SeekCurrent)
			if err != nil {
				return 0, nil, fmt.Errorf("skip chunk %s: %w", string(id[:]), err)
			}
		}
	}
}
