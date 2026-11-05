//go:build !windows

package app

import "fmt"

func updateCLIWindows(string) error {
	return fmt.Errorf("Windows update helper called on a non-Windows platform")
}
