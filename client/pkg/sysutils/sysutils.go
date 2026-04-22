package sysutils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// OpenFile opens a file with the default application for the current OS.
func OpenFile(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// GetTempDir returns a directory for temporary files, ensuring it exists.
func GetTempDir() (string, error) {
	dir := os.TempDir()
	mandalaDir := fmt.Sprintf("%s/mandala-workspace", dir)
	if _, err := os.Stat(mandalaDir); os.IsNotExist(err) {
		err = os.MkdirAll(mandalaDir, 0755)
		if err != nil {
			return "", err
		}
	}
	return mandalaDir, nil
}
