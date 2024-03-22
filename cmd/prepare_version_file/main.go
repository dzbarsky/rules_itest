package main

import (
	"os"
	"path/filepath"
)

func sandboxEscape(path string) (string, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}

	path, err = os.Readlink(path)
	if err != nil {
		return "", err
	}

	return filepath.Clean(path), nil
}

func main() {
	rawVersionFilePath := os.Args[1]
	versionFilePath := os.Args[2]
	unusedInputsFilePath := os.Args[3]

	abs, err := sandboxEscape(rawVersionFilePath)
	if err != nil {
		panic(err)
	}
	err = os.Symlink(abs, versionFilePath)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(unusedInputsFilePath, []byte(rawVersionFilePath), 0644)
	if err != nil {
		panic(err)
	}
}
