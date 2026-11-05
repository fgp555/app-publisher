package app

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

var versionCodeRe = regexp.MustCompile(`(?m)^([ \t]*versionCode(?:[ \t]*=[ \t]*|[ \t]+))([0-9]+)\b`)

// bumpVersionCode writes the published versionCode plus one to build.gradle(.kts).
func bumpVersionCode(cfg Config) (int64, error) {
	max, err := publishedMax(cfg)
	if err != nil {
		return 0, err
	}
	next := max + 1

	path, err := findAppBuildFile(cfg.GradleRoot)
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("could not read %s: %w", path, err)
	}
	if !versionCodeRe.Match(data) {
		return 0, fmt.Errorf("no numeric versionCode declaration found in %s", path)
	}

	nextString := []byte(strconv.FormatInt(next, 10))
	out := versionCodeRe.ReplaceAllFunc(data, func(match []byte) []byte {
		groups := versionCodeRe.FindSubmatch(match)
		updated := append([]byte(nil), groups[1]...)
		return append(updated, nextString...)
	})
	if err := os.WriteFile(path, out, 0644); err != nil {
		return 0, fmt.Errorf("could not update versionCode in %s: %w", path, err)
	}
	return next, nil
}
