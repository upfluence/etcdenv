package etcdenv

import (
	"fmt"
	"os"
	"os/exec"
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

	go r.cmd.Run()

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

	_, err := r.cmd.Process.Wait()

	return err
}
