package etcdenv

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/coreos/go-etcd/etcd"
	"github.com/upfluence/goutils/log"
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
	shutdownBehaviour string, watchedKeys []string, username string, password string) (*Context, error) {

	if shutdownBehaviour != "keepalive" && shutdownBehaviour != "restart" &&
		shutdownBehaviour != "exit" {
		return nil,
			errors.New(
				"Choose a correct shutdown behaviour : keepalive | exit | restart",
			)
	}

        etcdClient := etcd.NewClient(endpoints)

        if username != "" && password != "" {
                etcdClient.SetCredentials(username, password)
        }

	return &Context{
		Namespaces:        namespaces,
		Runner:            NewRunner(command),
		etcdClient:        etcdClient,
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
		log.Errorf("etcd fetching error: %s", err.Error())

		if e, ok := err.(*etcd.EtcdError); ok && e.ErrorCode == etcd.ErrCodeEtcdNotReachable {
			log.Error("Can't join the etcd server, fallback to the env variables")
		} else if e, ok := err.(*etcd.EtcdError); ok && e.ErrorCode == ErrKeyNotFound {
			log.Error("The namespace does not exist, fallback to the env variables")
		}

		if currentRetry < ctx.maxRetry {
			log.Info("retry fetching variables")
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
					log.Errorf("etcd fetching error: %s", err.Error())

					if e, ok := err.(*etcd.EtcdError); ok && e.ErrorCode == etcd.ErrCodeEtcdNotReachable {
						t = b.NextBackOff()
						log.Noticef("Can't join the etcd server, wait %v", t)
						time.Sleep(t)
					}

					if t == backoff.Stop {
						return
					} else {
						continue
					}
				}

				log.Infof("%s key changed", resp.Node.Key)

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
			log.Notice("Environment changed, restarting child process..")
			ctx.CurrentEnv = ctx.fetchEtcdVariables()
			ctx.Runner.Restart(ctx.CurrentEnv)
			log.Notice("Process restarted")
		case <-ctx.ExitChan:
			log.Notice("Asking the runner to stop")
			ctx.Runner.Stop()
			os.Stderr.Sync()
			log.Notice("Runner stopped")
		case status := <-processExitChan:
			log.Noticef("Child process exited with status %d", status)
			if ctx.ShutdownBehaviour == "exit" {
				ctx.ExitChan <- true
				os.Stderr.Sync()
				os.Exit(status)
			} else if ctx.ShutdownBehaviour == "restart" {
				ctx.CurrentEnv = ctx.fetchEtcdVariables()
				ctx.Runner.Restart(ctx.CurrentEnv)
				go ctx.Runner.WatchProcess(processExitChan)
				log.Notice("Process restarted")
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
