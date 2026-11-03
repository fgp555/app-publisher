package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const kotlinDeployWorkflow = `name: Deploy Android app to Google Play

on:
  push:
    branches:
      - deploy
  workflow_dispatch:

permissions:
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Check out repository
        uses: actions/checkout@v4

      - name: Set up JDK 17
        uses: actions/setup-java@v4
        with:
          distribution: temurin
          java-version: '17'
          cache: gradle

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.27.0'

      - name: Install play-tool
        run: go install github.com/fgp555/play-tool@latest

      - name: Create service account JSON from secret
        env:
          PLAY_SERVICE_ACCOUNT_JSON: ${{ secrets.PLAY_SERVICE_ACCOUNT_JSON }}
        run: |
          key_file="$RUNNER_TEMP/play-service-account.json"
          printf '%s' "$PLAY_SERVICE_ACCOUNT_JSON" > "$key_file"

      - name: Decode release keystore from secret
        env:
          RELEASE_KEYSTORE_BASE64: ${{ secrets.RELEASE_KEYSTORE_BASE64 }}
        run: |
          key_file="$RUNNER_TEMP/release.keystore"
          printf '%s' "$RELEASE_KEYSTORE_BASE64" | base64 --decode > "$key_file"

      - name: Make Gradle wrapper executable
        run: chmod +x gradlew

      - name: Build and publish
        run: play-tool deploy
        env:
          PLAY_TOOL_KEY: ${{ runner.temp }}/play-service-account.json
          KEYSTORE_FILE: ${{ runner.temp }}/release.keystore
          KEYSTORE_PASSWORD: ${{ secrets.KEYSTORE_PASSWORD }}
          KEY_ALIAS: ${{ secrets.KEY_ALIAS }}
          KEY_PASSWORD: ${{ secrets.KEY_PASSWORD }}
`

const kotlinReleaseSigningMarker = "// play-tool: release signing from local.properties or environment"
const kotlinPropertiesImport = "import java.util.Properties"

const kotlinReleaseSigningConfig = `// play-tool: release signing from local.properties or environment
val playToolLocalProperties = Properties()
val playToolLocalPropertiesFile = rootProject.file("local.properties")
if (playToolLocalPropertiesFile.exists()) {
    playToolLocalPropertiesFile.inputStream().use { playToolLocalProperties.load(it) }
}
fun playToolSigningValue(name: String): String? =
    System.getenv(name)?.takeIf { it.isNotBlank() }
        ?: playToolLocalProperties.getProperty(name)?.takeIf { it.isNotBlank() }

val playToolKeystoreFile = playToolSigningValue("KEYSTORE_FILE")
if (!playToolKeystoreFile.isNullOrBlank()) {
    android {
        signingConfigs {
            create("playToolRelease") {
                storeFile = file(playToolKeystoreFile)
                storePassword = playToolSigningValue("KEYSTORE_PASSWORD")
                keyAlias = playToolSigningValue("KEY_ALIAS")
                keyPassword = playToolSigningValue("KEY_PASSWORD")
            }
        }
        buildTypes {
            getByName("release") {
                signingConfig = signingConfigs.getByName("playToolRelease")
            }
        }
    }
}
`

type generatedPlayToolConfig struct {
	Package string `json:"package,omitempty"`
	Track   string `json:"track"`
	AAB     string `json:"aab"`
	Key     string `json:"key"`
}

const reactNativeDeployWorkflow = `name: Deploy React Native app to Google Play

on:
  push:
    branches:
      - deploy
  workflow_dispatch:

permissions:
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Check out repository
        uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: npm

      - name: Install JavaScript dependencies
        run: npm ci

      - name: Generate Android native project
        run: npx expo prebuild --platform android --no-install

      - name: Set up JDK 17
        uses: actions/setup-java@v5
        with:
          distribution: temurin
          java-version: '17'
          cache: gradle
          cache-dependency-path: |
            android/*.gradle*
            android/gradle/wrapper/gradle-wrapper.properties

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.27.0'

      - name: Install play-tool
        run: go install github.com/fgp555/play-tool@latest

      - name: Create service account JSON from secret
        env:
          PLAY_SERVICE_ACCOUNT_JSON: ${{ secrets.PLAY_SERVICE_ACCOUNT_JSON }}
        run: |
          key_file="$RUNNER_TEMP/play-service-account.json"
          printf '%s' "$PLAY_SERVICE_ACCOUNT_JSON" > "$key_file"

      - name: Decode release keystore from secret
        env:
          RELEASE_KEYSTORE_BASE64: ${{ secrets.RELEASE_KEYSTORE_BASE64 }}
        run: |
          printf '%s' "$RELEASE_KEYSTORE_BASE64" | base64 --decode > android/app/release.keystore

      - name: Make Gradle wrapper executable
        run: chmod +x android/gradlew

      - name: Build and publish
        run: play-tool deploy
        env:
          PLAY_TOOL_KEY: ${{ runner.temp }}/play-service-account.json
          KEYSTORE_FILE: ${{ github.workspace }}/android/app/release.keystore
          KEYSTORE_PASSWORD: ${{ secrets.KEYSTORE_PASSWORD }}
          KEY_ALIAS: ${{ secrets.KEY_ALIAS }}
          KEY_PASSWORD: ${{ secrets.KEY_PASSWORD }}
`

const androidReleaseSigningPlugin = `const { withAppBuildGradle } = require("expo/config-plugins");

const marker = "// Configure release signing from play-tool environment variables";

module.exports = function withAndroidReleaseSigning(config) {
  return withAppBuildGradle(config, (config) => {
    const contents = config.modResults.contents;

    if (!contents.includes(marker)) {
      const signingBlock = [
        marker,
        'def playToolKeystoreFile = System.getenv("KEYSTORE_FILE")',
        'if (playToolKeystoreFile) {',
        '    android {',
        '        signingConfigs {',
        '            playToolRelease {',
        '                storeFile file(playToolKeystoreFile)',
        '                storePassword System.getenv("KEYSTORE_PASSWORD")',
        '                keyAlias System.getenv("KEY_ALIAS")',
        '                keyPassword System.getenv("KEY_PASSWORD")',
        '            }',
        '        }',
        '        buildTypes {',
        '            release {',
        '                signingConfig signingConfigs.playToolRelease',
        '            }',
        '        }',
        '    }',
        '}',
      ].join("\n");
      config.modResults.contents = contents.trimEnd() + "\n\n" + signingBlock + "\n";
    }

    return config;
  });
};
`

func initWorkflow(profile string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get current directory: %w", err)
	}

	workflowPath := filepath.Join(projectRoot, ".github", "workflows", "deploy-play.yml")
	playToolConfigPath := filepath.Join(projectRoot, configName)
	switch profile {
	case "kt":
		gradlePath := filepath.Join(projectRoot, "app", "build.gradle.kts")
		configContents, err := generatePlayToolConfig(profile, projectRoot)
		if err != nil {
			return err
		}
		updatedGradle, gradleChanged, err := addKotlinReleaseSigning(gradlePath)
		if err != nil {
			return err
		}
		if gradleChanged {
			info, err := os.Stat(gradlePath)
			if err != nil {
				return fmt.Errorf("could not inspect %s: %w", gradlePath, err)
			}
			if err := os.WriteFile(gradlePath, updatedGradle, info.Mode().Perm()); err != nil {
				return fmt.Errorf("could not update Kotlin Gradle config %s: %w", gradlePath, err)
			}
			fmt.Printf("✅ Configured release signing in %s\n", gradlePath)
		} else {
			fmt.Printf("ℹ️  Release signing is already configured in %s\n", gradlePath)
		}
		if err := writeIfMissing(playToolConfigPath, configContents); err != nil {
			return err
		}
		return writeIfMissing(workflowPath, kotlinDeployWorkflow)
	case "rn":
		appJSONPath := filepath.Join(projectRoot, "app.json")
		configContents, err := generatePlayToolConfig(profile, projectRoot)
		if err != nil {
			return err
		}
		updatedAppJSON, appJSONChanged, err := addExpoPlugin(appJSONPath, "./plugins/withAndroidReleaseSigning")
		if err != nil {
			return err
		}

		pluginPath := filepath.Join(projectRoot, "plugins", "withAndroidReleaseSigning.js")
		if err := writeIfMissing(pluginPath, androidReleaseSigningPlugin); err != nil {
			return err
		}
		if err := writeIfMissing(workflowPath, reactNativeDeployWorkflow); err != nil {
			return err
		}
		if err := writeIfMissing(playToolConfigPath, configContents); err != nil {
			return err
		}
		if appJSONChanged {
			if err := os.WriteFile(appJSONPath, updatedAppJSON, 0o644); err != nil {
				return fmt.Errorf("could not update Expo config %s: %w", appJSONPath, err)
			}
			fmt.Println("✅ Registered release signing plugin in app.json")
		} else {
			fmt.Println("ℹ️  Release signing plugin is already registered in app.json")
		}
		return nil
	default:
		return fmt.Errorf("unknown workflow profile %q; use kt or rn", profile)
	}
}

func detectWorkflowProfile() (string, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get current directory: %w", err)
	}

	packageJSONPath := filepath.Join(projectRoot, "package.json")
	appJSONPath := filepath.Join(projectRoot, "app.json")
	if fileExists(packageJSONPath) && fileExists(appJSONPath) {
		data, err := os.ReadFile(appJSONPath)
		if err != nil {
			return "", fmt.Errorf("could not read Expo config %s: %w", appJSONPath, err)
		}
		var config struct {
			Expo json.RawMessage `json:"expo"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return "", fmt.Errorf("app.json is not valid JSON: %w", err)
		}
		if len(config.Expo) > 0 {
			return "rn", nil
		}
	}

	if fileExists(filepath.Join(projectRoot, "app", "build.gradle.kts")) {
		return "kt", nil
	}
	return "", fmt.Errorf("could not detect project type in %s; run play-tool init kt or play-tool init rn", projectRoot)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func generatePlayToolConfig(profile, projectRoot string) (string, error) {
	config := generatedPlayToolConfig{Track: "internal", Key: ""}
	switch profile {
	case "kt":
		config.AAB = filepath.ToSlash(filepath.Join("app", "build", "outputs", "bundle", "release", "app-release.aab"))
		if packageName, err := readApplicationID(projectRoot); err == nil {
			config.Package = packageName
		}
	case "rn":
		config.AAB = filepath.ToSlash(filepath.Join("android", "app", "build", "outputs", "bundle", "release", "app-release.aab"))
		appJSONPath := filepath.Join(projectRoot, "app.json")
		data, err := os.ReadFile(appJSONPath)
		if err != nil {
			return "", fmt.Errorf("could not read Expo config %s: %w", appJSONPath, err)
		}
		var expoConfig struct {
			Expo struct {
				Android struct {
					Package string `json:"package"`
				} `json:"android"`
			} `json:"expo"`
		}
		if err := json.Unmarshal(data, &expoConfig); err != nil {
			return "", fmt.Errorf("Expo config %s is not valid JSON: %w", appJSONPath, err)
		}
		config.Package = expoConfig.Expo.Android.Package
		if config.Package == "" {
			if packageName, err := readApplicationID(filepath.Join(projectRoot, "android")); err == nil {
				config.Package = packageName
			}
		}
	default:
		return "", fmt.Errorf("unknown workflow profile %q; use kt or rn", profile)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not generate %s: %w", configName, err)
	}
	return string(data) + "\n", nil
}

func addKotlinReleaseSigning(gradlePath string) ([]byte, bool, error) {
	data, err := os.ReadFile(gradlePath)
	if err != nil {
		return nil, false, fmt.Errorf("could not read Kotlin Gradle config %s: %w", gradlePath, err)
	}
	if !bytes.Contains(data, []byte("plugins {")) || !bytes.Contains(data, []byte("android {")) {
		return nil, false, fmt.Errorf("%s does not look like an Android Kotlin DSL build.gradle.kts file", gradlePath)
	}
	updated, importAdded := ensureKotlinPropertiesImport(data)
	if bytes.Contains(data, []byte(kotlinReleaseSigningMarker)) {
		fixed := bytes.ReplaceAll(updated, []byte("java.util.Properties()"), []byte("Properties()"))
		return fixed, importAdded || !bytes.Equal(fixed, data), nil
	}
	updated = append([]byte(nil), updated...)
	newline := []byte("\n")
	if bytes.Contains(data, []byte("\r\n")) {
		newline = []byte("\r\n")
	}
	if len(updated) > 0 && updated[len(updated)-1] != '\n' {
		updated = append(updated, newline...)
	}
	updated = append(updated, newline...)
	config := bytes.ReplaceAll([]byte(kotlinReleaseSigningConfig), []byte("\n"), newline)
	updated = append(updated, config...)
	return updated, true, nil
}

func ensureKotlinPropertiesImport(data []byte) ([]byte, bool) {
	if bytes.Contains(data, []byte(kotlinPropertiesImport)) {
		return data, false
	}
	newline := []byte("\n")
	if bytes.Contains(data, []byte("\r\n")) {
		newline = []byte("\r\n")
	}
	updated := make([]byte, 0, len(data)+len(kotlinPropertiesImport)+len(newline))
	updated = append(updated, kotlinPropertiesImport...)
	updated = append(updated, newline...)
	updated = append(updated, data...)
	return updated, true
}

func writeIfMissing(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create directory for %s: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			fmt.Printf("ℹ️  %s already exists; leaving it unchanged\n", path)
			return nil
		}
		return fmt.Errorf("could not create %s: %w", path, err)
	}

	if _, err := file.WriteString(contents); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("could not close %s: %w", path, err)
	}

	fmt.Printf("✅ Created %s\n", path)
	return nil
}

func addExpoPlugin(appJSONPath, pluginName string) ([]byte, bool, error) {
	data, err := os.ReadFile(appJSONPath)
	if err != nil {
		return nil, false, fmt.Errorf("could not read Expo config %s: %w", appJSONPath, err)
	}
	if !json.Valid(data) {
		return nil, false, fmt.Errorf("Expo config %s is not valid JSON", appJSONPath)
	}

	expoStart, expoEnd, found, err := findJSONMember(data, "expo")
	if err != nil {
		return nil, false, fmt.Errorf("could not parse Expo config %s: %w", appJSONPath, err)
	}
	if !found {
		return nil, false, fmt.Errorf("Expo config %s has no top-level expo object", appJSONPath)
	}
	expoJSON := data[expoStart:expoEnd]
	if len(expoJSON) == 0 || expoJSON[0] != '{' {
		return nil, false, fmt.Errorf("Expo config %s has an invalid expo object", appJSONPath)
	}

	pluginsStart, pluginsEnd, hasPlugins, err := findJSONMember(expoJSON, "plugins")
	if err != nil {
		return nil, false, fmt.Errorf("could not parse plugins in %s: %w", appJSONPath, err)
	}
	encodedPlugin, err := json.Marshal(pluginName)
	if err != nil {
		return nil, false, err
	}

	var updatedExpo []byte
	if hasPlugins {
		pluginsJSON := expoJSON[pluginsStart:pluginsEnd]
		var plugins []json.RawMessage
		if err := json.Unmarshal(pluginsJSON, &plugins); err != nil {
			return nil, false, fmt.Errorf("Expo config %s has an invalid plugins array: %w", appJSONPath, err)
		}
		for _, rawPlugin := range plugins {
			var name string
			if json.Unmarshal(rawPlugin, &name) == nil && name == pluginName {
				return data, false, nil
			}
			var pluginWithOptions []json.RawMessage
			if json.Unmarshal(rawPlugin, &pluginWithOptions) == nil && len(pluginWithOptions) > 0 &&
				json.Unmarshal(pluginWithOptions[0], &name) == nil && name == pluginName {
				return data, false, nil
			}
		}

		updatedPlugins, err := appendJSONArray(pluginsJSON, encodedPlugin)
		if err != nil {
			return nil, false, fmt.Errorf("could not add release signing plugin to %s: %w", appJSONPath, err)
		}
		updatedExpo = spliceJSON(expoJSON, pluginsStart, pluginsEnd, updatedPlugins)
	} else {
		pluginArray := append(append([]byte{'['}, encodedPlugin...), ']')
		member := append([]byte(`"plugins": `), pluginArray...)
		updatedExpo, err = appendJSONObjectMember(expoJSON, member)
		if err != nil {
			return nil, false, fmt.Errorf("could not add release signing plugin to %s: %w", appJSONPath, err)
		}
	}

	return spliceJSON(data, expoStart, expoEnd, updatedExpo), true, nil
}

func findJSONMember(object []byte, member string) (int, int, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(object))
	token, err := decoder.Token()
	if err != nil {
		return 0, 0, false, err
	}
	if token != json.Delim('{') {
		return 0, 0, false, fmt.Errorf("expected JSON object")
	}

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return 0, 0, false, err
		}
		name, ok := token.(string)
		if !ok {
			return 0, 0, false, fmt.Errorf("expected object key")
		}
		valueOffset := int(decoder.InputOffset())
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return 0, 0, false, err
		}
		if name == member {
			start, err := jsonValueStart(object, valueOffset)
			if err != nil {
				return 0, 0, false, err
			}
			return start, start + len(raw), true, nil
		}
	}
	return 0, 0, false, nil
}

func jsonValueStart(data []byte, offset int) (int, error) {
	for offset < len(data) && isJSONSpace(data[offset]) {
		offset++
	}
	if offset >= len(data) || data[offset] != ':' {
		return 0, fmt.Errorf("expected colon after object key")
	}
	offset++
	for offset < len(data) && isJSONSpace(data[offset]) {
		offset++
	}
	if offset >= len(data) {
		return 0, fmt.Errorf("missing JSON value")
	}
	return offset, nil
}

func appendJSONArray(array, value []byte) ([]byte, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(array, &items); err != nil {
		return nil, err
	}
	if len(array) < 2 || array[0] != '[' || array[len(array)-1] != ']' {
		return nil, fmt.Errorf("expected JSON array")
	}

	if len(items) == 0 {
		inner := array[1 : len(array)-1]
		if bytes.Contains(inner, []byte("\n")) {
			closeIndent := trailingIndent(inner)
			itemIndent := closeIndent + "  "
			contents := []byte(newlineFor(array) + itemIndent)
			contents = append(contents, value...)
			contents = append(contents, []byte(newlineFor(array)+closeIndent)...)
			return append(append([]byte{'['}, contents...), ']'), nil
		}
		result := append([]byte{'['}, value...)
		result = append(result, inner...)
		return append(result, ']'), nil
	}

	decoder := json.NewDecoder(bytes.NewReader(array))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	lastValueEnd, lastValueStart := 0, 0
	for decoder.More() {
		lastValueStart = skipJSONSpace(array, int(decoder.InputOffset()))
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		lastValueEnd = int(decoder.InputOffset())
	}

	var insertion []byte
	trailing := array[lastValueEnd : len(array)-1]
	if bytes.Contains(trailing, []byte("\n")) {
		insertion = []byte("," + newlineFor(array) + lineIndent(array, lastValueStart))
	} else {
		insertion = []byte(", ")
	}
	insertion = append(insertion, value...)
	return spliceJSON(array, lastValueEnd, lastValueEnd, insertion), nil
}

func appendJSONObjectMember(object, member []byte) ([]byte, error) {
	if len(object) < 2 || object[0] != '{' || object[len(object)-1] != '}' {
		return nil, fmt.Errorf("expected JSON object")
	}

	decoder := json.NewDecoder(bytes.NewReader(object))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	memberCount := 0
	firstKeyStart, lastValueEnd := 0, 0
	for decoder.More() {
		keyOffset := int(decoder.InputOffset())
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		if memberCount == 0 {
			firstKeyStart = skipJSONSpace(object, keyOffset)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		lastValueEnd = int(decoder.InputOffset())
		memberCount++
	}

	if memberCount == 0 {
		inner := object[1 : len(object)-1]
		if bytes.Contains(inner, []byte("\n")) {
			closeIndent := trailingIndent(inner)
			insertion := []byte(newlineFor(object) + closeIndent + "  ")
			insertion = append(insertion, member...)
			insertion = append(insertion, []byte(newlineFor(object)+closeIndent)...)
			return append(append([]byte{'{'}, insertion...), '}'), nil
		}
		result := append([]byte{'{'}, member...)
		result = append(result, inner...)
		return append(result, '}'), nil
	}

	var insertion []byte
	trailing := object[lastValueEnd : len(object)-1]
	if bytes.Contains(trailing, []byte("\n")) {
		insertion = []byte("," + newlineFor(object) + lineIndent(object, firstKeyStart))
	} else {
		insertion = []byte(", ")
	}
	insertion = append(insertion, member...)
	return spliceJSON(object, lastValueEnd, lastValueEnd, insertion), nil
}

func spliceJSON(data []byte, start, end int, replacement []byte) []byte {
	result := make([]byte, 0, len(data)-(end-start)+len(replacement))
	result = append(result, data[:start]...)
	result = append(result, replacement...)
	return append(result, data[end:]...)
}

func skipJSONSpace(data []byte, offset int) int {
	for offset < len(data) && isJSONSpace(data[offset]) {
		offset++
	}
	return offset
}

func isJSONSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func newlineFor(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func lineIndent(data []byte, offset int) string {
	lineStart := bytes.LastIndex(data[:offset], []byte("\n")) + 1
	for lineStart < offset && (data[lineStart] == ' ' || data[lineStart] == '\t') {
		lineStart++
	}
	return string(data[lineStart:offset])
}

func trailingIndent(data []byte) string {
	lineStart := bytes.LastIndex(data, []byte("\n")) + 1
	for lineStart < len(data) && (data[lineStart] == ' ' || data[lineStart] == '\t' || data[lineStart] == '\r') {
		lineStart++
	}
	if lineStart == len(data) {
		return string(data[bytes.LastIndex(data, []byte("\n"))+1:])
	}
	return ""
}
