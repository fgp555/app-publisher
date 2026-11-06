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

	"github.com/chzyer/readline"
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
	return streamAppLogcat(cfg, adb, device, pid)
}

func launchDevApp(adb, dir, device, packageName string) error {
	fmt.Printf("🚀 Launching %s...\n", packageName)
	output, err := captureToolOutput(adb, dir,
		"-s", device, "shell", "cmd", "package", "resolve-activity", "--brief",
		"-a", "android.intent.action.MAIN", "-c", "android.intent.category.LAUNCHER", packageName,
	)
	if err != nil {
		return fmt.Errorf("could not resolve the launcher activity for %s: %w", packageName, err)
	}

	activity := launcherComponent(string(output))
	if activity == "" {
		return fmt.Errorf("could not find a launcher activity for %s; package manager returned %q", packageName, strings.TrimSpace(string(output)))
	}

	if err := runTool(adb, dir, "-s", device, "shell", "am", "start", "-n", activity); err != nil {
		return fmt.Errorf("could not launch %s using its launcher activity %s: %w", packageName, activity, err)
	}
	return nil
}

func launcherComponent(output string) string {
	for _, line := range strings.Split(output, "\n") {
		candidate := strings.TrimSpace(line)
		if strings.HasPrefix(candidate, "ComponentInfo{") && strings.HasSuffix(candidate, "}") {
			candidate = strings.TrimSuffix(strings.TrimPrefix(candidate, "ComponentInfo{"), "}")
		}
		if strings.Contains(candidate, "/") && !strings.ContainsAny(candidate, " \t\r") {
			return candidate
		}
	}
	return ""
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

func streamAppLogcat(cfg Config, adb, device, pid string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	keys, restoreTerminal, err := startDevKeyReader(ctx)
	if err != nil {
		return fmt.Errorf("could not enable immediate keyboard input: %w", err)
	}
	defer func() {
		if err := restoreTerminal(); err != nil {
			fmt.Fprintf(os.Stderr, "\n⚠️  Could not restore terminal input mode: %v\n", err)
		}
	}()

	logCancel, logDone := startAppLogcat(ctx, adb, cfg.GradleRoot, device, pid)
	fmt.Printf("📋 Streaming logcat for %s (press R to restart, Ctrl+C to stop)...\n", cfg.Pkg)
	for {
		select {
		case <-ctx.Done():
			logCancel()
			<-logDone
			fmt.Println("\n⏹️  Development session stopped.")
			return nil

		case err := <-logDone:
			if ctx.Err() != nil {
				fmt.Println("\n⏹️  Development session stopped.")
				return nil
			}
			if err == nil {
				return fmt.Errorf("app logcat stopped unexpectedly")
			}
			return fmt.Errorf("could not stream app logcat: %w", err)

		case key, ok := <-keys:
			if !ok {
				if ctx.Err() != nil {
					logCancel()
					<-logDone
					fmt.Println("\n⏹️  Development session stopped.")
					return nil
				}
				return fmt.Errorf("keyboard input stopped unexpectedly")
			}
			if key == 3 {
				stop()
				continue
			}
			if key != 'r' && key != 'R' {
				continue
			}

			logCancel()
			<-logDone
			if ctx.Err() != nil {
				fmt.Println("\n⏹️  Development session stopped.")
				return nil
			}

			fmt.Println("🔄 Restarting debug app...")
			if err := runGradleTasks(cfg, "installDebug"); err != nil {
				return fmt.Errorf("could not reinstall the debug app: %w", err)
			}
			if err := runTool(adb, cfg.GradleRoot, "-s", device, "shell", "am", "force-stop", cfg.Pkg); err != nil {
				return fmt.Errorf("could not stop the running app before restarting: %w", err)
			}
			if err := launchDevApp(adb, cfg.GradleRoot, device, cfg.Pkg); err != nil {
				return err
			}
			pid, err = waitForAppPID(adb, cfg.GradleRoot, device, cfg.Pkg)
			if err != nil {
				return err
			}
			fmt.Println("✅ App restarted; press R to restart again.")
			logCancel, logDone = startAppLogcat(ctx, adb, cfg.GradleRoot, device, pid)
		}
	}
}

func startDevKeyReader(ctx context.Context) (<-chan rune, func() error, error) {
	terminalState, err := readline.MakeRaw(readline.GetStdin())
	if err != nil {
		return nil, nil, err
	}

	keys := make(chan rune, 1)
	go func() {
		defer close(keys)
		var input [1]byte
		for {
			n, err := os.Stdin.Read(input[:])
			if err != nil {
				return
			}
			if n == 0 {
				continue
			}
			key := rune(input[0])
			select {
			case keys <- key:
			case <-ctx.Done():
				return
			}
		}
	}()

	restoreTerminal := func() error {
		return readline.Restore(readline.GetStdin(), terminalState)
	}
	return keys, restoreTerminal, nil
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
