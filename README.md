# vflat-share-cli

Downloads files sent from the **vFlat-Scan** mobile app via its *share-with-pc* feature to your local disk, with optional image transcription through Gemini. Built by reverse-engineering the `send.vflat.com` protocol.

Originally a python script, but 4-6 MB is smaller than 40 MB.

## How it works

1. Creates a session (UUID) via the vFlat API (`https://send.vflat.com/api/create`).
2. Displays a QR code in the terminal — scan it in the vFlat app on your phone
   and send your photos/scans.
3. After you press Enter, it fetches the file list from the local vFlat server
   (`http://BASE_ADDRESS:PORT/<uuid>/info`) and downloads them concurrently.
4. Optionally transcribes each image into a `.txt` file with the same name.

## Build

```bash
go build -o vflat-fs .            # current platform
go build -ldflags="-s -w" -o vflat-fs .   # smaller (stripped) binary
```

Cross-compile (no toolchain needed on the target):

```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o vflat-go.exe .
GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o vflat-go-macos .
```

## Configuration

Values can come from environment variables (a `.env` file next to the binary
or in the working directory) or command-line flags. Flags win over
environment variables, which win over `.env`.

`.env`:

```dotenv
PORT=8818
BASE_ADDRESS=100.104.231.68
GEMINI_API_KEY=your_api_key
```

| Setting        | Environment variable | Flag             | Default            |
|----------------|----------------------|------------------|--------------------|
| Server port    | `PORT`               | `--port`         | `8818`             |
| Server address | `BASE_ADDRESS`       | `--base-address` | —                  |
| Gemini key     | `GEMINI_API_KEY`     | `--api-key`      | —                  |

## Usage

```bash
./vflat                                   # pick a directory via GUI dialog
./vflat -o ~/scans                        # download to a specific directory
./vflat -o ~/scans --transcribe           # download + transcribe
./vflat --base-address 192.168.1.50 --port 8818 --transcribe -o ~/scans
```

### All flags

```
--port            vFlat server port (env: PORT)
--base-address    vFlat server IP/host (env: BASE_ADDRESS)
--api-key         Gemini API key (env: GEMINI_API_KEY)
-o, --output      Destination directory (omitted -> GUI picker dialog)
--transcribe      Transcribe images into .txt files
--model           Gemini model (default: gemini-3.5-flash)
```

## Directory picker

The picker shells out to the platform's built-in dialog — no GUI dependencies,
so the binary stays small and pure-Go (cross-compiles cleanly):

- **Linux** — `zenity`
- **Windows** — PowerShell `FolderBrowserDialog`
- **macOS** — `osascript` (`choose folder`)

If the dialog tool isn't available, it falls back to typing the path on stdin.
You can always skip the picker with `-o/--output`.

## Files

- `main.go` — config, flags, `.env` loading, pipeline orchestration.
- `vflat.go` — vFlat API calls and concurrent downloads.
- `transcribe.go` — image transcription via the Gemini REST API.
- `picker.go` — cross-platform directory picker.
