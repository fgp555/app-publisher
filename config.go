package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const configName = "app-publisher.json"

var applicationIDRe = regexp.MustCompile(`(?m)^[ \t]*applicationId(?:[ \t]*=[ \t]*|[ \t]+)["']([^"']+)["']`)

type Config struct {
	Pkg        string `json:"package"`
	Track      string `json:"track"`
	AAB        string `json:"aab"`
	APKDebug   string `json:"apkDebug"`
	APKRelease string `json:"apkRelease"`
	Key        string `json:"key"`
	GradleRoot string `json:"gradleRoot"`
	ConfigPath string `json:"-"`
}

// loadConfig uses automatic values from the Gradle project and applies
// optional overrides from app-publisher.json.
func loadConfig() (Config, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	gradleRoot, projectRoot, foundProject, err := findProjectRoots(startDir)
	if err != nil {
		return Config{}, err
	}
	if !foundProject {
		return Config{}, fmt.Errorf("no Android/React Native project detected from %s; run app-publisher inside the project or its android directory", startDir)
	}

	path, err := findConfig(startDir, projectRoot)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Track:      "internal",
		GradleRoot: gradleRoot,
		ConfigPath: path,
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("could not read configuration %s: %w", path, err)
		}

		var overrides Config
		if err := json.Unmarshal(data, &overrides); err != nil {
			return Config{}, fmt.Errorf("configuration %s does not contain valid JSON: %w", path, err)
		}

		base := filepath.Dir(path)
		if overrides.GradleRoot != "" {
			cfg.GradleRoot = resolve(base, overrides.GradleRoot)
		}
		if overrides.Pkg != "" {
			cfg.Pkg = overrides.Pkg
		}
		if overrides.Track != "" {
			cfg.Track = overrides.Track
		}
		if overrides.AAB != "" {
			cfg.AAB = resolve(base, overrides.AAB)
		}
		if overrides.APKDebug != "" {
			cfg.APKDebug = resolve(base, overrides.APKDebug)
		}
		if overrides.APKRelease != "" {
			cfg.APKRelease = resolve(base, overrides.APKRelease)
		}
		if overrides.Key != "" {
			cfg.Key = resolve(base, overrides.Key)
		}
	}

	if cfg.Pkg == "" {
		cfg.Pkg, err = readApplicationID(cfg.GradleRoot)
		if err != nil {
			return Config{}, err
		}
	}
	if cfg.AAB == "" {
		cfg.AAB = filepath.Join(cfg.GradleRoot, "app", "build", "outputs", "bundle", "release", "app-release.aab")
	}
	if cfg.APKDebug == "" {
		cfg.APKDebug = filepath.Join(cfg.GradleRoot, "app", "build", "outputs", "apk", "debug", "app-debug.apk")
	}
	if cfg.APKRelease == "" {
		cfg.APKRelease = filepath.Join(cfg.GradleRoot, "app", "build", "outputs", "apk", "release", "app-release.apk")
	}

	if key := strings.TrimSpace(os.Getenv("APP_PUBLISHER_KEY")); key != "" {
		cfg.Key = resolve(cfg.GradleRoot, key)
	}

	return cfg, nil
}

func readApplicationID(gradleRoot string) (string, error) {
	path, err := findAppBuildFile(gradleRoot)
	if err != nil {
		return "", fmt.Errorf("%w; add \"package\" to %s or check the Android module structure", err, configName)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not read %s to detect applicationId: %w", path, err)
	}
	if match := applicationIDRe.FindSubmatch(data); len(match) == 2 {
		return string(match[1]), nil
	}
	return "", fmt.Errorf("no literal applicationId found in %s; add \"package\" to %s", path, configName)
}

func findAppBuildFile(gradleRoot string) (string, error) {
	paths := []string{
		filepath.Join(gradleRoot, "app", "build.gradle.kts"),
		filepath.Join(gradleRoot, "app", "build.gradle"),
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("Gradle configuration path is not a regular file: %s", path)
			}
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("could not inspect Gradle file %s: %w", path, err)
		}
	}
	return "", fmt.Errorf("neither app/build.gradle.kts nor app/build.gradle was found in %s", gradleRoot)
}

func resolve(base, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

func findConfig(startDir, projectRoot string) (string, error) {
	dir := startDir

	for {
		candidate := filepath.Join(dir, configName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("could not inspect %s: %w", candidate, err)
		}
		if dir == projectRoot {
			return "", nil
		}

		parent := filepath.Dir(dir)
		dir = parent
	}
}

func findProjectRoots(startDir string) (gradleRoot, projectRoot string, found bool, err error) {
	gradleRoot, found, err = findGradleRoot(startDir)
	if err != nil {
		return "", "", false, err
	}
	if found {
		projectRoot = gradleRoot
		if filepath.Base(gradleRoot) == "android" {
			parent := filepath.Dir(gradleRoot)
			if isRegularFile(filepath.Join(parent, "package.json")) {
				projectRoot = parent
			}
		}
		return gradleRoot, projectRoot, true, nil
	}

	// From the React Native root, the Gradle wrapper is in android/ and is not
	// found by traversing only parent directories.
	dir := startDir
	for {
		androidDir := filepath.Join(dir, "android")
		hasMarker, markerErr := hasGradleMarker(androidDir)
		if markerErr != nil {
			return "", "", false, markerErr
		}
		if hasMarker && isRegularFile(filepath.Join(dir, "package.json")) {
			return androidDir, dir, true, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false, nil
		}
		dir = parent
	}
}

func findGradleRoot(startDir string) (string, bool, error) {
	dir := startDir
	for {
		hasMarker, err := hasGradleMarker(dir)
		if err != nil {
			return "", false, err
		}
		if hasMarker {
			return dir, true, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
		dir = parent
	}
}

func hasGradleMarker(dir string) (bool, error) {
	for _, marker := range []string{"gradlew", "gradlew.bat", "settings.gradle", "settings.gradle.kts"} {
		path := filepath.Join(dir, marker)
		if _, err := os.Stat(path); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("could not inspect Gradle marker %s: %w", path, err)
		}
	}
	return false, nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
