package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/manifoldco/promptui"
)

// version can be overridden at build time with -ldflags.
var version = "26.11.1"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("app-publisher %s %s %s/%s\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "init" {
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
			"🔨 Build and install",
			"🔍 Info and verification",
			"☁️  Google Play",
			"🚪 Exit",
		})
		if err != nil {
			return
		}

		switch idx {
		case 0:
			buildInstallMenu(cfg)
		case 1:
			infoMenu(cfg)
		case 2:
			googlePlayMenu(cfg)
		case 3:
			return
		}
	}
}

func buildInstallMenu(cfg Config) {
	items := []string{
		"app-publisher assembleDebug",
		"app-publisher installDebug",
		"app-publisher adbInstallDebug",
		"app-publisher assembleRelease",
		"app-publisher installRelease",
		"app-publisher adbInstallRelease",
		"app-publisher bundleRelease",
		"app-publisher cleanBundleRelease",
		"Back",
	}
	for {
		idx, err := selectOption("Build and install", items)
		if err != nil || idx == len(items)-1 {
			return
		}
		commands := [][]string{
			{"assembleDebug"},
			{"installDebug"},
			{"adbInstallDebug"},
			{"assembleRelease"},
			{"installRelease"},
			{"adbInstallRelease"},
			{"bundleRelease"},
			{"cleanBundleRelease"},
		}
		report(runCI(cfg, commands[idx]))
	}
}

func infoMenu(cfg Config) {
	items := []string{
		"app-publisher info assembleDebug",
		"app-publisher info assembleRelease",
		"app-publisher info bundleRelease",
		"app-publisher verify apk debug",
		"app-publisher verify apk release",
		"app-publisher verify bundle",
		"app-publisher manifest min-sdk debug",
		"app-publisher manifest target-sdk debug",
		"app-publisher manifest min-sdk release",
		"app-publisher manifest target-sdk release",
		"Back",
	}
	commands := [][]string{
		{"info", "assembleDebug"},
		{"info", "assembleRelease"},
		{"info", "bundleRelease"},
		{"verify", "apk", "debug"},
		{"verify", "apk", "release"},
		{"verify", "bundle"},
		{"manifest", "min-sdk", "debug"},
		{"manifest", "target-sdk", "debug"},
		{"manifest", "min-sdk", "release"},
		{"manifest", "target-sdk", "release"},
	}
	for {
		idx, err := selectOption("Info and verification", items)
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
		}
	}
}

func printHelp() {
	fmt.Printf(`app-publisher %s

Usage:
  app-publisher
  app-publisher init
  app-publisher init kt
  app-publisher init rn
  app-publisher show
  app-publisher assembleDebug
  app-publisher assembleRelease
  app-publisher bundleRelease
  app-publisher cleanBundleRelease
  app-publisher installDebug
  app-publisher installRelease
  app-publisher adbInstallDebug
  app-publisher adbInstallRelease
  app-publisher info assembleDebug
  app-publisher info assembleRelease
  app-publisher info bundleRelease
  app-publisher verify apk debug
  app-publisher verify apk release
  app-publisher verify bundle
  app-publisher manifest min-sdk debug
  app-publisher manifest min-sdk release
  app-publisher manifest target-sdk debug
  app-publisher manifest target-sdk release
  app-publisher upload
  app-publisher deploy
  app-publisher --version
  app-publisher --help

Configuration:
  app-publisher.json is optional. Native Android and React Native projects are detected.
  package is detected from applicationId in build.gradle.kts or build.gradle.
  For React Native, the wrapper runs from android/, and the JSON can be placed
  in the directory containing package.json.
  aab uses the standard release path, and APP_PUBLISHER_KEY points to the Google Play key.
  APKs use the standard paths under app/build/outputs/apk; override them with
  apkDebug and apkRelease. Android tools are searched in the SDK or PATH;
  keytool is searched in JAVA_HOME or PATH.
  info bundleRelease also requires APP_BUNDLETOOL_JAR to point to bundletool-all.jar.
  The JSON can override these values; relative paths are resolved from its directory.

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
