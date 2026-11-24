//go:build !unix

package main

import "os/exec"

// isolate keeps the default cancel (kill the process) where process groups do not exist (local runs on Windows).
func isolate(*exec.Cmd) {}
