package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// dispatchAlias forwards invocations through the ap launcher to the current
// app-publisher executable in the same installation directory.
func dispatchAlias() (int, bool) {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), filepath.Ext(os.Args[0]))
	if base != "ap" {
		return 0, false
	}

	current, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ap: could not locate its executable:", err)
		return 1, true
	}
	canonicalName := "app-publisher"
	if runtime.GOOS == "windows" {
		canonicalName += ".exe"
	}
	target := filepath.Join(filepath.Dir(current), canonicalName)
	if info, statErr := os.Stat(target); statErr != nil || !info.Mode().IsRegular() {
		target, err = exec.LookPath("app-publisher")
		if err != nil {
			fmt.Fprintln(os.Stderr, "ap: app-publisher was not found next to the launcher or on PATH")
			return 1, true
		}
	}

	cmd := exec.Command(target, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), true
		}
		fmt.Fprintln(os.Stderr, "ap: could not run app-publisher:", err)
		return 1, true
	}
	return 0, true
}

func createAlias() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate app-publisher executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("could not resolve app-publisher executable path: %w", err)
	}

	aliasName := "ap"
	if runtime.GOOS == "windows" {
		aliasName += ".exe"
	}
	aliasPath := filepath.Join(filepath.Dir(executable), aliasName)
	if _, err := os.Lstat(aliasPath); err == nil {
		return fmt.Errorf("%s already exists; it was left unchanged", aliasPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not inspect alias path %s: %w", aliasPath, err)
	}

	if runtime.GOOS == "windows" {
		if err := copyAliasLauncher(executable, aliasPath); err != nil {
			return err
		}
	} else if err := os.Symlink(filepath.Base(executable), aliasPath); err != nil {
		return fmt.Errorf("could not create alias %s: %w", aliasPath, err)
	}

	fmt.Printf("✅ Created launcher: %s\n", aliasPath)
	fmt.Println("ℹ️  The app-publisher installation directory must be on PATH to run `ap` from any terminal.")
	return nil
}

func copyAliasLauncher(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("could not open app-publisher executable %s: %w", source, err)
	}
	defer input.Close()

	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return fmt.Errorf("could not create alias launcher %s: %w", destination, err)
	}

	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		os.Remove(destination)
		return fmt.Errorf("could not write alias launcher %s: %w", destination, err)
	}
	if err := output.Close(); err != nil {
		os.Remove(destination)
		return fmt.Errorf("could not finish alias launcher %s: %w", destination, err)
	}
	return nil
}
