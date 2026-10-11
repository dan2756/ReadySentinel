//go:build !linux

package main

// isZombie always reports false where there is no /proc to look at; an
// unreaped child counts as running until its parent waits for it.
func isZombie(int) bool { return false }
