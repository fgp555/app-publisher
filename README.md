# play-tool

`play-tool` lets you inspect and publish native Android and React Native apps on Google Play. It detects the Gradle wrapper for building and uploading, and identifies a React Native project root using `package.json` and `android/`.

## Installation on Windows

If Go is not installed, install it with WinGet:

```powershell
winget install GoLang.Go
```

Open a new terminal after installation so `go` is available on `PATH`.

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

`play-tool init` generates `play-tool.json` with the detected package (when available), the `internal` track, the standard AAB path, and an empty `key` field. Edit it to match your project. For example, a Kotlin Android project can use:

```json
{
  "package": "com.example.app",
  "track": "internal",
  "aab": "app/build/outputs/bundle/release/app-release.aab",
  "apkDebug": "app/build/outputs/apk/debug/app-debug.apk",
  "apkRelease": "app/build/outputs/apk/release/app-release.apk",
  "key": "C:/path/to/play-service-account.json"
}
```

For React Native, the default AAB path starts with `android/app/`. Replace the example `key` path with the location of your Google Play service account JSON. The Actions workflow uses the `PLAY_TOOL_KEY` environment variable instead.

The JSON can override `package`, `track`, `aab`, `apkDebug`, `apkRelease`, `key`, and `gradleRoot`. Its relative paths are resolved from the directory containing the JSON; the relative path in `PLAY_TOOL_KEY` is resolved from the Gradle root. If `applicationId` is not written as a literal in `app/build.gradle.kts` or `app/build.gradle`, set `package` manually. For projects with flavors, configure the paths and package for the variant you publish. Do not commit credentials to the repository.

Google Play has four standard track types. Set `track` in `play-tool.json` to the matching identifier:

| Track type | `track` value |
| --- | --- |
| Internal testing | `internal` |
| Closed testing | The custom track name configured in Play Console |
| Open testing | `beta` |
| Production | `production` |

You can create multiple closed testing tracks, each with its own custom name, so closed testing has no single fixed `track` value. See [Google Play track names](https://developers.google.com/android-publisher/tracks) and [testing tracks in Play Console](https://support.google.com/googleplay/android-developer/answer/9845334?hl=en-en).

To inspect AAB metadata, obtain `bundletool-all.jar` and set its path in your PowerShell session:

```powershell
$env:PLAY_BUNDLETOOL_JAR = "E:\tools\bundletool-all.jar"
```

## Usage

List the available commands without configuring the project:

```powershell
play-tool --help
```

Run from the project root to detect the project type and generate the matching setup:

```powershell
play-tool init
```

The command detects Expo/React Native projects from `package.json` and `app.json`. Otherwise, it detects Kotlin Android projects from `app/build.gradle.kts`. If detection fails, use the explicit `kt` or `rn` profile.

To choose the Kotlin Android profile explicitly:

```powershell
play-tool init kt
```

For Kotlin DSL projects, this also adds a release `signingConfig` to `app/build.gradle.kts`. Locally, it reads signing values from the project's ignored `local.properties`; environment variables override those values. Add these lines to `local.properties` (keep any existing Android SDK settings):

```properties
KEYSTORE_FILE=E:/path/to/upload.keystore
KEYSTORE_PASSWORD=<keystore password>
KEY_ALIAS=<key alias>
KEY_PASSWORD=<key password>
```

Keep `local.properties` and the keystore out of Git. The generated GitHub Actions workflow restores `RELEASE_KEYSTORE_BASE64` to the runner's temporary directory and supplies the same signing variables through the environment during deployment.

For a React Native project with `package-lock.json`, generate the React Native workflow and release signing config plugin:

```powershell
play-tool init rn
```

The React Native command creates `plugins/withAndroidReleaseSigning.js` and registers it in `app.json`. It works with Expo Continuous Native Generation: the workflow runs Prebuild to generate `android/` before Gradle setup. Both workflows run `play-tool deploy` automatically on pushes to the `deploy` branch and can also be started manually from GitHub Actions. Add these repository Actions secrets:

Both commands also create `play-tool.json` with the detected package when available, the `internal` track, and the standard AAB path for that project type. Existing `play-tool.json` files are left unchanged.

- `PLAY_SERVICE_ACCOUNT_JSON`: the Google Play service account JSON.
- `RELEASE_KEYSTORE_BASE64`: the release keystore file encoded as Base64.
- `KEYSTORE_PASSWORD`, `KEY_ALIAS`, and `KEY_PASSWORD`: the Android signing credentials.

The workflows write the service account JSON to a temporary file and restore the release keystore before building and uploading the AAB. The React Native workflow also sets up Node.js 22, runs `npm ci`, and runs `npx expo prebuild --platform android --no-install`.

The React Native plugin configures the generated Gradle release signing to use `KEYSTORE_FILE`, `KEYSTORE_PASSWORD`, `KEY_ALIAS`, and `KEY_PASSWORD`. It only applies this signing config when `KEYSTORE_FILE` is set, so local debug builds keep their existing configuration. Existing workflow and plugin files are left unchanged; `app.json` is updated only to register the plugin when needed.

Show the version without requiring a `play-tool.json` in the current directory:

```powershell
play-tool --version
# play-tool 1.0.9 go1.27.0 windows/amd64
```

Run these commands from any directory inside the project, including its root, without additional configuration:

```powershell
play-tool                              # Open the interactive menu
play-tool init                        # Detect the project type and initialize it
play-tool init kt                     # Generate the Kotlin Android deploy workflow
play-tool init rn                     # Generate the React Native deploy workflow
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
play-tool --version                    # Show the play-tool, Go, and platform versions
play-tool --help                       # Show help
```

`cleanBundleRelease` runs `clean bundleRelease`. The `assembleDebug`, `assembleRelease`, and `bundleRelease` commands run the corresponding Gradle task. `installDebug` and `installRelease` install through Gradle; `adbInstallDebug` and `adbInstallRelease` install an existing APK with `adb install -r -d`. `verify` displays the APK/AAB certificate, and `manifest` reads minSdk or targetSdk. Tools are searched for in standard Android SDK/JDK locations and then on `PATH`.

`deploy` builds Kotlin Android projects with `clean bundleRelease`. For React Native/Expo projects, it runs `bundleRelease` without `clean` to avoid cleaning native CMake/Gradle outputs before the build.

`info` inspects an existing artifact and displays its package, versions, minSdk/targetSdk, signature, SHA-1, SHA-256, path, and size. AAB inspection requires `PLAY_BUNDLETOOL_JAR` and displays the bundle's upload certificate. `compileSdk` is not included in the final artifact and is not reported.

Commands display status with emojis. `upload` shows a percentage bar based on chunked AAB upload progress, then reports when it publishes and confirms the track.

When present, the program finds the nearest `play-tool.json` while traversing upward from the current directory and stops at the nearest Gradle root. You can omit it if automatic detection supports your project and `PLAY_TOOL_KEY` is set.

Running `play-tool` without a command opens the interactive menu, grouped into Build and install, Info and verification, and Google Play. Deploy and publishing actions ask for confirmation before running.
