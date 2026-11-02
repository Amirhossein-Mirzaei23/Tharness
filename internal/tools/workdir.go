package tools

import "os"

// absoluteWorkdir returns the process working directory as an absolute path.
func absoluteWorkdir() (string, error) {
	return os.Getwd()
}
