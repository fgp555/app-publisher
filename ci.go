package main

import (
	"fmt"
	"os"
)

// runCI executes commands without the interactive menu, including in GitHub Actions.
func runCI(cfg Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command; run play-tool --help")
	}

	switch args[0] {
	case "initWorkflow":
		if len(args) != 2 {
			return fmt.Errorf("usage: play-tool initWorkflow kt|rn")
		}
		return initWorkflow(args[1])

	case "assembleDebug", "assembleRelease", "bundleRelease", "installDebug", "installRelease":
		if err := requireArgs(args, 1, args[0]); err != nil {
			return err
		}
		fmt.Printf("🛠️  Running Gradle %s...\n", args[0])
		return runGradleTasks(cfg, args[0])

	case "adbInstallDebug":
		if err := requireArgs(args, 1, args[0]); err != nil {
			return err
		}
		return adbInstall(cfg, "debug")

	case "adbInstallRelease":
		if err := requireArgs(args, 1, args[0]); err != nil {
			return err
		}
		return adbInstall(cfg, "release")

	case "cleanBundleRelease":
		if err := requireArgs(args, 1, "cleanBundleRelease"); err != nil {
			return err
		}
		return build(cfg)

	case "install":
		if len(args) != 2 {
			return fmt.Errorf("usage: play-tool installDebug|installRelease|adbInstallDebug|adbInstallRelease")
		}
		switch args[1] {
		case "installDebug", "installRelease":
			fmt.Printf("📲 Running Gradle %s...\n", args[1])
			return runGradleTasks(cfg, args[1])
		case "adbInstallDebug":
			return adbInstall(cfg, "debug")
		case "adbInstallRelease":
			return adbInstall(cfg, "release")
		default:
			return fmt.Errorf("invalid install task; use installDebug, installRelease, adbInstallDebug, or adbInstallRelease")
		}

	case "info":
		if len(args) != 2 {
			return fmt.Errorf("usage: play-tool info assembleDebug|assembleRelease|bundleRelease")
		}
		return showArtifactInfo(cfg, args[1])

	case "show":
		if err := requireArgs(args, 1, "show"); err != nil {
			return err
		}
		return showVersion(cfg)

	case "verify":
		return verifyArtifact(cfg, args[1:])

	case "manifest":
		return inspectManifest(cfg, args[1:])

	case "upload":
		if err := requireArgs(args, 1, "upload"); err != nil {
			return err
		}
		return uploadAAB(cfg)

	case "deploy":
		if err := requireArgs(args, 1, "deploy"); err != nil {
			return err
		}
		n, err := bumpVersionCode(cfg)
		if err != nil {
			return fmt.Errorf("bump: %w", err)
		}
		fmt.Printf("🔢 Assigned versionCode: %d\n", n)

		if err := build(cfg); err != nil {
			return fmt.Errorf("build: %w", err)
		}
		return uploadAAB(cfg)

	default:
		return fmt.Errorf("unknown command %q; run play-tool --help", args[0])
	}
}

func requireArgs(args []string, count int, command string) error {
	if len(args) != count {
		return fmt.Errorf("incorrect usage of %s; run play-tool --help", command)
	}
	return nil
}
func adbInstall(cfg Config, variant string) error {
	apk, err := apkPath(cfg, variant)
	if err != nil {
		return err
	}
	if err := requireFile(apk, "APK"); err != nil {
		return err
	}
	adb, err := resolveAndroidTool("adb")
	if err != nil {
		return err
	}
	fmt.Printf("📲 Installing %s APK on the connected device...\n", variant)
	return runTool(adb, cfg.GradleRoot, "install", "-r", "-d", apk)
}

func verifyArtifact(cfg Config, args []string) error {
	if len(args) == 1 && args[0] == "bundle" {
		if err := requireFile(cfg.AAB, "AAB"); err != nil {
			return err
		}
		keytool, err := resolveKeytool()
		if err != nil {
			return err
		}
		fmt.Println("🔏 Verifying AAB certificate...")
		return runTool(keytool, cfg.GradleRoot, "-printcert", "-jarfile", cfg.AAB)
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: play-tool verify apk debug|release or play-tool verify bundle")
	}
	switch args[0] {
	case "apk":
		apk, err := apkPath(cfg, args[1])
		if err != nil {
			return err
		}
		if err := requireFile(apk, "APK"); err != nil {
			return err
		}
		apksigner, err := resolveAndroidTool("apksigner")
		if err != nil {
			return err
		}
		fmt.Printf("🔏 Verifying %s APK certificate...\n", args[1])
		return runTool(apksigner, cfg.GradleRoot, "verify", "--print-certs", apk)

	default:
		return fmt.Errorf("invalid artifact; use apk debug|release or bundle")
	}
}

func inspectManifest(cfg Config, args []string) error {
	if len(args) != 2 || (args[0] != "min-sdk" && args[0] != "target-sdk") {
		return fmt.Errorf("usage: play-tool manifest min-sdk|target-sdk debug|release")
	}
	apk, err := apkPath(cfg, args[1])
	if err != nil {
		return err
	}
	if err := requireFile(apk, "APK"); err != nil {
		return err
	}
	apkanalyzer, err := resolveAndroidTool("apkanalyzer")
	if err != nil {
		return err
	}
	fmt.Printf("📄 Reading APK %s...\n", args[0])
	return runTool(apkanalyzer, cfg.GradleRoot, "manifest", args[0], apk)
}

func apkPath(cfg Config, variant string) (string, error) {
	switch variant {
	case "debug":
		return cfg.APKDebug, nil
	case "release":
		return cfg.APKRelease, nil
	default:
		return "", fmt.Errorf("invalid variant %q; use debug or release", variant)
	}
}

func requireFile(path, description string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s not found at %s: %w", description, path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s path is not a file: %s", description, path)
	}
	return nil
}
