FROM busybox

ENV ETCDENV_VERSION=0.4.1

ADD https://github.com/upfluence/etcdenv/releases/download/v${ETCDENV_VERSION}/etcdenv-linux-amd64-${ETCDENV_VERSION} /etcdenv
RUN ["chmod", "+x", "/etcdenv"]

ENTRYPOINT ["/etcdenv"]
