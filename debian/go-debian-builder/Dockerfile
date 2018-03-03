FROM golang:latest
RUN apt-get update && apt-get install -y --no-install-recommends debhelper build-essential dh-systemd && apt-get clean
# Run it with :
#   docker run --rm -v "$GOPATH/src":/go/src -w /go/src/github.com/hekmon/transmissionbutler go-debian-builder dpkg-buildpackage -us -uc -b
