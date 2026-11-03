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
var version = "1.0.9"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("play-tool %s %s %s/%s\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("❌ Error:", err)
		os.Exit(1)
	}

	configLabel := cfg.ConfigPath
	if configLabel == "" {
		configLabel = "automatic (no play-tool.json)"
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
		"🛠️  assembleDebug",
		"📲 installDebug (Gradle)",
		"📲 adbInstallDebug (existing APK)",
		"🛠️  assembleRelease",
		"📲 installRelease (Gradle)",
		"📲 adbInstallRelease (existing APK)",
		"📦 bundleRelease",
		"🧹 Clean AAB build (clean + bundleRelease)",
		"↩️  Back",
	}
	for {
		idx, err := selectOption("🔨 Build and install", items)
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
		"📄 info assembleDebug",
		"📄 info assembleRelease",
		"📄 info bundleRelease",
		"🔏 verify APK debug",
		"🔏 verify APK release",
		"🔏 verify bundle",
		"🤖 manifest min-sdk debug",
		"🎯 manifest target-sdk debug",
		"🤖 manifest min-sdk release",
		"🎯 manifest target-sdk release",
		"↩️  Back",
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
		idx, err := selectOption("🔍 Info and verification", items)
		if err != nil || idx == len(items)-1 {
			return
		}
		report(runCI(cfg, commands[idx]))
	}
}

func googlePlayMenu(cfg Config) {
	items := []string{
		"🔎 Show published versionCode",
		"📤 Upload existing AAB",
		"🚀 Build and upload",
		"🔢 Deploy (bump + build + upload)",
		"↩️  Back",
	}
	for {
		idx, err := selectOption("☁️  Google Play", items)
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
			if err := build(cfg); err != nil {
				report(err)
				continue
			}
			if confirm(fmt.Sprintf("🚀 Upload the newly built AAB to %s?", cfg.Track)) {
				report(uploadAAB(cfg))
			}
		case 3:
			if confirm(fmt.Sprintf("🔢 Increment versionCode, build, and publish to %s?", cfg.Track)) {
				report(runCI(cfg, []string{"deploy"}))
			}
		}
	}
}

func printHelp() {
	fmt.Printf(`play-tool %s

Usage:
  play-tool
  play-tool show
  play-tool assembleDebug
  play-tool assembleRelease
  play-tool bundleRelease
  play-tool cleanBundleRelease
  play-tool installDebug
  play-tool installRelease
  play-tool adbInstallDebug
  play-tool adbInstallRelease
  play-tool info assembleDebug
  play-tool info assembleRelease
  play-tool info bundleRelease
  play-tool verify apk debug
  play-tool verify apk release
  play-tool verify bundle
  play-tool manifest min-sdk debug
  play-tool manifest min-sdk release
  play-tool manifest target-sdk debug
  play-tool manifest target-sdk release
  play-tool upload
  play-tool deploy
  play-tool initWorkflow
  play-tool --version
  play-tool --help

Configuration:
  play-tool.json is optional. Native Android and React Native projects are detected.
  package is detected from applicationId in build.gradle.kts or build.gradle.
  For React Native, the wrapper runs from android/, and the JSON can be placed
  in the directory containing package.json.
  aab uses the standard release path, and PLAY_TOOL_KEY points to the Google Play key.
  APKs use the standard paths under app/build/outputs/apk; override them with
  apkDebug and apkRelease. Android tools are searched in the SDK or PATH;
  keytool is searched in JAVA_HOME or PATH.
  info bundleRelease also requires PLAY_BUNDLETOOL_JAR to point to bundletool-all.jar.
  The JSON can override these values; relative paths are resolved from its directory.
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

// build runs Gradle from the project directory defined in play-tool.json.
func build(cfg Config) error {
	fmt.Println("🛠️  Building release AAB...")
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
}
