//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func updateCLIWindows(goPath string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate app-publisher executable: %w", err)
	}
	if strings.ToLower(filepath.Base(executable)) != "app-publisher.exe" {
		return fmt.Errorf("update must be run from the installed app-publisher.exe")
	}

	stagingDir, err := os.MkdirTemp("", "app-publisher-update-*")
	if err != nil {
		return fmt.Errorf("could not create a temporary update directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	fmt.Printf("⬇️  Downloading %s@latest...\n", appPublisherModule)
	cmd := exec.Command(goPath, "install", appPublisherModule+"@latest")
	cmd.Env = append(os.Environ(), "GOBIN="+stagingDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not download the latest app-publisher version: %w", err)
	}
	newExecutable := filepath.Join(stagingDir, "app-publisher.exe")
	if err := requireFile(newExecutable, "updated app-publisher executable"); err != nil {
		return err
	}

	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		return fmt.Errorf("PowerShell was not found; cannot replace the running app-publisher.exe")
	}
	alias := filepath.Join(filepath.Dir(executable), "app.exe")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	script := fmt.Sprintf("Start-Sleep -Seconds 2; Move-Item -LiteralPath %s -Destination %s -Force; if (Test-Path -LiteralPath %s) { Copy-Item -LiteralPath %s -Destination %s -Force }; Remove-Item -LiteralPath %s -Recurse -Force", quote(newExecutable), quote(executable), quote(alias), quote(executable), quote(alias), quote(stagingDir))
	updater := exec.Command(powershell, "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
	if err := updater.Start(); err != nil {
		return fmt.Errorf("could not schedule the update: %w", err)
	}
	cleanup = false
	if err := updater.Process.Release(); err != nil {
		return fmt.Errorf("could not detach the update process; the update remains staged at %s: %w", newExecutable, err)
	}
	fmt.Println("✅ Update downloaded; app-publisher and its app launcher will be replaced after this command exits.")
	return nil
}
