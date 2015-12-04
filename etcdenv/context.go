package etcdenv

import (
	"errors"
	"github.com/cenkalti/backoff"
	"github.com/coreos/go-etcd/etcd"
	"log"
	"os"
	"reflect"
	"strings"
	"time"
)

type Context struct {
	Namespaces        []string
	Runner            *Runner
	ExitChan          chan bool
	ShutdownBehaviour string
	WatchedKeys       []string
	CurrentEnv        map[string]string
	maxRetry          int
	etcdClient        *etcd.Client
}

func NewContext(namespaces []string, endpoints, command []string,
	shutdownBehaviour string, watchedKeys []string) (*Context, error) {

	if shutdownBehaviour != "keepalive" && shutdownBehaviour != "restart" &&
		shutdownBehaviour != "exit" {
		return nil,
			errors.New("Choose a correct shutdown behaviour : keepalive | exit | restart")
	}

	return &Context{
		Namespaces:        namespaces,
		Runner:            NewRunner(command),
		etcdClient:        etcd.NewClient(endpoints),
		ShutdownBehaviour: shutdownBehaviour,
		ExitChan:          make(chan bool),
		WatchedKeys:       watchedKeys,
		CurrentEnv:        make(map[string]string),
		maxRetry:          3,
	}, nil
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

func (ctx *Context) fetchEtcdNamespaceVariables(namespace string, currentRetry int, b *backoff.ExponentialBackOff) map[string]string {
	result := make(map[string]string)

	response, err := ctx.etcdClient.Get(namespace, false, false)

	if err != nil {
		etcdErrorType := reflect.TypeOf(&etcd.EtcdError{})
		log.Println(err.Error())

		if !reflect.TypeOf(err).ConvertibleTo(etcdErrorType) {
			panic(err.Error())
		}

		if err.(*etcd.EtcdError).ErrorCode == etcd.ErrCodeEtcdNotReachable {
			log.Println("Can't join the etcd server, fallback to the env variables")
		} else if err.(*etcd.EtcdError).ErrorCode == ErrKeyNotFound {
			log.Println("The namespace does not exist, fallback to the env variables")
		}

		if currentRetry < ctx.maxRetry {
			log.Println("retry fetching variables")
			t := b.NextBackOff()
			time.Sleep(t)
			return ctx.fetchEtcdNamespaceVariables(namespace, currentRetry+1, b)
		} else {
			return result
		}

	}

	for _, node := range response.Node.Nodes {
		key := ctx.escapeNamespace(node.Key)
		if _, ok := result[key]; !ok {
			result[key] = node.Value
		}
	}

	return result
}

func (ctx *Context) fetchEtcdVariables() map[string]string {
	result := make(map[string]string)

	b := backoff.NewExponentialBackOff()

	for _, namespace := range ctx.Namespaces {
		b.Reset()

		for key, value := range ctx.fetchEtcdNamespaceVariables(namespace, 0, b) {
			if _, ok := result[key]; !ok {
				result[key] = value
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

	responseChan := make(chan *etcd.Response)
	processExitChan := make(chan int)

	for _, namespace := range ctx.Namespaces {
		go func(namespace string) {
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
		}(namespace)
	}

	go ctx.Runner.WatchProcess(processExitChan)

	for {
		select {
		case <-responseChan:
			log.Println("Environment changed, restarting child process..")
			ctx.CurrentEnv = ctx.fetchEtcdVariables()
			ctx.Runner.Restart(ctx.CurrentEnv)
			log.Println("Process restarted")
		case <-ctx.ExitChan:
			log.Println("Asking the runner to stop")
			ctx.Runner.Stop()
			log.Println("Runner stopped")
		case status := <-processExitChan:
			log.Printf("Child process exited with status %d\n", status)
			if ctx.ShutdownBehaviour == "exit" {
				ctx.ExitChan <- true
				os.Exit(status)
			} else if ctx.ShutdownBehaviour == "restart" {
				ctx.CurrentEnv = ctx.fetchEtcdVariables()
				ctx.Runner.Restart(ctx.CurrentEnv)
				go ctx.Runner.WatchProcess(processExitChan)
				log.Println("Process restarted")
			}
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
