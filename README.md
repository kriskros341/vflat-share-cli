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

Values can come from command-line flags, a JSON config file (`--config`),
environment variables (a `.env` file next to the binary or in the working
directory), a per-user config file (`~/.config/vflat/config.json`), or built-in
defaults. Precedence, high to low:

**explicit CLI flag → `--config` file → environment / `.env` → per-user config → default.**

A key only overrides lower layers when it is actually present, so you can keep
stable defaults (e.g. your `model` and `instructions`) in the per-user config
and override just the bits you need per run. Use `--print-config` to print the
resolved settings (API key masked) and exit, without contacting any server:

```bash
./vflat --config ./vflat.config.json --print-config
```

`.env`:

```dotenv
PORT=8818
BASE_ADDRESS=100.104.231.68
GEMINI_API_KEY=your_api_key
```

| Setting           | Environment variable | Flag             | JSON key        | Default            |
|-------------------|----------------------|------------------|-----------------|--------------------|
| Server port       | `PORT`               | `--port`         | `port`          | `8818`             |
| Server address    | `BASE_ADDRESS`       | `--base-address` | `base_address`  | —                  |
| Gemini key        | `GEMINI_API_KEY`     | `--api-key`      | `api_key`       | —                  |
| Output directory  | —                    | `-o, --output`   | `output`        | GUI picker         |
| Transcribe        | —                    | `--transcribe`   | `transcribe`    | `false`            |
| Gemini model      | —                    | `--model`        | `model`         | `gemini-3.5-flash` |
| Extra prompt      | —                    | `--instructions` | `instructions`  | —                  |
| Config file path  | `VFLAT_CONFIG`       | `--config`       | —               | —                  |
| Print & exit      | —                    | `--print-config` | —               | `false`            |

### Custom transcription instructions

Append your own guidance to the default Gemini prompt — for example to control
output format, language handling, or how specific markings are interpreted:

```bash
./vflat -o ~/scans --transcribe \
  --instructions "Output Markdown. Render checkboxes as - [ ] / - [x]. Expand abbreviations."
```

### JSON config files

Two JSON sources are supported, both with the same shape (every key optional):

- **`~/.config/vflat/config.json`** — a per-user file loaded automatically,
  sitting below env/`.env` in precedence. Good for your stable personal
  defaults. The location follows the OS convention via Go's `os.UserConfigDir()`
  — it honors `$XDG_CONFIG_HOME` on Linux, and resolves to
  `~/Library/Application Support/vflat/config.json` on macOS and
  `%AppData%\vflat\config.json` on Windows.
- **`--config <path>`** (or the `VFLAT_CONFIG` env var) — an explicit file that
  overrides env and the per-user file.

Any flag you also pass on the command line overrides both files:

```json
{
  "port": 8818,
  "base_address": "192.168.1.50",
  "api_key": "your_api_key",
  "output": "~/scans",
  "transcribe": true,
  "model": "gemini-3.5-flash",
  "instructions": "Output Markdown. Preserve the original language. Render checkboxes as - [ ] / - [x]."
}
```

```bash
./vflat --config ./vflat.config.json
./vflat --config ./vflat.config.json --base-address 192.168.1.99   # flag overrides file
```

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
--instructions    Extra instructions appended to the Gemini prompt
--config          Path to a JSON config file (env: VFLAT_CONFIG)
--print-config    Print the resolved configuration and exit
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
