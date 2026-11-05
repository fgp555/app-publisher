package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func runExpoPrebuild(cfg Config, clean bool) error {
	projectRoot, err := expoProjectRoot(cfg)
	if err != nil {
		return err
	}

	args := []string{"expo", "prebuild"}
	if clean {
		args = append(args, "--clean")
		fmt.Println("🧹 Running Expo Prebuild --clean...")
	} else {
		fmt.Println("🛠️  Running Expo Prebuild...")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/d", "/c", "npx"}, args...)
		cmd = exec.Command("cmd.exe", cmdArgs...)
	} else {
		cmd = exec.Command("npx", args...)
	}
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Expo Prebuild failed: %w", err)
	}
	return nil
}

func expoProjectRoot(cfg Config) (string, error) {
	if filepath.Base(cfg.GradleRoot) != "android" {
		return "", fmt.Errorf("prebuild commands require a React Native/Expo project; run app-publisher from the project root or android/ directory")
	}
	projectRoot := filepath.Dir(cfg.GradleRoot)
	if !isRegularFile(filepath.Join(projectRoot, "package.json")) {
		return "", fmt.Errorf("Expo project root was not found above %s", cfg.GradleRoot)
	}

	for _, name := range []string{"app.config.js", "app.config.ts", "app.config.mjs", "app.config.cjs"} {
		if isRegularFile(filepath.Join(projectRoot, name)) {
			return projectRoot, nil
		}
	}

	appJSONPath := filepath.Join(projectRoot, "app.json")
	data, err := os.ReadFile(appJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Expo config not found in %s; expected app.json or app.config.js/ts", projectRoot)
		}
		return "", fmt.Errorf("could not read Expo config %s: %w", appJSONPath, err)
	}
	var config struct {
		Expo json.RawMessage `json:"expo"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("app.json is not valid JSON: %w", err)
	}
	if len(config.Expo) == 0 || string(config.Expo) == "null" {
		return "", fmt.Errorf("app.json in %s does not contain an Expo config", projectRoot)
	}
	return projectRoot, nil
}
