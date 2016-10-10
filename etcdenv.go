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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/upfluence/etcdenv/etcdenv"
	"github.com/upfluence/goutils/log"
)

const currentVersion = "0.4.0"

var (
	flagset = flag.NewFlagSet("etcdenv", flag.ExitOnError)
	flags   = struct {
		Version           bool
		ShutdownBehaviour string
		Server            string
		Namespace         string
		WatchedKeys       string
	}{}
)

func usage() {
	fmt.Fprintf(os.Stderr, `
  NAME
  etcdenv - use your etcd keys as environment variables

  USAGE
  etcdenv [options] <command>

  OPTIONS
  `)
	flagset.PrintDefaults()
}

func init() {
	flagset.BoolVar(&flags.Version, "version", false, "Print the version and exit")
	flagset.BoolVar(&flags.Version, "v", false, "Print the version and exit")

	flagset.StringVar(&flags.ShutdownBehaviour, "b", "exit", "Behaviour when the process stop [exit|keepalive|restart]")
	flagset.StringVar(&flags.ShutdownBehaviour, "shutdown-behaviour", "exit", "Behaviour when the process stop [exit|keepalive|restart]")

	flagset.StringVar(&flags.Server, "server", "http://127.0.0.1:4001", "Location of the etcd server")
	flagset.StringVar(&flags.Server, "s", "http://127.0.0.1:4001", "Location of the etcd server")

	flagset.StringVar(&flags.Namespace, "namespace", "/environments/production", "etcd directory where the environment variables are fetched")
	flagset.StringVar(&flags.Namespace, "n", "/environments/production", "etcd directory where the environment variables are fetched")

	flagset.StringVar(&flags.WatchedKeys, "watched", "", "environment variables to watch, comma-separated")
	flagset.StringVar(&flags.WatchedKeys, "w", "", "environment variables to watch, comma-separated")
}

func main() {
	var watchedKeysList []string

	flagset.Parse(os.Args[1:])
	flagset.Usage = usage

	if len(os.Args) < 2 {
		flagset.Usage()
		os.Exit(0)
	}

	if flags.Version {
		fmt.Printf("etcdenv v%s", currentVersion)
		os.Exit(0)
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	if flags.WatchedKeys == "" {
		watchedKeysList = []string{}
	} else {
		watchedKeysList = strings.Split(flags.WatchedKeys, ",")
	}

	ctx, err := etcdenv.NewContext(
		strings.Split(flags.Namespace, ","),
		[]string{flags.Server},
		flagset.Args(),
		flags.ShutdownBehaviour,
		watchedKeysList,
	)

	if err != nil {
		log.Fatalf(err.Error())
		os.Exit(1)
	}

	go ctx.Run()

	select {
	case sig := <-signalChan:
		log.Noticef("Received signal %s", sig)
		ctx.ExitChan <- true
	case <-ctx.ExitChan:
		log.Infof("Catching ExitChan, doing nothin'")
	}
}
