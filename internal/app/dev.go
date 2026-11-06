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

func runDev(cfg Config, watch bool) error {
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

	if err := launchDevApp(adb, cfg.GradleRoot, device, cfg.Pkg); err != nil {
		return err
	}

	pid, err := waitForAppPID(adb, cfg.GradleRoot, device, cfg.Pkg)
	if err != nil {
		return err
	}
	if watch {
		return watchDev(cfg, adb, device, pid)
	}
	fmt.Printf("📋 Streaming logcat for %s (press Ctrl+C to stop)...\n", cfg.Pkg)
	return streamAppLogcat(adb, cfg.GradleRoot, device, pid)
}

func launchDevApp(adb, dir, device, packageName string) error {
	fmt.Printf("🚀 Launching %s...\n", packageName)
	if err := runTool(adb, dir,
		"-s", device, "shell", "monkey", "-p", packageName,
		"-c", "android.intent.category.LAUNCHER", "1",
	); err != nil {
		return fmt.Errorf("could not launch the app; check that %s has a launcher activity: %w", packageName, err)
	}
	return nil
}

func waitForAppPID(adb, dir, device, packageName string) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		output, err := captureToolOutput(adb, dir, "-s", device, "shell", "pidof", packageName)
		if err == nil {
			if pids := strings.Fields(string(output)); len(pids) > 0 {
				return pids[0], nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("app %s launched, but its process did not appear for logcat", packageName)
}

func streamAppLogcat(adb, dir, device, pid string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cmd := appLogcatCommand(ctx, adb, dir, device, pid)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\n⏹️  Log streaming stopped.")
			return nil
		}
		return fmt.Errorf("could not stream app logcat: %w", err)
	}
	return nil
}

func watchDev(cfg Config, adb, device, pid string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	changes := watchProjectFiles(ctx, cfg.GradleRoot)
	logCancel, logDone := startAppLogcat(ctx, adb, cfg.GradleRoot, device, pid)
	fmt.Printf("👀 Watching %s for changes (press Ctrl+C to stop)...\n", cfg.GradleRoot)
	for {
		select {
		case <-ctx.Done():
			logCancel()
			<-logDone
			fmt.Println("\n⏹️  Watch mode stopped.")
			return nil

		case err := <-logDone:
			if ctx.Err() != nil {
				fmt.Println("\n⏹️  Watch mode stopped.")
				return nil
			}
			if err == nil {
				return fmt.Errorf("app logcat stopped unexpectedly")
			}
			return fmt.Errorf("could not stream app logcat: %w", err)

		case err, ok := <-changes:
			if !ok {
				if ctx.Err() != nil {
					logCancel()
					<-logDone
					fmt.Println("\n⏹️  Watch mode stopped.")
					return nil
				}
				return fmt.Errorf("project file watcher stopped unexpectedly")
			}
			if err != nil {
				logCancel()
				<-logDone
				return err
			}

			logCancel()
			<-logDone
			if ctx.Err() != nil {
				fmt.Println("\n⏹️  Watch mode stopped.")
				return nil
			}

			fmt.Println("🔄 Change detected; rebuilding and reinstalling...")
			if err := runTool(adb, cfg.GradleRoot, "-s", device, "shell", "am", "force-stop", cfg.Pkg); err != nil {
				return fmt.Errorf("could not stop the running app before reinstalling: %w", err)
			}
			if err := runGradleTasks(cfg, "installDebug"); err != nil {
				return fmt.Errorf("could not reinstall the debug app: %w", err)
			}
			if err := launchDevApp(adb, cfg.GradleRoot, device, cfg.Pkg); err != nil {
				return err
			}
			pid, err = waitForAppPID(adb, cfg.GradleRoot, device, cfg.Pkg)
			if err != nil {
				return err
			}
			fmt.Println("✅ App restarted; watching for changes...")
			logCancel, logDone = startAppLogcat(ctx, adb, cfg.GradleRoot, device, pid)
		}
	}
}

func startAppLogcat(parent context.Context, adb, dir, device, pid string) (context.CancelFunc, <-chan error) {
	ctx, cancel := context.WithCancel(parent)
	cmd := appLogcatCommand(ctx, adb, dir, device, pid)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()
	return cancel, done
}

func appLogcatCommand(ctx context.Context, adb, dir, device, pid string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, adb, "-s", device, "logcat", "--pid="+pid, "-v", "time")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
