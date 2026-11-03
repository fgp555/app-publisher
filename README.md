# play-tool

`play-tool` lets you inspect and publish native Android and React Native apps on Google Play. It detects the Gradle wrapper for building and uploading, and identifies a React Native project root using `package.json` and `android/`.

## Installation on Windows

Install the latest CLI version from any terminal with Go installed:

```sh
go install github.com/fgp555/play-tool@latest
```

Go installs the executable in `$(go env GOPATH)/bin` (usually `%USERPROFILE%/go/bin` on Windows). Add that directory to your `PATH` to run `play-tool` from any terminal. Open a new terminal after changing `PATH`.

To install from a local checkout instead, run this from the repository root in PowerShell:

```powershell
Set-Location E:\dev\kotlin\com.frankgp.tools\tools\play-tool
go install .
```

The local install also places the executable in `$(go env GOPATH)\bin`.

## Automatic and optional configuration

`play-tool.json` is optional. Without it, `play-tool` detects `applicationId` in `app/build.gradle.kts` or `app/build.gradle`, uses the `internal` track, and assumes the standard AAB output path `app/build/outputs/bundle/release/app-release.aab`. For React Native, this path is resolved from the `android/` directory.

The `deploy` command increments the numeric `versionCode` in either Gradle file format before building and uploading.

Set the Google Play key with the `PLAY_TOOL_KEY` environment variable or the `key` field in `play-tool.json`. The environment variable takes precedence. In PowerShell, set it for the current terminal session with:

```powershell
$env:PLAY_TOOL_KEY = "E:\dev\android\play-service-account.json"
```

To configure the key per project, create a minimal `play-tool.json` in the Gradle root or React Native root (where `package.json` is located):

```json
{
  "key": "E:/dev/android/play-service-account.json"
}
```

The JSON can override `package`, `track`, `aab`, `apkDebug`, `apkRelease`, `key`, and `gradleRoot`. Its relative paths are resolved from the directory containing the JSON; the relative path in `PLAY_TOOL_KEY` is resolved from the Gradle root. If `applicationId` is not written as a literal in `app/build.gradle.kts` or `app/build.gradle`, set `package` manually. For projects with flavors, configure the paths and package for the variant you publish. Do not commit credentials to the repository.

To inspect AAB metadata, obtain `bundletool-all.jar` and set its path in your PowerShell session:

```powershell
$env:PLAY_BUNDLETOOL_JAR = "E:\tools\bundletool-all.jar"
```

## Usage

List the available commands without configuring the project:

```powershell
play-tool --help
```

Generate a GitHub Actions workflow in `.github/workflows/deploy-go.yml`:

```powershell
play-tool initWorkflow
```

The workflow runs `play-tool deploy` on pushes to `main` and can also be started manually. Add these repository Actions secrets:

- `PLAY_SERVICE_ACCOUNT_JSON`: the Google Play service account JSON.
- `RELEASE_KEYSTORE_BASE64`: the release keystore file encoded as Base64.
- `KEYSTORE_PASSWORD`, `KEY_ALIAS`, and `KEY_PASSWORD`: the Android signing credentials.

The workflow writes the service account JSON to a temporary file, restores the release keystore to `app/release.keystore`, then builds and uploads the AAB.

Show the version without requiring a `play-tool.json` in the current directory:

```powershell
play-tool --version
# play-tool 1.0.9 go1.27.0 windows/amd64
```

Run these commands from any directory inside the project, including its root, without additional configuration:

```powershell
play-tool                              # Open the interactive menu
play-tool show                         # Show published versions
play-tool assembleDebug                # Build the debug APK
play-tool assembleRelease              # Build the release APK
play-tool bundleRelease                # Build the release AAB
play-tool cleanBundleRelease           # Run clean and build the release AAB
play-tool installDebug                 # Install debug with Gradle
play-tool installRelease               # Install release with Gradle
play-tool adbInstallDebug              # Install the existing debug APK with adb
play-tool adbInstallRelease            # Install the existing release APK with adb
play-tool info assembleDebug           # Inspect the existing debug APK
play-tool info assembleRelease         # Inspect the existing release APK
play-tool info bundleRelease           # Inspect the existing release AAB
play-tool verify apk debug             # Verify the debug APK certificate
play-tool verify apk release           # Verify the release APK certificate
play-tool verify bundle                # Verify the AAB certificate
play-tool manifest min-sdk debug       # Show minSdk for the debug APK
play-tool manifest min-sdk release     # Show minSdk for the release APK
play-tool manifest target-sdk debug   # Show targetSdk for the debug APK
play-tool manifest target-sdk release  # Show targetSdk for the release APK
play-tool upload                       # Upload the configured AAB to Google Play
play-tool deploy                       # Update versionCode, build, and upload
play-tool initWorkflow                # Generate the GitHub Actions deploy workflow
play-tool --version                    # Show the play-tool, Go, and platform versions
play-tool --help                       # Show help
```

`cleanBundleRelease` runs `clean bundleRelease`. The `assembleDebug`, `assembleRelease`, and `bundleRelease` commands run the corresponding Gradle task. `installDebug` and `installRelease` install through Gradle; `adbInstallDebug` and `adbInstallRelease` install an existing APK with `adb install -r -d`. `verify` displays the APK/AAB certificate, and `manifest` reads minSdk or targetSdk. Tools are searched for in standard Android SDK/JDK locations and then on `PATH`.

`info` inspects an existing artifact and displays its package, versions, minSdk/targetSdk, signature, SHA-1, SHA-256, path, and size. AAB inspection requires `PLAY_BUNDLETOOL_JAR` and displays the bundle's upload certificate. `compileSdk` is not included in the final artifact and is not reported.

Commands display status with emojis. `upload` shows a percentage bar based on chunked AAB upload progress, then reports when it publishes and confirms the track.

When present, the program finds the nearest `play-tool.json` while traversing upward from the current directory and stops at the nearest Gradle root. You can omit it if automatic detection supports your project and `PLAY_TOOL_KEY` is set.

Running `play-tool` without a command opens the interactive menu, grouped into Build and install, Info and verification, and Google Play. Deploy and publishing actions ask for confirmation before running.
