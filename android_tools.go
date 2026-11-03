package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func sdkRoots() []string {
	var roots []string
	seen := make(map[string]bool)
	add := func(path string) {
		if path == "" {
			return
		}
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
		path = filepath.Clean(path)
		key := strings.ToLower(path)
		if !seen[key] {
			seen[key] = true
			roots = append(roots, path)
		}
	}

	add(os.Getenv("ANDROID_SDK_ROOT"))
	add(os.Getenv("ANDROID_HOME"))
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			add(filepath.Join(localAppData, "Android", "Sdk"))
		}
	} else if userHome, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(userHome, "Android", "Sdk"))
	}
	return roots
}

func resolveAndroidTool(name string) (string, error) {
	for _, root := range sdkRoots() {
		switch name {
		case "adb":
			if path := toolInDir(filepath.Join(root, "platform-tools"), name); path != "" {
				return path, nil
			}
		case "apksigner":
			if path := newestVersionedTool(root, filepath.Join("build-tools"), name); path != "" {
				return path, nil
			}
		case "apkanalyzer":
			latest := filepath.Join(root, "cmdline-tools", "latest", "bin")
			if path := toolInDir(latest, name); path != "" {
				return path, nil
			}
			if path := newestVersionedTool(root, filepath.Join("cmdline-tools"), name, "bin"); path != "" {
				return path, nil
			}
			if path := toolInDir(filepath.Join(root, "tools", "bin"), name); path != "" {
				return path, nil
			}
		}
	}

	if path, err := lookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found; check ANDROID_SDK_ROOT/ANDROID_HOME or add the tool to PATH", name)
}

func resolveKeytool() (string, error) {
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		if path := toolInDir(filepath.Join(javaHome, "bin"), "keytool"); path != "" {
			return path, nil
		}
	}
	if path, err := lookPath("keytool"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("keytool not found; check JAVA_HOME or add the JDK to PATH")
}

func newestVersionedTool(sdkRoot, relativeDir, tool string, suffix ...string) string {
	dir := filepath.Join(sdkRoot, relativeDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})
	for _, version := range versions {
		toolDir := filepath.Join(append([]string{dir, version}, suffix...)...)
		if path := toolInDir(toolDir, tool); path != "" {
			return path
		}
	}
	return ""
}

func compareVersions(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	count := len(left)
	if len(right) > count {
		count = len(right)
	}
	for i := 0; i < count; i++ {
		var x, y int
		if i < len(left) {
			x = leadingNumber(left[i])
		}
		if i < len(right) {
			y = leadingNumber(right[i])
		}
		if x > y {
			return 1
		}
		if x < y {
			return -1
		}
	}
	return strings.Compare(a, b)
}

func leadingNumber(value string) int {
	end := 0
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(value[:end])
	return n
}

func toolInDir(dir, name string) string {
	for _, candidate := range executableNames(name) {
		path := filepath.Join(dir, candidate)
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() {
			if absolute, absErr := filepath.Abs(path); absErr == nil {
				return absolute
			}
			return path
		}
	}
	return ""
}

func executableNames(name string) []string {
	if runtime.GOOS != "windows" {
		return []string{name}
	}
	return []string{name + ".bat", name + ".exe", name}
}

func lookPath(name string) (string, error) {
	for _, candidate := range executableNames(name) {
		if path, err := exec.LookPath(candidate); err == nil {
			if absolute, absErr := filepath.Abs(path); absErr == nil {
				return absolute, nil
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("%s is not in PATH", name)
}

func runTool(path, dir string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
