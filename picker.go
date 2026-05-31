package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// askDirectory opens the platform's built-in folder picker by shelling out to
// it (zenity / PowerShell / osascript). If no GUI picker is available or the
// user cancels via an unavailable tool, it falls back to a stdin prompt.
// Returns the chosen path, or "" when cancelled.
func askDirectory() string {
	var path string
	var err error

	switch runtime.GOOS {
	case "linux":
		path, err = askZenity()
	case "windows":
		path, err = askWindows()
	case "darwin":
		path, err = askMacOS()
	default:
		err = exec.ErrNotFound
	}

	// Tool missing entirely -> fall back to typing a path.
	if errors.Is(err, exec.ErrNotFound) {
		return promptPath()
	}
	// Any other error (incl. user cancel) -> no directory.
	if err != nil {
		return ""
	}
	return strings.TrimSpace(path)
}

func askZenity() (string, error) {
	out, err := exec.Command("zenity", "--file-selection", "--directory").Output()
	return string(out), err
}

func askWindows() (string, error) {
	script := `Add-Type -AssemblyName System.Windows.Forms; ` +
		`$f = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
		`if ($f.ShowDialog() -eq 'OK') { [Console]::Out.Write($f.SelectedPath) }`
	out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script).Output()
	return string(out), err
}

func askMacOS() (string, error) {
	out, err := exec.Command("osascript", "-e", "POSIX path of (choose folder)").Output()
	return string(out), err
}

func promptPath() string {
	fmt.Print("Enter destination directory: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}
