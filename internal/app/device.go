package app

import (
	"fmt"
	"strings"
)

func requireConnectedAndroidDevice(cfg Config) error {
	adb, err := resolveAndroidTool("adb")
	if err != nil {
		return err
	}
	output, err := captureToolOutput(adb, cfg.GradleRoot, "devices")
	if err != nil {
		return fmt.Errorf("could not check connected Android devices with adb: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			return nil
		}
	}
	return fmt.Errorf("no authorized Android device or emulator detected; connect and unlock a device, accept the USB debugging prompt, and confirm `adb devices` shows state `device`")
}
