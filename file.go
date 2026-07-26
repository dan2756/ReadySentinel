package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
)

// fileTarget is ready once path exists (and, with nonempty, has content),
// or with gone, once it no longer exists.
type fileTarget struct {
	path     string
	nonempty bool
	gone     bool
}

func newFileTarget(path string, o *options) (target, error) {
	if path == "" {
		return nil, errors.New("file: empty path")
	}
	return &fileTarget{path: path, nonempty: o.nonempty, gone: o.gone}, nil
}

func (t *fileTarget) String() string {
	switch {
	case t.gone:
		return "removal of " + t.path
	case t.nonempty:
		return "non-empty file " + t.path
	}
	return "file " + t.path
}

func (t *fileTarget) Check(context.Context) error {
	fi, err := os.Stat(t.path)
	if t.gone {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		return errors.New("still exists")
	}

	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("does not exist")
	}
	if err != nil {
		return err
	}
	if t.nonempty {
		empty, err := isEmpty(t.path, fi)
		if err != nil {
			return err
		}
		if empty {
			return errors.New("is empty")
		}
	}
	return nil
}

// isEmpty reports whether a file has zero bytes or a directory has no entries.
func isEmpty(path string, fi fs.FileInfo) (bool, error) {
	if !fi.IsDir() {
		return fi.Size() == 0, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.Readdirnames(1); err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}
