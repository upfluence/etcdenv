package etcdenv

import (
	"github.com/cenkalti/backoff"
	"github.com/coreos/go-etcd/etcd"
	"log"
	"reflect"
	"strings"
	"time"
)

type Context struct {
	Namespaces      []string
	Runner          *Runner
	ExitChan        chan bool
	RestartOnChange bool
	WatchedKeys     []string
	CurrentEnv      map[string]string

	etcdClient *etcd.Client
}

func NewContext(namespaces []string, endpoints, command []string, restart bool, watchedKeys []string) *Context {
	return &Context{
		Namespaces:      namespaces,
		Runner:          NewRunner(command),
		etcdClient:      etcd.NewClient(endpoints),
		RestartOnChange: restart,
		ExitChan:        make(chan bool),
		WatchedKeys:     watchedKeys,
		CurrentEnv:      make(map[string]string),
	}
}

func (ctx *Context) escapeNamespace(key string) string {
	for _, namespace := range ctx.Namespaces {
		if strings.HasPrefix(key, namespace) {
			key = strings.TrimPrefix(key, namespace)
			break
		}
	}

	return strings.TrimPrefix(key, "/")
}

func (ctx *Context) fetchEtcdVariables() map[string]string {
	result := make(map[string]string)

	for _, namespace := range ctx.Namespaces {
		response, err := ctx.etcdClient.Get(namespace, false, false)

		if err != nil {
			etcdErrorType := reflect.TypeOf(&etcd.EtcdError{})
			log.Println(err.Error())

			if !reflect.TypeOf(err).ConvertibleTo(etcdErrorType) {
				panic(err.Error())
			}

			if err.(*etcd.EtcdError).ErrorCode == etcd.ErrCodeEtcdNotReachable {
				log.Println("Can't join the etcd server, fallback to the env variables")

				break
			} else if err.(*etcd.EtcdError).ErrorCode == ErrKeyNotFound {
				log.Println("The namespace does not exist, fallback to the env variables")

				break
			} else {
				panic(err.Error())
			}
		}

		for _, node := range response.Node.Nodes {
			key := ctx.escapeNamespace(node.Key)
			if _, ok := result[key]; !ok {
				result[key] = node.Value
			}
		}
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
	etcdErrorType := reflect.TypeOf(&etcd.EtcdError{})
	ctx.CurrentEnv = ctx.fetchEtcdVariables()
	ctx.Runner.Start(ctx.CurrentEnv)

	if ctx.RestartOnChange {
		responseChan := make(chan *etcd.Response)

		for _, namespace := range ctx.Namespaces {
			go func() {
				var t time.Duration
				b := backoff.NewExponentialBackOff()
				b.Reset()

				for {
					resp, err := ctx.etcdClient.Watch(namespace, 0, true, nil, ctx.ExitChan)

					if err != nil {
						log.Println(err.Error())

						if !reflect.TypeOf(err).ConvertibleTo(etcdErrorType) {
							continue
						}

						if err.(*etcd.EtcdError).ErrorCode == etcd.ErrCodeEtcdNotReachable {
							t = b.NextBackOff()
							log.Printf("Can't join the etcd server, wait %v", t)
							time.Sleep(t)
						}

						if t == backoff.Stop {
							return
						} else {
							continue
						}
					}

					log.Printf("%s key changed", resp.Node.Key)

					if ctx.shouldRestart(ctx.escapeNamespace(resp.Node.Key), resp.Node.Value) {
						responseChan <- resp
					}
				}
			}()
		}

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
