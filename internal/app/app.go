package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/manifoldco/promptui"
)

// version can be overridden at build time with -ldflags.
var version = "1.0.0"

// Run starts the app-publisher CLI using the product version supplied by the root entry point.
func Run(productVersion string) {
	version = productVersion
	if exitCode, handled := dispatchAlias(); handled {
		os.Exit(exitCode)
	}
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		printVersion()
		return
	}
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h" || os.Args[1] == "?") {
		printHelp()
		return
	}
	if len(os.Args) > 1 && !isKnownCommand(os.Args[1]) {
		fmt.Printf("❌ Error: unknown command %q; run app-publisher --help\n", os.Args[1])
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runCI(Config{}, os.Args[1:]); err != nil {
			fmt.Println("❌ Error:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "createAlias" || os.Args[1] == "update" || os.Args[1] == "uninstall") {
		if err := runCI(Config{}, os.Args[1:]); err != nil {
			fmt.Println("❌ Error:", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("❌ Error:", err)
		os.Exit(1)
	}

	configLabel := cfg.ConfigPath
	if configLabel == "" {
		configLabel = "automatic (no app-publisher.json)"
	}
	fmt.Printf("⚙️  Config:   %s\n📱 App:      %s\n📂 Project:  %s\n", configLabel, cfg.Pkg, cfg.GradleRoot)

	if len(os.Args) > 1 {
		if err := runCI(cfg, os.Args[1:]); err != nil {
			fmt.Println("❌ Error:", err)
			os.Exit(1)
		}
		return
	}

	runInteractive(cfg)
}

func runInteractive(cfg Config) {
	for {
		idx, err := selectOption("🧭 Main menu", []string{
			"☁️  Google Play",
			"🔨 Build and install",
			"🔍 Info",
			"⚙️  Project setup",
			"🚪 Exit",
		})
		if err != nil {
			return
		}

		switch idx {
		case 0:
			googlePlayMenu(cfg)
		case 1:
			buildInstallMenu(cfg)
		case 2:
			infoMenu(cfg)
		case 3:
			if projectSetupMenu(cfg) {
				return
			}
		case 4:
			return
		}
	}
}

func projectSetupMenu(cfg Config) bool {
	items := []string{
		"app-publisher init",
		"app-publisher init kt",
		"app-publisher init rn",
		"app-publisher createAlias",
		"app-publisher update",
		"app-publisher uninstall",
		"app-publisher --version",
		"app-publisher --help",
		"Back",
	}
	commands := [][]string{
		{"init"},
		{"init", "kt"},
		{"init", "rn"},
	}
	for {
		idx, err := selectOption("Project setup", items)
		if err != nil || idx == len(items)-1 {
			return false
		}
		switch idx {
		case 4:
			report(runCI(cfg, []string{"update"}))
			return true
		case 5:
			report(runCI(cfg, []string{"uninstall"}))
			return true
		case 6:
			printVersion()
			fmt.Println()
		case 7:
			printHelp()
			fmt.Println()
		case 3:
			report(runCI(cfg, []string{"createAlias"}))
		default:
			report(runCI(cfg, commands[idx]))
		}
	}
}

func printVersion() {
	fmt.Printf("app-publisher %s %s %s/%s\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func buildInstallMenu(cfg Config) {
	items := []string{
		"app-publisher prebuild",
		"app-publisher prebuildClean",
		"app-publisher clean",
		"app-publisher build",
		"app-publisher dev",
		"app-publisher install",
		"app-publisher assembleDebug",
		"app-publisher assembleRelease",
		"app-publisher bundleRelease",
		"app-publisher installDebug",
		"app-publisher installRelease",
		"Back",
	}
	for {
		idx, err := selectOption("Build and install", items)
		if err != nil || idx == len(items)-1 {
			return
		}
		commands := [][]string{
			{"prebuild"},
			{"prebuildClean"},
			{"clean"},
			{"build"},
			{"dev"},
			{"install"},
			{"assembleDebug"},
			{"assembleRelease"},
			{"bundleRelease"},
			{"installDebug"},
			{"installRelease"},
		}
		report(runCI(cfg, commands[idx]))
	}
}

func infoMenu(cfg Config) {
	items := []string{
		"app-publisher info",
		"app-publisher info assembleDebug",
		"app-publisher info assembleRelease",
		"app-publisher info bundleRelease",
		"Back",
	}
	commands := [][]string{
		{"info"},
		{"info", "assembleDebug"},
		{"info", "assembleRelease"},
		{"info", "bundleRelease"},
	}
	for {
		idx, err := selectOption("Info", items)
		if err != nil || idx == len(items)-1 {
			return
		}
		report(runCI(cfg, commands[idx]))
	}
}

func googlePlayMenu(cfg Config) {
	items := []string{
		"app-publisher show",
		"app-publisher upload",
		"app-publisher deploy",
		"app-publisher openStore",
		"Back",
	}
	for {
		idx, err := selectOption("Google Play", items)
		if err != nil || idx == len(items)-1 {
			return
		}

		switch idx {
		case 0:
			report(showVersion(cfg))
		case 1:
			if confirm(fmt.Sprintf("📤 Upload %s to %s?", cfg.AAB, cfg.Track)) {
				report(uploadAAB(cfg))
			}
		case 2:
			if confirm(fmt.Sprintf("🔢 Increment versionCode, build, and publish to %s?", cfg.Track)) {
				report(runCI(cfg, []string{"deploy"}))
			}
		case 3:
			report(openStore(cfg))
		}
	}
}

func printHelp() {
	fmt.Printf(`app-publisher %s

Usage:
  app-publisher

  [Google Play]
  app-publisher show
  app-publisher upload
  app-publisher deploy
  app-publisher openStore

  [Build and install]
  app-publisher prebuild
  app-publisher prebuildClean
  app-publisher clean
  app-publisher build
  app-publisher dev
  app-publisher install
  app-publisher assembleDebug
  app-publisher assembleRelease
  app-publisher bundleRelease
  app-publisher installDebug
  app-publisher installRelease

  [Info]
  app-publisher info
  app-publisher info assembleDebug
  app-publisher info assembleRelease
  app-publisher info bundleRelease

  [Project setup]
  app-publisher init
  app-publisher init kt
  app-publisher init rn
  app-publisher createAlias
  app-publisher update
  app-publisher uninstall
  app-publisher --version
  app-publisher -v
  app-publisher --help
  app-publisher -h
  app-publisher ?

Documentation:
  https://github.com/fgp555/app-publisher#readme

Author:
  Frank GP Developer: https://frankgp.com
`, version)
}

func selectOption(label string, items []string) (int, error) {
	p := promptui.Select{
		Label: label,
		Items: items,
		Size:  len(items),
	}
	idx, _, err := p.Run()
	return idx, err
}

func confirm(question string) bool {
	idx, err := selectOption(question, []string{"Yes", "No"})
	return err == nil && idx == 0
}

// build runs Gradle from the project directory defined in app-publisher.json.
func build(cfg Config) error {
	fmt.Println("🛠️  Building release AAB...")
	if filepath.Base(cfg.GradleRoot) == "android" && isRegularFile(filepath.Join(filepath.Dir(cfg.GradleRoot), "package.json")) {
		return runGradleTasks(cfg, "bundleRelease")
	}
	return runGradleTasks(cfg, "clean", "bundleRelease")
}

func runGradleTasks(cfg Config, tasks ...string) error {
	gradle := filepath.Join(cfg.GradleRoot, "gradlew")
	if runtime.GOOS == "windows" {
		gradle += ".bat"
	}

	cmd := exec.Command(gradle, tasks...)
	cmd.Dir = cfg.GradleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func report(err error) {
	if err != nil {
		fmt.Println("❌ Error:", err)
		return
	}
	fmt.Println("✅ Done.")
	fmt.Println()
}
