package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var signerFingerprintRe = regexp.MustCompile(`(?i)certificate SHA-(1|256) digest:\s*([[:xdigit:]:]+)`)
var keytoolFingerprintRe = regexp.MustCompile(`(?im)^\s*SHA-?(1|256):\s*(.+)$`)

const bundletoolReleasesURL = "https://github.com/google/bundletool/releases"

type artifactInfo struct {
	Package     string
	VersionCode string
	VersionName string
	MinSDK      string
	TargetSDK   string
	SHA1        string
	SHA256      string
	Signature   string
	Path        string
	Size        int64
}

func showArtifactInfo(cfg Config, target string) error {
	var info artifactInfo
	var err error
	switch target {
	case "assembleDebug":
		info, err = inspectAPK(cfg.APKDebug)
	case "assembleRelease":
		info, err = inspectAPK(cfg.APKRelease)
	case "bundleRelease":
		info, err = inspectBundle(cfg.AAB)
	default:
		return fmt.Errorf("invalid artifact %q; use assembleDebug, assembleRelease, or bundleRelease", target)
	}
	if err != nil {
		return err
	}
	printArtifactInfo(info, target)
	return nil
}

func inspectAPK(path string) (artifactInfo, error) {
	if err := requireFile(path, "APK"); err != nil {
		return artifactInfo{}, err
	}
	apkanalyzer, err := resolveAndroidTool("apkanalyzer")
	if err != nil {
		return artifactInfo{}, err
	}
	apksigner, err := resolveAndroidTool("apksigner")
	if err != nil {
		return artifactInfo{}, err
	}

	info := artifactInfo{Path: path}
	queries := []struct {
		args []string
		dst  *string
	}{
		{[]string{"manifest", "application-id", path}, &info.Package},
		{[]string{"manifest", "version-code", path}, &info.VersionCode},
		{[]string{"manifest", "version-name", path}, &info.VersionName},
		{[]string{"manifest", "min-sdk", path}, &info.MinSDK},
		{[]string{"manifest", "target-sdk", path}, &info.TargetSDK},
	}
	for _, query := range queries {
		output, err := captureToolOutput(apkanalyzer, filepath.Dir(path), query.args...)
		if err != nil {
			return artifactInfo{}, fmt.Errorf("apkanalyzer %s: %w", strings.Join(query.args[:2], " "), err)
		}
		*query.dst = strings.TrimSpace(string(output))
	}

	certOutput, err := captureCombinedToolOutput(apksigner, filepath.Dir(path), "verify", "--print-certs", path)
	if err != nil {
		return artifactInfo{}, fmt.Errorf("could not verify APK signature: %w", err)
	}
	info.Signature = "valid"
	info.SHA1, info.SHA256 = parseFingerprints(signerFingerprintRe, certOutput)
	stat, err := os.Stat(path)
	if err != nil {
		return artifactInfo{}, fmt.Errorf("could not read APK size: %w", err)
	}
	info.Size = stat.Size()
	return info, nil
}

func inspectBundle(path string) (artifactInfo, error) {
	if err := requireFile(path, "AAB"); err != nil {
		return artifactInfo{}, err
	}
	bundletool, err := resolveBundletoolJar()
	if err != nil {
		return artifactInfo{}, err
	}
	java, err := resolveJDKTool("java")
	if err != nil {
		return artifactInfo{}, err
	}
	keytool, err := resolveJDKTool("keytool")
	if err != nil {
		return artifactInfo{}, err
	}
	jarsigner, err := resolveJDKTool("jarsigner")
	if err != nil {
		return artifactInfo{}, err
	}

	manifest, err := captureToolOutput(java, filepath.Dir(path), "-jar", bundletool, "dump", "manifest", "--bundle="+path)
	if err != nil {
		return artifactInfo{}, fmt.Errorf("bundletool could not read the AAB manifest: %w", err)
	}
	info := artifactInfo{Path: path}
	if err := readBundleManifest(manifest, &info); err != nil {
		return artifactInfo{}, err
	}
	certOutput, err := captureToolOutput(keytool, filepath.Dir(path), "-printcert", "-jarfile", path)
	if err != nil {
		return artifactInfo{}, fmt.Errorf("could not read AAB certificate: %w", err)
	}
	info.SHA1, info.SHA256 = parseFingerprints(keytoolFingerprintRe, certOutput)
	if _, err := captureToolOutput(jarsigner, filepath.Dir(path), "-verify", path); err != nil {
		return artifactInfo{}, fmt.Errorf("could not verify AAB JAR signature: %w", err)
	}
	info.Signature = "valid (upload certificate)"
	stat, err := os.Stat(path)
	if err != nil {
		return artifactInfo{}, fmt.Errorf("could not read AAB size: %w", err)
	}
	info.Size = stat.Size()
	return info, nil
}

func readBundleManifest(data []byte, info *artifactInfo) error {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("bundletool returned an invalid AAB manifest: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range start.Attr {
			switch start.Name.Local {
			case "manifest":
				switch attr.Name.Local {
				case "package":
					info.Package = attr.Value
				case "versionCode":
					info.VersionCode = attr.Value
				case "versionName":
					info.VersionName = attr.Value
				}
			case "uses-sdk":
				switch attr.Name.Local {
				case "minSdkVersion":
					info.MinSDK = attr.Value
				case "targetSdkVersion":
					info.TargetSDK = attr.Value
				}
			}
		}
	}
	return nil
}

func parseFingerprints(re *regexp.Regexp, output []byte) (sha1, sha256 string) {
	for _, match := range re.FindAllSubmatch(output, -1) {
		if len(match) != 3 {
			continue
		}
		switch strings.TrimSpace(string(match[1])) {
		case "1":
			sha1 = strings.TrimSpace(string(match[2]))
		case "256":
			sha256 = strings.TrimSpace(string(match[2]))
		}
	}
	return sha1, sha256
}

func printArtifactInfo(info artifactInfo, variant string) {
	fmt.Printf("\n📦 Artifact: %s (%s)\n", variant, formatBytes(info.Size))
	fmt.Printf("📍 File:      %s\n", info.Path)
	fmt.Printf("📱 Package:   %s\n", valueOrUnknown(info.Package))
	fmt.Printf("🔢 versionCode: %s\n", valueOrUnknown(info.VersionCode))
	fmt.Printf("🏷️  versionName: %s\n", valueOrUnknown(info.VersionName))
	fmt.Printf("🤖 minSdk:      %s\n", valueOrUnknown(info.MinSDK))
	fmt.Printf("🎯 targetSdk:   %s\n", valueOrUnknown(info.TargetSDK))
	fmt.Printf("🛡️  Signature:  %s\n", valueOrUnknown(info.Signature))
	fmt.Printf("🔐 SHA-1:       %s\n", formatFingerprint(info.SHA1))
	fmt.Printf("🔐 SHA-256:     %s\n", formatFingerprint(info.SHA256))
}

func formatFingerprint(value string) string {
	digest := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
	if digest == "" || len(digest)%2 != 0 {
		return "no disponible"
	}
	var formatted strings.Builder
	for i := 0; i < len(digest); i += 2 {
		if i > 0 {
			formatted.WriteByte(':')
		}
		formatted.WriteString(digest[i : i+2])
	}
	return formatted.String()
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "no disponible"
	}
	return value
}

func captureToolOutput(path, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, err
}

func captureCombinedToolOutput(path, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	return output, nil
}

func resolveBundletoolJar() (string, error) {
	path := strings.TrimSpace(os.Getenv("PLAY_BUNDLETOOL_JAR"))
	if path == "" {
		return "", fmt.Errorf("to inspect an AAB, download bundletool-all.jar from %s and set PLAY_BUNDLETOOL_JAR to its path", bundletoolReleasesURL)
	}
	if err := requireFile(path, "bundletool JAR"); err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve PLAY_BUNDLETOOL_JAR path %s: %w", path, err)
	}
	return absolute, nil
}

func resolveJDKTool(name string) (string, error) {
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		if path := toolInDir(filepath.Join(javaHome, "bin"), name); path != "" {
			return path, nil
		}
	}
	if path, err := lookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found; check JAVA_HOME or add the JDK to PATH", name)
}

