//go:build !linux

package queue

import "os"

func openDirectory(path string) (*os.Root, *os.File, error) {
	return nil, nil, ErrUnsupported
}

func openPrivate(root *os.Root, name string, flags int, create bool) (*os.File, error) {
	return nil, ErrUnsupported
}

func checkDirectory(info os.FileInfo) error {
	return ErrUnsupported
}

func checkFile(info os.FileInfo) error {
	return ErrUnsupported
}

func checkPath(root *os.Root, name string, info os.FileInfo) error {
	return ErrUnsupported
}

func lockFile(file *os.File) error {
	return ErrUnsupported
}
