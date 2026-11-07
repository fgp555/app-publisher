//go:build !windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
)

func uninstallCLI() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate app-publisher executable: %w", err)
	}
	if filepath.Base(executable) != "app-publisher" {
		return fmt.Errorf("uninstall must be run from the installed app-publisher executable")
	}
	if err := os.Remove(executable); err != nil {
		return fmt.Errorf("could not remove app-publisher executable %s: %w", executable, err)
	}

	alias := filepath.Join(filepath.Dir(executable), "app")
	if err := os.Remove(alias); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("app-publisher was removed, but could not remove alias %s: %w", alias, err)
	}
	fmt.Printf("🗑️  Removed app-publisher and its adjacent app launcher from %s.\n", filepath.Dir(executable))
	return nil
}
