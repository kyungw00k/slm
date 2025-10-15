// +build !release

package main

import "errors"

// Dummy functions for development build
func Asset(name string) ([]byte, error) {
	return nil, errors.New("assets not bundled")
}

func AssetDir(name string) ([]string, error) {
	return nil, errors.New("assets not bundled")
}

func RestoreAssets(dir, name string) error {
	return errors.New("assets not bundled")
}