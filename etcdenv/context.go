package etcdenv

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
	response, err := ctx.etcdClient.Get(ctx.Namespace, false, false)

	if err != nil {
		panic(err.Error())
	}

	result := make(map[string]string)

	for _, node := range response.Node.Nodes {
		key := strings.TrimPrefix(node.Key, ctx.Namespace)
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
