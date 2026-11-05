package app

import (
	"fmt"
	"os"
)

// runCI executes commands without the interactive menu, including in GitHub Actions.
func runCI(cfg Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command; run app-publisher --help")
	}

	switch args[0] {
	case "build":
		if err := requireArgs(args, 1, "build"); err != nil {
			return err
		}
		if err := checkGeneratedReleaseSigning(cfg); err != nil {
			return err
		}
		fmt.Println("🛠️  Building debug APK, release APK, and release AAB...")
		return runGradleTasks(cfg, "assembleDebug", "assembleRelease", "bundleRelease")

	case "prebuild", "prebuildClean":
		if err := requireArgs(args, 1, args[0]); err != nil {
			return err
		}
		return runExpoPrebuild(cfg, args[0] == "prebuildClean")

	case "createAlias":
		if err := requireArgs(args, 1, "createAlias"); err != nil {
			return err
		}
		return createAlias()

	case "update":
		if err := requireArgs(args, 1, "update"); err != nil {
			return err
		}
		return updateCLI()

	case "uninstall":
		if err := requireArgs(args, 1, "uninstall"); err != nil {
			return err
		}
		return uninstallCLI()

	case "init":
		if len(args) == 1 {
			profile, err := detectWorkflowProfile()
			if err != nil {
				return err
			}
			fmt.Printf("🔎 Detected %s project\n", profile)
			return initWorkflow(profile)
		}
		if len(args) == 2 {
			return initWorkflow(args[1])
		}
		return fmt.Errorf("usage: app-publisher init [kt|rn]")

	case "assembleDebug", "assembleRelease", "bundleRelease", "installDebug", "installRelease":
		if err := requireArgs(args, 1, args[0]); err != nil {
			return err
		}
		if requiresReleaseSigning(args[0]) {
			if err := checkGeneratedReleaseSigning(cfg); err != nil {
				return err
			}
		}
		fmt.Printf("🛠️  Running Gradle %s...\n", args[0])
		return runGradleTasks(cfg, args[0])

	case "clean":
		if err := requireArgs(args, 1, "clean"); err != nil {
			return err
		}
		fmt.Println("🧹 Running Gradle clean...")
		return runGradleTasks(cfg, "clean")

	case "install":
		if err := requireArgs(args, 1, "install"); err != nil {
			return err
		}
		if err := checkGeneratedReleaseSigning(cfg); err != nil {
			return err
		}
		fmt.Println("📲 Running Gradle installRelease...")
		return runGradleTasks(cfg, "installRelease")

	case "info":
		if len(args) > 2 {
			return fmt.Errorf("usage: app-publisher info [assembleDebug|assembleRelease|bundleRelease]")
		}
		target := ""
		if len(args) == 2 {
			target = args[1]
		}
		return showArtifactInfo(cfg, target)

	case "show":
		if err := requireArgs(args, 1, "show"); err != nil {
			return err
		}
		return showVersion(cfg)

	case "openStore":
		if err := requireArgs(args, 1, "openStore"); err != nil {
			return err
		}
		return openStore(cfg)

	case "upload":
		if err := requireArgs(args, 1, "upload"); err != nil {
			return err
		}
		return uploadAAB(cfg)

	case "deploy":
		if err := requireArgs(args, 1, "deploy"); err != nil {
			return err
		}
		if err := checkGeneratedReleaseSigning(cfg); err != nil {
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
		return fmt.Errorf("unknown command %q; run app-publisher --help", args[0])
	}
}

func requiresReleaseSigning(task string) bool {
	switch task {
	case "assembleRelease", "bundleRelease", "installRelease":
		return true
	default:
		return false
	}
}

func requireArgs(args []string, count int, command string) error {
	if len(args) != count {
		return fmt.Errorf("incorrect usage of %s; run app-publisher --help", command)
	}
	return nil
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
