package etcdenv

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Runner struct {
	Command    []string
	DefaultEnv []string

	cmd *exec.Cmd
}

func NewRunner(command []string) *Runner {
	return &Runner{
		Command:    command,
		DefaultEnv: os.Environ(),
	}
}

func (r *Runner) buildEnvs(envVariables map[string]string) []string {
	envs := r.DefaultEnv

	for k, v := range envVariables {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	return envs
}

func (r *Runner) Start(envVariables map[string]string) error {
	if r.cmd != nil && r.cmd.Process != nil {
		return newError(ErrAlreadyStarted)
	}

	r.cmd = exec.Command(r.Command[0], r.Command[1:]...)

	r.cmd.Env = r.buildEnvs(envVariables)
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr
	r.cmd.Stdin = os.Stdin

	r.cmd.Start()

	return nil
}

func (r *Runner) Stop() error {
	if r.cmd == nil || r.cmd.Process == nil {
		return newError(ErrNotStarted)
	}

	r.cmd.Process.Kill()
	r.cmd.Process.Wait()

	r.cmd = nil

	return nil
}

func (r *Runner) Restart(envVariables map[string]string) error {
	r.Stop()

	return r.Start(envVariables)
}

func (r *Runner) Wait() error {
	if r.cmd == nil || r.cmd.Process == nil {
		return newError(ErrNotStarted)
	}

	err := r.cmd.Wait()

	return err
}

func (r *Runner) WatchProcess(exitStatus chan int) {
	time.Sleep(200 * time.Millisecond)
	err := r.Wait()
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			exitStatus <- status.ExitStatus()
		} else {
			exitStatus <- 0
		}
	} else {
		exitStatus <- 0
	}
}
