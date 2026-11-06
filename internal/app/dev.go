package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

func runDev(cfg Config) error {
	if filepath.Base(cfg.GradleRoot) == "android" && isRegularFile(filepath.Join(filepath.Dir(cfg.GradleRoot), "package.json")) {
		return fmt.Errorf("app-publisher dev supports native Android/Kotlin projects; use the Expo development server for React Native projects")
	}

	adb, devices, err := connectedAndroidDevices(cfg)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return noAuthorizedAndroidDeviceError()
	}
	if len(devices) > 1 {
		return fmt.Errorf("app-publisher dev requires exactly one authorized Android device or emulator; found %d. Disconnect the others and retry", len(devices))
	}
	device := devices[0]

	fmt.Printf("📱 Device: %s\n", device)
	fmt.Println("🛠️  Running Gradle installDebug...")
	if err := runGradleTasks(cfg, "installDebug"); err != nil {
		return fmt.Errorf("could not install the debug app: %w", err)
	}

	fmt.Printf("🚀 Launching %s...\n", cfg.Pkg)
	if err := runTool(adb, cfg.GradleRoot,
		"-s", device, "shell", "monkey", "-p", cfg.Pkg,
		"-c", "android.intent.category.LAUNCHER", "1",
	); err != nil {
		return fmt.Errorf("could not launch the app; check that %s has a launcher activity: %w", cfg.Pkg, err)
	}

	pid, err := waitForAppPID(adb, cfg.GradleRoot, device, cfg.Pkg)
	if err != nil {
		return err
	}
	fmt.Printf("📋 Streaming logcat for %s (press Ctrl+C to stop)...\n", cfg.Pkg)
	return streamAppLogcat(adb, cfg.GradleRoot, device, pid)
}

func waitForAppPID(adb, dir, device, packageName string) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		output, err := captureToolOutput(adb, dir, "-s", device, "shell", "pidof", "-s", packageName)
		if err == nil {
			if pid := strings.TrimSpace(string(output)); pid != "" {
				return pid, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("app %s launched, but its process did not appear for logcat", packageName)
}

func streamAppLogcat(adb, dir, device, pid string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cmd := exec.CommandContext(ctx, adb, "-s", device, "logcat", "--pid="+pid, "-v", "time")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\n⏹️  Log streaming stopped.")
			return nil
		}
		return fmt.Errorf("could not stream app logcat: %w", err)
	}
	return nil
}
