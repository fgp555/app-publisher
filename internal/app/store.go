package app

import "fmt"

// openStore is the platform-neutral command entry point. Android is the
// currently supported platform; iOS store routing can be added here later.
func openStore(cfg Config) error {
	return openAndroidStore(cfg)
}

func openAndroidStore(cfg Config) error {
	adb, err := resolveAndroidTool("adb")
	if err != nil {
		return err
	}

	uri := fmt.Sprintf("market://details?id=%s", cfg.Pkg)
	fmt.Printf("🛍️  Opening Google Play listing for %s...\n", cfg.Pkg)
	return runTool(adb, cfg.GradleRoot,
		"shell", "am", "start",
		"-a", "android.intent.action.VIEW",
		"-d", uri,
	)
}
