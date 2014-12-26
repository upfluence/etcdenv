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
)

const currentVersion = "0.0.1"

var (
	flagset = flag.NewFlagSet("etcdenv", flag.ExitOnError)
	flags   = struct {
		Version         bool
		RestartOnChange bool

		Server    string
		Namespace string
	}{}
)

func init() {
	flagset.BoolVar(&flags.Version, "version", false, "Print the version and exit")
	flagset.BoolVar(&flags.Version, "v", false, "Print the version and exit")

	flagset.BoolVar(&flags.RestartOnChange, "auto-restart", true, "Automaticly restart the command when a value change")
	flagset.BoolVar(&flags.RestartOnChange, "r", true, "Automaticly restart the command when a value change")

	flagset.StringVar(&flags.Server, "server", "http://127.0.0.1:4001", "Location of the etcd server")
	flagset.StringVar(&flags.Server, "s", "http://127.0.0.1:4001", "Location of the etcd server")

	flagset.StringVar(&flags.Namespace, "namespace", "/environments/production", "etcd directory where the environment variables are fetched")
	flagset.StringVar(&flags.Namespace, "n", "/environments/production", "etcd directory where the environment variables are fetched")
}

func main() {
	flagset.Parse(os.Args[1:])

	if flags.Version {
		fmt.Printf("etcdenv v.%s", currentVersion)
		os.Exit(0)
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt, os.Kill)

	ctx := NewContext(
		flags.Namespace,
		[]string{flags.Server},
		flagset.Args(),
		flags.RestartOnChange,
	)

	go ctx.Run()

	select {
	case <-signalChan:
		ctx.ExitChan <- true
	case <-ctx.ExitChan:
	}
}
