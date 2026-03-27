//go:build windows

package tts

import (
	"errors"
	"os"
)

var errPauseResumeUnsupported = errors.New("pause and resume are not supported on windows")

func pausePlaybackProcess(process *os.Process) error {
	_ = process
	return errPauseResumeUnsupported
}

func resumePlaybackProcess(process *os.Process) error {
	_ = process
	return errPauseResumeUnsupported
}
