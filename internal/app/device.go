package app

import (
	"fmt"
	"strings"
)

func requireConnectedAndroidDevice(cfg Config) error {
	_, devices, err := connectedAndroidDevices(cfg)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return noAuthorizedAndroidDeviceError()
	}
	return nil
}

func connectedAndroidDevices(cfg Config) (string, []string, error) {
	adb, err := resolveAndroidTool("adb")
	if err != nil {
		return "", nil, err
	}
	output, err := captureToolOutput(adb, cfg.GradleRoot, "devices")
	if err != nil {
		return "", nil, fmt.Errorf("could not check connected Android devices with adb: %w", err)
	}
	var devices []string
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			devices = append(devices, fields[0])
		}
	}
	return adb, devices, nil
}

func noAuthorizedAndroidDeviceError() error {
	return fmt.Errorf("no authorized Android device or emulator detected; connect and unlock a device, accept the USB debugging prompt, and confirm `adb devices` shows state `device`")
}
