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
	"github.com/coreos/go-etcd/etcd"
	"log"
	"strings"
	"time"
)

type Context struct {
	Namespace       string
	Runner          *Runner
	ExitChan        chan bool
	RestartOnChange bool

	etcdClient *etcd.Client
}

func NewContext(namespace string, endpoints, command []string, restart bool) *Context {
	return &Context{
		Namespace:       namespace,
		Runner:          NewRunner(command),
		etcdClient:      etcd.NewClient(endpoints),
		RestartOnChange: restart,
		ExitChan:        make(chan bool),
	}
}

func (ctx *Context) fetchEtcdVariables() map[string]string {
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

func (ctx *Context) Run() {
	ctx.Runner.Start(ctx.fetchEtcdVariables())

	if ctx.RestartOnChange {
		responseChan := make(chan *etcd.Response)

		go func() {
			for {
				resp, err := ctx.etcdClient.Watch(ctx.Namespace, 0, true, nil, ctx.ExitChan)
				if err != nil {
					continue
				}
				responseChan <- resp
			}
		}()

		for {
			select {
			case <-responseChan:
				log.Println("Process restarted")

				ctx.Runner.Restart(ctx.fetchEtcdVariables())
			case <-ctx.ExitChan:
				ctx.Runner.Stop()
			}
		}

	} else {
		processExitChan := make(chan bool)

		time.Sleep(200 * time.Millisecond)

		go func() {
			ctx.Runner.Wait()
			processExitChan <- true
		}()

		select {
		case <-ctx.ExitChan:
			ctx.Runner.Stop()
		case <-processExitChan:
			ctx.ExitChan <- true
		}
	}
}
