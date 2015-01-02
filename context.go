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
	"github.com/coreos/go-etcd/etcd"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Context struct {
	Namespace       string
	Command         []string
	ExitChan        chan bool
	RestartOnChange bool

	etcdClient     *etcd.Client
	currentEnviron []string
}

func NewContext(namespace string, endpoints, command []string, restart bool) *Context {
	return &Context{
		Namespace:       namespace,
		Command:         command,
		etcdClient:      etcd.NewClient(endpoints),
		currentEnviron:  os.Environ(),
		RestartOnChange: restart,
		ExitChan:        make(chan bool),
	}
}

func (ctx *Context) getEnvs() []string {
	result := ctx.currentEnviron

	for k, v := range ctx.fetchEnvVariables() {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}

	return result
}

func (ctx *Context) fetchEnvVariables() map[string]string {
	response, err := ctx.etcdClient.Get(flags.Namespace, false, false)

	if err != nil {
		panic(err.Error())
	}

	result := make(map[string]string)

	for _, node := range response.Node.Nodes {
    key := strings.TrimPrefix(node.Key, flags.Namespace)
    key = strings.TrimPrefix(key, "/")
    result[key] = node.Value
	}

	return result
}

func (ctx *Context) runCommand() *exec.Cmd {
	cmd := exec.Command(ctx.Command[0], ctx.Command[1:]...)

	cmd.Env = ctx.getEnvs()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	go cmd.Run()

	return cmd
}

func (ctx *Context) Run() {
	cmd := ctx.runCommand()

	if ctx.RestartOnChange {
		responseChan := make(chan *etcd.Response)

		go ctx.etcdClient.Watch(ctx.Namespace, 0, true, responseChan, ctx.ExitChan)

		for {
			select {
			case <-responseChan:
				log.Println("Restarted")

				cmd.Process.Kill()
				cmd = ctx.runCommand()

			case <-ctx.ExitChan:
				cmd.Process.Kill()
			}
		}

	} else {
		processExitChan := make(chan bool)

		time.Sleep(200 * time.Millisecond)

		go func() {
			cmd.Process.Wait()
			processExitChan <- true
		}()

		select {
		case <-ctx.ExitChan:
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		case <-processExitChan:
			ctx.ExitChan <- true
		}
	}
}
