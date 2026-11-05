# Releasing app-publisher

This guide is for maintainers publishing a new app-publisher release. The CLI's displayed version and the Go module tag use separate version numbers:

- `app-publisher --version` reports the product version, currently `1.0.0`.
- Go module releases use semantic version tags in the `v1.x` line. The module path remains `github.com/fgp555/app-publisher` so the user install command does not need to change each year.

## Release checklist

1. Update the `version` variable in the repository root's `main.go` to the product version for the release.
2. Review the README, generated workflows, and release notes for accuracy.
3. Run the project checks before releasing:

   ```sh
   go test ./...
   go vet ./...
   go build .
   ```

4. Commit and push the release changes to `main`.
5. Create a new Go module tag in the `v1.x` line. The first compatible Go module release after the already published `v1.0.0` tag should use `v1.0.0`; keep the CLI product version in `main.go` independent.
6. Push the tag and create a GitHub Release with user-facing notes.
7. Verify both pinned and latest installation from a clean environment:

   ```sh
   go install github.com/fgp555/app-publisher@v1.0.0
   go install github.com/fgp555/app-publisher@latest
   app-publisher --version
   ```

Do not move or reuse a published tag. Create a new patch or minor tag for follow-up fixes and features.
