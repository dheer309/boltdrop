package utils

import (
	"errors"
	"os"
)

// a function which checks if the given file path exists
func CheckFileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	//return !os.IsNotExist(err)
	return !errors.Is(error, os.ErrNotExist)
}
