//go:build !windows

package main

import "fmt"

func updateCLIWindows(string) error {
	return fmt.Errorf("Windows update helper called on a non-Windows platform")
}
