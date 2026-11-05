package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const appPublisherModule = "github.com/fgp555/app-publisher"

func updateCLI() error {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("Go was not found on PATH; install Go, then run app-publisher update")
	}
	if runtime.GOOS == "windows" {
		return updateCLIWindows(goPath)
	}

	fmt.Printf("⬇️  Updating app-publisher from %s@latest...\n", appPublisherModule)
	cmd := exec.Command(goPath, "install", appPublisherModule+"@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not update app-publisher with go install: %w", err)
	}
	fmt.Println("✅ app-publisher updated to the latest version")
	return nil
}
