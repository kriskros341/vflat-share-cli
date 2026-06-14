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
	"encoding/json"
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
	Port         int
	BaseAddress  string
	APIKey       string
	Output       string
	Transcribe   bool
	Model        string
	Instructions string
}

// fileConfig mirrors Config for JSON config files. Pointers let us tell an
// omitted key apart from a zero value, so absent keys don't clobber other
// sources.
type fileConfig struct {
	Port         *int    `json:"port"`
	BaseAddress  *string `json:"base_address"`
	APIKey       *string `json:"api_key"`
	Output       *string `json:"output"`
	Transcribe   *bool   `json:"transcribe"`
	Model        *string `json:"model"`
	Instructions *string `json:"instructions"`
}

const defaultModel = "gemini-3.5-flash"

// userConfigSubdir names this app's directory under the OS config root.
const userConfigSubdir = "vflat"

func parseConfig() (Config, error) {
	// Populate process env from .env files (cwd first, then next to the binary).
	// Real environment variables always win — loadEnvFile never overrides them.
	loadEnvFile(".env")
	if dir, err := os.Executable(); err == nil {
		loadEnvFile(filepath.Join(filepath.Dir(dir), ".env"))
	}

	// Flags carry only built-in defaults; env, config files, and the per-user
	// config are layered in explicitly below so each source keeps its own rung
	// in the precedence ladder.
	port := flag.Int("port", 8818, "vFlat server port (env: PORT)")
	base := flag.String("base-address", "", "vFlat server IP/host (env: BASE_ADDRESS)")
	apiKey := flag.String("api-key", "", "Gemini API key for transcription (env: GEMINI_API_KEY)")
	model := flag.String("model", defaultModel, "Gemini model used for transcription")
	transcribe := flag.Bool("transcribe", false,
		"After downloading, transcribe the images into .txt files")
	instructions := flag.String("instructions", "",
		"Extra instructions appended to the Gemini transcription prompt")
	configPath := flag.String("config", os.Getenv("VFLAT_CONFIG"),
		"Path to a JSON config file (env: VFLAT_CONFIG)")
	printConfig := flag.Bool("print-config", false,
		"Print the resolved configuration and exit")

	var output string
	flag.StringVar(&output, "output", "", "Destination directory (omitted -> GUI picker)")
	flag.StringVar(&output, "o", "", "Destination directory (shorthand)")

	flag.Parse()

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	// Build the config from the lowest-priority source up. Precedence, low -> high:
	//   built-in defaults < ~/.vflat.config.json < env / .env < --config file < CLI flags
	cfg := Config{Port: 8818, Model: defaultModel}

	// 1. Per-user config under the OS config dir (XDG on Linux:
	// ~/.config/vflat/config.json), the lowest file layer.
	if dir, err := os.UserConfigDir(); err == nil {
		userPath := filepath.Join(dir, userConfigSubdir, "config.json")
		if _, statErr := os.Stat(userPath); statErr == nil {
			fc, err := loadFileConfig(userPath)
			if err != nil {
				return Config{}, err
			}
			applyFileConfig(&cfg, fc)
		}
	}

	// 2. Environment / .env (only keys actually present override the user config).
	if v, ok := os.LookupEnv("PORT"); ok {
		cfg.Port = atoiDefault(v, cfg.Port)
	}
	if v, ok := os.LookupEnv("BASE_ADDRESS"); ok {
		cfg.BaseAddress = v
	}
	if v, ok := os.LookupEnv("GEMINI_API_KEY"); ok {
		cfg.APIKey = v
	}

	// 3. Explicit --config file.
	if *configPath != "" {
		fc, err := loadFileConfig(*configPath)
		if err != nil {
			return Config{}, err
		}
		applyFileConfig(&cfg, fc)
	}

	// 4. Explicit CLI flags (only those actually passed).
	if set["port"] {
		cfg.Port = *port
	}
	if set["base-address"] {
		cfg.BaseAddress = *base
	}
	if set["api-key"] {
		cfg.APIKey = *apiKey
	}
	if set["model"] {
		cfg.Model = *model
	}
	if set["transcribe"] {
		cfg.Transcribe = *transcribe
	}
	if set["instructions"] {
		cfg.Instructions = *instructions
	}
	if set["output"] || set["o"] {
		cfg.Output = output
	}

	if *printConfig {
		printResolvedConfig(cfg)
		os.Exit(0)
	}

	return cfg, nil
}

// printResolvedConfig writes the effective settings to stdout. The API key is
// masked so the output is safe to paste when debugging.
func printResolvedConfig(cfg Config) {
	apiKey := "(unset)"
	if cfg.APIKey != "" {
		apiKey = "(set)"
	}
	instructions := cfg.Instructions
	if instructions == "" {
		instructions = "(none)"
	}
	fmt.Println("Resolved configuration:")
	fmt.Printf("  port:         %d\n", cfg.Port)
	fmt.Printf("  base-address: %s\n", cfg.BaseAddress)
	fmt.Printf("  api-key:      %s\n", apiKey)
	fmt.Printf("  output:       %s\n", cfg.Output)
	fmt.Printf("  transcribe:   %t\n", cfg.Transcribe)
	fmt.Printf("  model:        %s\n", cfg.Model)
	fmt.Printf("  instructions: %s\n", instructions)
}

// applyFileConfig overlays the keys present in fc onto cfg, leaving absent
// (nil) keys untouched.
func applyFileConfig(cfg *Config, fc fileConfig) {
	if fc.Port != nil {
		cfg.Port = *fc.Port
	}
	if fc.BaseAddress != nil {
		cfg.BaseAddress = *fc.BaseAddress
	}
	if fc.APIKey != nil {
		cfg.APIKey = *fc.APIKey
	}
	if fc.Output != nil {
		cfg.Output = *fc.Output
	}
	if fc.Transcribe != nil {
		cfg.Transcribe = *fc.Transcribe
	}
	if fc.Model != nil {
		cfg.Model = *fc.Model
	}
	if fc.Instructions != nil {
		cfg.Instructions = *fc.Instructions
	}
}

// loadFileConfig reads and decodes a JSON config file.
func loadFileConfig(path string) (fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fileConfig{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return fc, nil
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
		transcribeAll(cfg.APIKey, cfg.Model, cfg.Instructions, files, directory)
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
	cfg, err := parseConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
