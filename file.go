package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// fileTarget is ready once path exists (and, with nonempty, has content),
// or with gone, once it no longer exists.
type fileTarget struct {
	path     string
	nonempty bool
	gone     bool

	running chan error // result of a check an earlier attempt gave up on
}

// stat and lstat are os.Stat and os.Lstat. Tests replace them.
var stat, lstat = os.Stat, os.Lstat

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

// Check runs the file system calls in the background, because they cannot
// be cancelled and on a hung network mount they can block for minutes;
// waiting on ctx instead keeps the timeouts and Ctrl-C working. If an attempt
// gives up on a check, the next attempt waits for that same check rather than
// starting another one. Check must not be called concurrently.
func (t *fileTarget) Check(ctx context.Context) error {
	if t.running == nil {
		ch := make(chan error, 1)
		go func() { ch <- t.check() }()
		t.running = ch
	}
	select {
	case err := <-t.running:
		t.running = nil
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *fileTarget) check() error {
	if t.gone {
		// Lstat, so that a symlink counts as present even when what it
		// points to doesn't exist, as with "ln -s $$ deploy.lock" locks.
		// ENOTDIR means part of the path is not a directory, so the path
		// cannot exist.
		_, err := lstat(t.path)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return nil
		}
		if err != nil {
			return err
		}
		return errors.New("still exists")
	}

	fi, err := stat(t.path)
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
