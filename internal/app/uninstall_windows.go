//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func uninstallCLI() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate app-publisher executable: %w", err)
	}
	if strings.ToLower(filepath.Base(executable)) != "app-publisher.exe" {
		return fmt.Errorf("uninstall must be run from the installed app-publisher.exe")
	}

	alias := filepath.Join(filepath.Dir(executable), "app.exe")
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		return fmt.Errorf("PowerShell was not found; cannot schedule removal of the running executable")
	}

	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	script := fmt.Sprintf("Start-Sleep -Seconds 2; Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue; Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue", quote(executable), quote(alias))
	cmd := exec.Command(powershell, "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not schedule app-publisher removal: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("could not detach app-publisher removal process: %w", err)
	}

	fmt.Printf("🗑️  Scheduled removal of %s and its adjacent app.exe launcher.\n", executable)
	return nil
}
