/*
   Copyright 2014 Upfluence, Inc.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

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
	if r.cmd != nil {
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

	return r.cmd.Process.Kill()
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
