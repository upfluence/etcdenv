# etcdenv

Etcdenv provide a convenient way to populate environments variables from one your etcd directory

## Installation

Easy! You can just download the binary from the command line:

* Linux

```shell
curl -sL https://github.com/upfluence/etcdenv/releases/download/v0.0.1/etcdenv-linux-amd64-0.0.1 > etcdenv
```

* OSX

```shell
curl -sL https://github.com/upfluence/etcdenv/releases/download/v0.0.1/etcdenv-darwin-amd64-0.0.1 > etcdenv
```

If you would prefer compile the binary (assuming buildtools and Go are installed) :

```shell
git clone git@github.com:upfluence/etcdenv.git
cd etcdenv
go get github.com/tools/godep
GOPATH=`pwd`/Godeps/_workspace go build -o etcdenv .
```

## Usage

### Options

| Option | Default | Description |
| ------ | ------- | ----------- |
| `server`, `s` | http://127.0.0.1:4001 | Location of the etcd server |
| `namespace`, `n`| /environments/production | Etcd directory where the environment variables are fetched. You can watch multiple namespaces by using a comma-separated list (/environments/production,/environments/global) |
| `shutdownBehavour`, `b` | keepalive | Strategy to apply when the process exit, further information into the next paragraph |
| `watched`, `w` | `""` | A comma-separated list of environment variables triggering the command restart when they change |


### Shutdown strategies

* `restart`: `etcdenv` rerun the command when the wrapped process exits
* `exit`:  The `etcdenv` process exits with the same exit status as the
  wrapped process's
* `keepalive`: The `etcdenv` process will stay alive and keep looking
  for etcd changes to re-run the command

### Command line

The CLI interface supports all of the options detailed above.


#### Example

*Assuming a etcd server is launched on your machine*

**You can print env variables fetched from the `/` directory of your local node of etcd**

```shell
$ curl -XPOST -d "value=bar" http://127.0.0.1:4001/v2/keys/FOO
$ etcdenv -n / -r false printenv
# ... your local environment variables
FOO=bar
$
```

**You can also follow the changes of the directory**

```shell
$ curl -XPOST -d "value=bar" http://127.0.0.1:4001/v2/keys/FOO
$ etcdenv -n / printenv &
# ... your local environment variables
FOO=bar
$ curl -XPOST -d "value=buz" http://127.0.0.1:4001/v2/keys/FOO
2014/12/29 00:30:00 Restarted
# ... your local environment variables
FOO=buz
```

**To watch a set of keys only**

```shell
$ curl -XPOST -d "value=google.com" http://127.0.0.1:4001/v2/keys/GOOGLE_URL
$ etcdenv -n / -w "FOO,GOOGLE_URL" printenv &
# ... your local environment variables
GOOGLE_URL=google.com
$ curl -XPOST -d "value=foo" http://127.0.0.1:4001/v2/keys/BAR
# ... the running command does not restart
$ curl -XPOST -d "value=baz" http://127.0.0.1:4001/v2/keys/FOO
# ... your local environment variables
FOO=baz
```

## Contributing

