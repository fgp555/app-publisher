package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const deployWorkflow = `name: Deploy Android app to Google Play

on:
  push:
    branches:
      - main
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
          printf '%s' "$RELEASE_KEYSTORE_BASE64" | base64 --decode > app/release.keystore

      - name: Make Gradle wrapper executable
        run: chmod +x gradlew

      - name: Build and publish
        run: play-tool deploy
        env:
          PLAY_TOOL_KEY: ${{ runner.temp }}/play-service-account.json
          KEYSTORE_PASSWORD: ${{ secrets.KEYSTORE_PASSWORD }}
          KEY_ALIAS: ${{ secrets.KEY_ALIAS }}
          KEY_PASSWORD: ${{ secrets.KEY_PASSWORD }}
`

func initWorkflow() error {
	workflowPath := filepath.Join(".github", "workflows", "deploy-go.yml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		return fmt.Errorf("could not create workflow directory: %w", err)
	}

	file, err := os.OpenFile(workflowPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("workflow already exists at %s; leaving it unchanged", workflowPath)
		}
		return fmt.Errorf("could not create workflow %s: %w", workflowPath, err)
	}

	if _, err := file.WriteString(deployWorkflow); err != nil {
		file.Close()
		os.Remove(workflowPath)
		return fmt.Errorf("could not write workflow %s: %w", workflowPath, err)
	}
	if err := file.Close(); err != nil {
		os.Remove(workflowPath)
		return fmt.Errorf("could not close workflow %s: %w", workflowPath, err)
	}

	fmt.Printf("✅ Created GitHub Actions workflow: %s\n", workflowPath)
	return nil
}
