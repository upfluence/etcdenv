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
	WatchedKeys     []string
	CurrentEnv      map[string]string

	etcdClient *etcd.Client
}

func NewContext(namespace string, endpoints, command []string, restart bool, watchedKeys []string) *Context {
	return &Context{
		Namespace:       namespace,
		Runner:          NewRunner(command),
		etcdClient:      etcd.NewClient(endpoints),
		RestartOnChange: restart,
		ExitChan:        make(chan bool),
		WatchedKeys:     watchedKeys,
		CurrentEnv:      make(map[string]string),
	}
}

func (ctx *Context) escapeNamespace(key string) string {
	key = strings.TrimPrefix(key, ctx.Namespace)
	return strings.TrimPrefix(key, "/")
}

func (ctx *Context) fetchEtcdVariables() map[string]string {
	response, err := ctx.etcdClient.Get(ctx.Namespace, false, false)

	if err != nil {
		if err.(*etcd.EtcdError).ErrorCode == etcd.ErrCodeEtcdNotReachable {
			log.Println("Can't join the etcd server, fallback to the env variables")
			return make(map[string]string)
		} else {
			panic(err.Error())
		}
	}

	result := make(map[string]string)

	for _, node := range response.Node.Nodes {
		key := ctx.escapeNamespace(node.Key)
		result[key] = node.Value
	}

	return result
}

func (ctx *Context) shouldRestart(envVar, value string) bool {
	if v, ok := ctx.CurrentEnv[envVar]; ok && v == value {
		return false
	}

	if len(ctx.WatchedKeys) == 0 || containsString(ctx.WatchedKeys, envVar) {
		return true
	}

	return false
}

func (ctx *Context) Run() {
	ctx.CurrentEnv = ctx.fetchEtcdVariables()
	ctx.Runner.Start(ctx.CurrentEnv)

	if ctx.RestartOnChange {
		responseChan := make(chan *etcd.Response)

		go func() {
			for {
				resp, err := ctx.etcdClient.Watch(ctx.Namespace, 0, true, nil, ctx.ExitChan)

				if err != nil {
					continue
				}

				log.Printf("%s key changed", resp.Node.Key)

				if ctx.shouldRestart(ctx.escapeNamespace(resp.Node.Key), resp.Node.Value) {
					responseChan <- resp
				}
			}
		}()

		for {
			select {
			case <-responseChan:
				log.Println("Process restarted")
				ctx.CurrentEnv = ctx.fetchEtcdVariables()
				ctx.Runner.Restart(ctx.CurrentEnv)
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

func containsString(keys []string, item string) bool {
	for _, elt := range keys {
		if elt == item {
			return true
		}
	}

	return false
}
