# app-publisher

`app-publisher` lets you inspect and publish native Android and React Native apps on Google Play. It detects the Gradle wrapper for building and uploading, and identifies a React Native project root using `package.json` and `android/`.

## Install on Windows

If Go is not installed, install it with WinGet:

```powershell
winget install GoLang.Go
go version
```

Open a new terminal after installation so `go` is available on `PATH`.

Install the latest CLI version from any terminal with Go installed:

```sh
go install github.com/fgp555/app-publisher@latest
app-publisher --version
app-publisher --help
```

Go installs the executable in `$(go env GOPATH)/bin` (usually `%USERPROFILE%/go/bin` on Windows). Add that directory to your `PATH` to run `app-publisher` from any terminal. Open a new terminal after changing `PATH`.

To build or install app-publisher from a local source checkout, see [CONTRIBUTING.md](docs/CONTRIBUTING.md).

## Automatic and optional configuration

`app-publisher.json` is optional. Without it, `app-publisher` detects `applicationId` in `app/build.gradle.kts` or `app/build.gradle`, uses the `internal` track, and assumes the standard AAB output path `app/build/outputs/bundle/release/app-release.aab`. For React Native, this path is resolved from the `android/` directory.

The `deploy` command increments the numeric `versionCode` in either Gradle file format before building and uploading.

## Create a Google Play service account

An API service account is required for `app-publisher upload` and `app-publisher deploy` to access Google Play. Create and authorize one before using these commands:

1. Open the [Google Cloud Console](https://console.cloud.google.com) and create a project, or select an existing project.
2. In **APIs & Services > Library**, enable the **Google Play Android Developer API**.
3. In **IAM & Admin > Service Accounts**, create a service account, for example `play-publisher@your-project.iam.gserviceaccount.com`.
4. Open the service account's **Keys** tab, create a JSON key, and download it. Store the file outside your repository and do not commit it.
5. In Google Play Console, open **Users and permissions** or **Setup > API access**, link the Cloud project, and grant the service account access to the app. Grant at least **Release to production** or **Release to testing tracks**, plus **View app information**.

Google Play may take 24–48 hours to activate the service account permissions.

## Generate a release keystore

The following Bash commands generate and inspect a PKCS12 keystore, then encode it as Base64 for the `RELEASE_KEYSTORE_BASE64` GitHub Actions secret. Run them in Bash, Git Bash, WSL, or Linux with `keytool` and `base64` available:

```bash
# Generate a PKCS12 keystore (.p12 / .pfx)
keytool -genkeypair -v -storetype PKCS12 \
  -keystore upload.keystore \
  -storepass "tutorial123" -alias tutorial123 -keypass "tutorial123" \
  -dname "CN=Frank GP, OU=Development, O=Frank GP Dev, L=Puno, ST=Puno, C=PE" \
  -keyalg RSA -keysize 2048 -validity 10000

# Encode it for the GitHub Actions secret
base64 upload.keystore > keystore.base64.txt

KEYSTORE_FILE=C:/path/to/upload.keystore
KEYSTORE_PASSWORD=tutorial123
KEY_ALIAS=tutorial123
KEY_PASSWORD=tutorial123
```

`tutorial123` and the identity values above are examples only. Use unique, strong passwords and your own certificate details for a real release. Keep the keystore and `keystore.base64.txt` outside Git; the Base64 file contains the keystore's secret data.

Set the Google Play key with the `APP_PUBLISHER_KEY` environment variable or the `key` field in `app-publisher.json`. The environment variable takes precedence. In PowerShell, set it for the current terminal session with:

```powershell
$env:APP_PUBLISHER_KEY = "C:\path\to\play-service-account.json"
```

`app-publisher init` generates `app-publisher.json` with the detected package (when available), the `internal` track, the standard AAB path, and an empty `key` field. Edit it to match your project. For example, a Kotlin Android project can use:

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

For React Native, the default AAB path starts with `android/app/`. Replace the example `key` path with the location of your Google Play service account JSON. The Actions workflow uses the `APP_PUBLISHER_KEY` environment variable instead.

The JSON can override `package`, `track`, `aab`, `apkDebug`, `apkRelease`, `key`, and `gradleRoot`. Its relative paths are resolved from the directory containing the JSON; the relative path in `APP_PUBLISHER_KEY` is resolved from the Gradle root. If `applicationId` is not written as a literal in `app/build.gradle.kts` or `app/build.gradle`, set `package` manually. For projects with flavors, configure the paths and package for the variant you publish. Do not commit credentials to the repository.

Google Play has four standard track types. Set `track` in `app-publisher.json` to the matching identifier:

| Track type | `track` value |
| --- | --- |
| Internal testing | `internal` |
| Closed testing | The custom track name configured in Play Console |
| Open testing | `beta` |
| Production | `production` |

You can create multiple closed testing tracks, each with its own custom name, so closed testing has no single fixed `track` value. See [Google Play track names](https://developers.google.com/android-publisher/tracks) and [testing tracks in Play Console](https://support.google.com/googleplay/android-developer/answer/9845334?hl=en-en).

To inspect AAB metadata, obtain `bundletool-all.jar` and set its path in your PowerShell session:

```powershell
$env:APP_BUNDLETOOL_JAR = "C:\path\to\bundletool-all.jar"
```

## Usage

List the available commands without configuring the project:

```powershell
app-publisher --help
```

Run from the project root to detect the project type and generate the matching setup:

```powershell
app-publisher init
```

The command detects Expo/React Native projects from `package.json` and `app.json`. Otherwise, it detects Kotlin Android projects from `app/build.gradle.kts`. If detection fails, use the explicit `kt` or `rn` profile.

To choose the Kotlin Android profile explicitly:

```powershell
app-publisher init kt
```

For Kotlin DSL projects, this adds a release `signingConfig` to `app/build.gradle.kts`. If it detects an existing release signing configuration, it leaves the Gradle file unchanged and warns you. Comment out the existing `signingConfig = signingConfigs.getByName("release")` line inside `buildTypes.release`, then run `app-publisher init kt` again to add workflow signing. Locally, the generated configuration reads signing values from the project's ignored `local.properties`; environment variables override those values. Add these lines to `local.properties` (keep any existing Android SDK settings):

```properties
KEYSTORE_FILE=C:/path/to/upload.keystore
KEYSTORE_PASSWORD=tutorial123
KEY_ALIAS=tutorial123
KEY_PASSWORD=tutorial123
```

Keep `local.properties` and the keystore out of Git. The generated GitHub Actions workflow restores `RELEASE_KEYSTORE_BASE64` to the runner's temporary directory and supplies the same signing variables through the environment during deployment.

Before `app-publisher assembleRelease`, `bundleRelease`, `install`, `installRelease`, or `deploy` builds, it checks that the generated release-signing configuration has all four values and that the keystore file exists. It reads environment variables first, then the Gradle root's `local.properties`. If values are missing, it reports their names and stops before building; `deploy` also stops before changing `versionCode`. This check applies to signing configuration generated by app-publisher; custom Gradle signing remains under the project's control. Debug commands and `clean` do not run this check.

For a React Native project with `package-lock.json`, generate the React Native workflow and release signing config plugin:

```powershell
app-publisher init rn
```

The React Native command creates `plugins/withAndroidReleaseSigning.js` and registers it in `app.json`. It works with Expo Continuous Native Generation: the workflow runs Prebuild to generate `android/` before Gradle setup. Both workflows run `app-publisher deploy` automatically on pushes to the `deploy` branch and can also be started manually from GitHub Actions. Add these repository Actions secrets:

Both commands also create `app-publisher.json` with the detected package when available, the `internal` track, and the standard AAB path for that project type. Existing `app-publisher.json` files are left unchanged.

- `PLAY_SERVICE_ACCOUNT_JSON`: the Google Play service account JSON.
- `RELEASE_KEYSTORE_BASE64`: the release keystore file encoded as Base64.
- `KEYSTORE_PASSWORD`, `KEY_ALIAS`, and `KEY_PASSWORD`: the Android signing credentials.

The workflows write the service account JSON to a temporary file and restore the release keystore before building and uploading the AAB. The React Native workflow also sets up Node.js 22, runs `npm ci`, and runs `npx expo prebuild --platform android --no-install`.

The React Native plugin configures the generated Gradle release signing to use `KEYSTORE_FILE`, `KEYSTORE_PASSWORD`, `KEY_ALIAS`, and `KEY_PASSWORD`. It only applies this signing config when `KEYSTORE_FILE` is set, so local debug builds keep their existing configuration. Existing workflow files are left unchanged. The plugin is upgraded automatically when it matches the previous generated version; customized plugin files are left unchanged. `app.json` is updated only to register the plugin when needed.

For local React Native signing, put the credentials in `.env.local` at the project root:

```dotenv
KEYSTORE_FILE=C:/path/to/upload.keystore
KEYSTORE_PASSWORD=tutorial123
KEY_ALIAS=tutorial123
KEY_PASSWORD=tutorial123
```

The config plugin copies these values into `android/local.properties` each time Expo Prebuild runs, preserving other properties such as `sdk.dir`. The generated Gradle signing config reads `android/local.properties`; environment variables take precedence. Expo's `.env*.local` ignore rule keeps `.env.local` out of Git. Run `npx expo prebuild --platform android --no-install` after changing `.env.local` so the generated Android project receives the updated values. Keep the keystore and local signing files out of Git.

Show the version without requiring an `app-publisher.json` in the current directory:

```powershell
app-publisher --version
```

Run these commands from any directory inside the project, including its root, without additional configuration:

```powershell
app-publisher                              # Open the interactive menu

# Google Play
app-publisher show                        # Show published versions
app-publisher upload                      # Upload the configured AAB to Google Play
app-publisher deploy                      # Update versionCode, build, and upload
app-publisher openStore                   # Open the app's store listing on a connected device

# Build and install
app-publisher prebuild                    # Run Expo Prebuild
app-publisher prebuildClean               # Run Expo Prebuild with --clean
app-publisher clean                        # Run the Gradle clean task
app-publisher build                       # Build debug APK, release APK, and release AAB
app-publisher install                      # Install release with Gradle
app-publisher assembleDebug                # Build the debug APK
app-publisher assembleRelease              # Build the release APK
app-publisher bundleRelease                # Build the release AAB
app-publisher installDebug                 # Install debug with Gradle
app-publisher installRelease               # Install release with Gradle

# Info
app-publisher info                        # Inspect debug APK, release APK, and release AAB
app-publisher info assembleDebug           # Inspect the existing debug APK
app-publisher info assembleRelease         # Inspect the existing release APK
app-publisher info bundleRelease           # Inspect the existing release AAB

# Project setup
app-publisher init                        # Detect the project type and initialize it
app-publisher init kt                     # Generate the Kotlin Android deploy workflow
app-publisher init rn                     # Generate the React Native deploy workflow
app-publisher createAlias                 # Create the ap launcher next to the executable
app-publisher update                      # Install the latest app-publisher version with Go
app-publisher uninstall                   # Remove the installed executable and adjacent ap launcher
app-publisher --version                    # Show the app-publisher, Go, and platform versions
app-publisher -v                           # Show the app-publisher, Go, and platform versions
app-publisher --help                       # Show help
app-publisher -h                           # Show help
app-publisher ?                            # Show help
```

`app-publisher createAlias` creates `ap.exe` on Windows or an `ap` symbolic link on macOS/Linux beside the installed executable. The launcher forwards commands to the current `app-publisher` executable. It does not modify `PATH` and leaves an existing `ap` file untouched.

For Expo projects, `prebuild` runs `npx expo prebuild`, and `prebuildClean` runs `npx expo prebuild --clean`. The clean variant deletes and regenerates the native `android/` and `ios/` directories; use it only when those directories are generated and do not contain manual native changes.

`build` runs `assembleDebug`, `assembleRelease`, and `bundleRelease` in one Gradle invocation without `clean`. It produces both APKs and the release AAB under the standard Gradle output folders.

`clean` runs the Gradle `clean` task. To clean and then build the release AAB, run `app-publisher clean` followed by `app-publisher bundleRelease`. The `assembleDebug`, `assembleRelease`, and `bundleRelease` commands run the corresponding Gradle task. `install` and `installRelease` run Gradle's `installRelease` task; `installDebug` runs `installDebug`. Install commands check for at least one authorized Android device or emulator with `adb` before starting Gradle. Tools are searched for in standard Android SDK/JDK locations and then on `PATH`.

`deploy` builds Kotlin Android projects with `clean bundleRelease`. For React Native/Expo projects, it runs `bundleRelease` without `clean` to avoid cleaning native CMake/Gradle outputs before the build.

On Android, `openStore` opens the detected app's Google Play listing on a connected device using `adb`. It reads the package from `app-publisher.json` or the project's `applicationId`.

`info` inspects the existing debug APK, release APK, and release AAB in that order. You can pass `assembleDebug`, `assembleRelease`, or `bundleRelease` to inspect only one artifact. It displays the package, versions, minSdk/targetSdk, signature, signing certificate DN, MD5, SHA-1, SHA-256, path, and size. APK information comes from `apksigner`; AAB information displays the upload certificate and requires `APP_BUNDLETOOL_JAR`. `compileSdk` is not included in the final artifact and is not reported.

Commands display status with emojis. `upload` shows a percentage bar based on chunked AAB upload progress, then reports when it publishes and confirms the track.

When present, the program finds the nearest `app-publisher.json` while traversing upward from the current directory and stops at the nearest Gradle root. You can omit it if automatic detection supports your project and `APP_PUBLISHER_KEY` is set.

`app-publisher update` runs `go install github.com/fgp555/app-publisher@latest`; Go must be installed and available on `PATH`. `app-publisher uninstall` removes the running app-publisher executable and its adjacent `ap` launcher. It does not remove Go or project files.

Running `app-publisher` without a command opens the interactive menu, grouped into Google Play, Build and install, Info, and Project setup. Project setup offers `app-publisher init`, `app-publisher init kt`, `app-publisher init rn`, `app-publisher createAlias`, `app-publisher update`, `app-publisher uninstall`, `app-publisher --version`, and `app-publisher --help`. Deploy and publishing actions ask for confirmation before running.

## License

This project is licensed under the [MIT License](LICENSE).

## Author

Developed by [Frank GP Developer](https://frankgp.com).
