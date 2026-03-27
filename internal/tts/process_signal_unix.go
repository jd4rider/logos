//go:build !windows

package tts

import (
	"os"
	"syscall"
)

func pausePlaybackProcess(process *os.Process) error {
	return process.Signal(syscall.SIGSTOP)
}

func resumePlaybackProcess(process *os.Process) error {
	return process.Signal(syscall.SIGCONT)
}
