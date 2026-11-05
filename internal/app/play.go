package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// type Config struct {
// 	Pkg   string
// 	Track string
// 	AAB   string
// 	Key   string
// }

func newService(key string) (*androidpublisher.Service, error) {
	if key == "" {
		return nil, fmt.Errorf("required field \"key\" is missing from %s", configName)
	}
	info, err := os.Stat(key)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Google Play key does not exist at %s; check the \"key\" field in %s: %w", key, configName, err)
		}
		return nil, fmt.Errorf("could not access the Google Play key at %s; check the \"key\" field in %s: %w", key, configName, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("the path configured in the \"key\" field is a directory, not a file: %s", key)
	}

	svc, err := androidpublisher.NewService(context.Background(), option.WithCredentialsFile(key))
	if err != nil {
		return nil, fmt.Errorf("could not load Google Play key %s: %w", key, err)
	}
	return svc, nil
}

// showVersion displays versionCodes published to the track. It makes no changes.
func showVersion(cfg Config) error {
	fmt.Println("🔐 Validating Google Play credentials...")
	svc, err := newService(cfg.Key)
	if err != nil {
		return err
	}

	fmt.Printf("🔎 Querying Google Play (%s)...\n", cfg.Track)
	edit, err := svc.Edits.Insert(cfg.Pkg, &androidpublisher.AppEdit{}).Do()
	if err != nil {
		return err
	}
	// Without Commit, the edit is discarded when closed.
	defer svc.Edits.Delete(cfg.Pkg, edit.Id).Do()

	t, err := svc.Edits.Tracks.Get(cfg.Pkg, edit.Id, cfg.Track).Do()
	if err != nil {
		return err
	}

	var codes []int64
	var max int64
	for _, r := range t.Releases {
		for _, c := range r.VersionCodes {
			codes = append(codes, c)
			if c > max {
				max = c
			}
		}
	}

	fmt.Printf("🎯 Track:            %s\n", cfg.Track)
	fmt.Printf("📋 Versions:         %v\n", codes)
	fmt.Printf("🏷️  versionCode max: %d\n", max)
	fmt.Printf("🔗 Play Store:        https://play.google.com/store/apps/details?id=%s\n", cfg.Pkg)
	return nil
}

// uploadAAB uploads the already-built AAB and publishes it to the track.
func uploadAAB(cfg Config) error {
	f, err := os.Open(cfg.AAB)
	if err != nil {
		return fmt.Errorf("could not open AAB at %s; check the \"aab\" field in %s: %w", cfg.AAB, configName, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("could not get the size of AAB %s: %w", cfg.AAB, err)
	}
	fileSize := info.Size()

	fmt.Println("🔐 Validating Google Play credentials...")
	svc, err := newService(cfg.Key)
	if err != nil {
		return err
	}

	fmt.Println("🧾 Preparing upload...")
	edit, err := svc.Edits.Insert(cfg.Pkg, &androidpublisher.AppEdit{}).Do()
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			svc.Edits.Delete(cfg.Pkg, edit.Id).Do()
		}
	}()

	fmt.Printf("🔎 Checking track %q...\n", cfg.Track)
	t, err := svc.Edits.Tracks.Get(cfg.Pkg, edit.Id, cfg.Track).Do()
	if err != nil {
		return err
	}
	var maxPublished int64
	for _, r := range t.Releases {
		for _, c := range r.VersionCodes {
			if c > maxPublished {
				maxPublished = c
			}
		}
	}

	fmt.Printf("📦 File: %s (%s)\n", cfg.AAB, formatBytes(fileSize))
	printUploadProgress(0, fileSize)
	bundle, err := svc.Edits.Bundles.Upload(cfg.Pkg, edit.Id).
		ResumableMedia(context.Background(), f, fileSize, "application/octet-stream").
		ProgressUpdater(googleapi.ProgressUpdater(printUploadProgress)).
		Do()
	if err != nil {
		fmt.Println()
		return fmt.Errorf("could not upload AAB: %w", err)
	}
	printUploadProgress(fileSize, fileSize)
	fmt.Println()

	if bundle.VersionCode <= maxPublished {
		return fmt.Errorf("AAB versionCode (%d) must be greater than the published versionCode (%d); update versionCode and rebuild",
			bundle.VersionCode, maxPublished)
	}

	fmt.Printf("🚀 Publishing versionCode %d to %q...\n", bundle.VersionCode, cfg.Track)
	_, err = svc.Edits.Tracks.Update(cfg.Pkg, edit.Id, cfg.Track, &androidpublisher.Track{
		Track: cfg.Track,
		Releases: []*androidpublisher.TrackRelease{
			{
				VersionCodes: []int64{bundle.VersionCode},
				Status:       "completed",
			},
		},
	}).Do()
	if err != nil {
		return fmt.Errorf("could not assign the release: %w", err)
	}

	fmt.Println("⏳ Confirming publication...")
	if _, err := svc.Edits.Commit(cfg.Pkg, edit.Id).Do(); err != nil {
		return fmt.Errorf("could not commit the edit: %w", err)
	}
	committed = true

	fmt.Printf("✅ Uploaded versionCode %d to track %q\n", bundle.VersionCode, cfg.Track)
	return nil
}

func printUploadProgress(current, total int64) {
	if total <= 0 {
		fmt.Print("\r⬆️  Uploading AAB...")
		return
	}
	if current < 0 {
		current = 0
	} else if current > total {
		current = total
	}
	percent := int(float64(current) / float64(total) * 100)
	const width = 24
	filled := percent * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	fmt.Printf("\r⬆️  Uploading AAB [%s] %3d%% (%s / %s)", bar, percent, formatBytes(current), formatBytes(total))
}

func formatBytes(size int64) string {
	const mb = 1024 * 1024
	if size >= mb {
		return fmt.Sprintf("%.1f MB", float64(size)/mb)
	}
	return fmt.Sprintf("%.1f KB", float64(size)/1024)
}

func publishedMax(cfg Config) (int64, error) {
	svc, err := newService(cfg.Key)
	if err != nil {
		return 0, err
	}

	edit, err := svc.Edits.Insert(cfg.Pkg, &androidpublisher.AppEdit{}).Do()
	if err != nil {
		return 0, err
	}
	defer svc.Edits.Delete(cfg.Pkg, edit.Id).Do()

	t, err := svc.Edits.Tracks.Get(cfg.Pkg, edit.Id, cfg.Track).Do()
	if err != nil {
		return 0, err
	}

	var max int64
	for _, r := range t.Releases {
		for _, c := range r.VersionCodes {
			if c > max {
				max = c
			}
		}
	}
	return max, nil
}
