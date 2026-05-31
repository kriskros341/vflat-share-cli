// vflat-go — downloads files sent from the vFlat mobile app (share-with-pc)
// to the local disk, with optional transcription via Gemini.
//
// Pipeline:
//  1. create a session (UUID) via the vFlat API,
//  2. display a QR code to scan in the mobile app,
//  3. fetch the file list and download the files concurrently,
//  4. (optionally) transcribe the images into .txt files.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// Config holds the resolved runtime settings.
type Config struct {
	Port        int
	BaseAddress string
	APIKey      string
	Output      string
	Transcribe  bool
	Model       string
}

const defaultModel = "gemini-3.5-flash"

func parseConfig() Config {
	// Populate process env from .env files (cwd first, then next to the binary).
	// Real environment variables always win — loadEnvFile never overrides them.
	loadEnvFile(".env")
	if dir, err := os.Executable(); err == nil {
		loadEnvFile(filepath.Join(filepath.Dir(dir), ".env"))
	}

	port := flag.Int("port", atoiDefault(os.Getenv("PORT"), 8818),
		"vFlat server port (env: PORT)")
	base := flag.String("base-address", os.Getenv("BASE_ADDRESS"),
		"vFlat server IP/host (env: BASE_ADDRESS)")
	apiKey := flag.String("api-key", os.Getenv("GEMINI_API_KEY"),
		"Gemini API key for transcription (env: GEMINI_API_KEY)")
	model := flag.String("model", defaultModel, "Gemini model used for transcription")
	transcribe := flag.Bool("transcribe", false,
		"After downloading, transcribe the images into .txt files")

	var output string
	flag.StringVar(&output, "output", "", "Destination directory (omitted -> GUI picker)")
	flag.StringVar(&output, "o", "", "Destination directory (shorthand)")

	flag.Parse()

	return Config{
		Port:        *port,
		BaseAddress: *base,
		APIKey:      *apiKey,
		Output:      output,
		Transcribe:  *transcribe,
		Model:       *model,
	}
}

func run(cfg Config) error {
	if cfg.BaseAddress == "" {
		return fmt.Errorf("missing BASE_ADDRESS: set it in .env or pass --base-address")
	}
	if cfg.Transcribe && cfg.APIKey == "" {
		return fmt.Errorf("missing GEMINI_API_KEY: set it in .env or pass --api-key")
	}

	directory := cfg.Output
	if directory == "" {
		directory = askDirectory()
	}
	if directory == "" {
		return fmt.Errorf("no directory selected")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}

	uuid, err := createSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	showQR(uuid)

	files, err := fetchFileList(cfg.BaseAddress, cfg.Port, uuid)
	if err != nil {
		return fmt.Errorf("fetch file list: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no files to download")
	}

	downloadAll(cfg.BaseAddress, cfg.Port, uuid, files, directory)

	if cfg.Transcribe {
		transcribeAll(cfg.APIKey, cfg.Model, files, directory)
	}
	return nil
}

func showQR(uuid string) {
	q, err := qrcode.New("vflat:uuid:"+uuid, qrcode.Low)
	if err != nil {
		fmt.Println("Failed to render QR code:", err)
		fmt.Println("Pair manually with UUID:", uuid)
	} else {
		fmt.Println(q.ToSmallString(false))
	}
	fmt.Println("Scan the QR code in the vFlat app, send your files, then...")
	fmt.Print("Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// loadEnvFile reads a simple KEY=VALUE .env file, setting only keys that are
// not already present in the environment. Missing files are ignored.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		if _, ok := os.LookupEnv(key); !ok {
			os.Setenv(key, val)
		}
	}
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func main() {
	if err := run(parseConfig()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
