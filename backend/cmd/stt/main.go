// stt is a command-line tool for testing the Voxtral speech APIs.
//
// Usage:
//
//	MISTRAL_API_KEY=<key> go run ./cmd/stt transcribe <file.wav>
//	MISTRAL_API_KEY=<key> go run ./cmd/stt speak [-voice <id>] <text> > out.mp3
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

	"github.com/svdx9/talki/backend/internal/tts"
	"github.com/svdx9/talki/backend/internal/tts/voxtral"
)

var (
	errNotRIFF          = errors.New("not a RIFF file")
	errNotWAVE          = errors.New("not a WAVE file")
	errFmtChunkTooSmall = errors.New("fmt chunk too small")
	errOnlyPCMSupported = errors.New("only PCM (format 1) supported")
	errDataBeforeFmt    = errors.New("data chunk before fmt chunk")
)

const (
	transcribeModel = "voxtral-mini-transcribe-realtime-2602"
	chunkSize       = 4096 // bytes per send (~128ms at 16kHz mono 16-bit)
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  stt transcribe <file.wav>")
	fmt.Fprintln(os.Stderr, "  stt speak [-voice <id>] <text>")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "MISTRAL_API_KEY is not set")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "transcribe":
		runTranscribe(apiKey, os.Args[2:])
	case "speak":
		runSpeak(apiKey, os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func runTranscribe(apiKey string, args []string) {
	fs := flag.NewFlagSet("transcribe", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "usage: stt transcribe <file.wav>") }
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	sampleRate, pcm, err := readWAV(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read wav: %v\n", err)
		os.Exit(1)
	}
	slog.Info("wav loaded", "sample_rate", sampleRate, "pcm_bytes", len(pcm))

	ctx := context.Background()
	af := tts.AudioFormat{Encoding: "pcm_s16le", SampleRate: sampleRate}
	client, err := voxtral.Dial(ctx, apiKey, transcribeModel, af, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = client.Close() }()

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

func runSpeak(apiKey string, args []string) {
	fs := flag.NewFlagSet("speak", flag.ExitOnError)
	voiceID := fs.String("voice", "fr_marie_neutral", "Mistral voice ID")
	refAudioPath := fs.String("ref-audio", "", "path to audio file for zero-shot voice cloning")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: stt speak [-voice <id>|-ref-audio <file>] <text>")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	var voice tts.SpeechVoice
	if *voiceID != "" {
		//exhaustruct:ignore
		voice = tts.SpeechVoice{VoiceID: *voiceID}
	} else {
		raw, err := os.ReadFile(*refAudioPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "speak: read ref-audio: %v\n", err)
			os.Exit(1)
		}
		//exhaustruct:ignore
		voice = tts.SpeechVoice{RefAudio: raw}
	}

	ctx := context.Background()
	client := voxtral.NewAudioClient(apiKey, nil)
	err := client.Speech(ctx, voice, fs.Arg(0), os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "speak: %v\n", err)
		os.Exit(1)
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
