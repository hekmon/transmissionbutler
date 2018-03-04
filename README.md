# TransmissionButler

Automagically manages your torrents seed life !

* Manages/maintains a minimal global ratio
* Does not mess with custom ratio torrents (skips them)
* (optionnal) Allows a limited period of unlimited seed (before switching back to the global ratio control)
* (optionnal) Auto deletes finished torrents (for stopped torrents with ratio above the configured target)

## Options / Configure

```json
{
    "server": {
        "host": "127.0.0.1",
        "port": 9090,
        "https": false,
        "user": "rpcuser",
        "password": "rpcpassword"
    },
    "butler": {
        "check_frequency_minutes": 60,
        "unlimited_seed_days": 90,
        "target_ratio": 3,
        "delete_when_done": true
    }
}
```

`unlimited_seed_days` can be set to `0` in order to deactivate the unlimited seed period.

## Build / Install

### Simple / Trying out

```bash
# Build
go get -u github.com/hekmon/transmissionbutler
cd "$GOPATH/github.com/hekmon/transmissionbutler"
go install
# Configure
vim config.json
# Run
./transmissionbutler
```

### Debian package


#### Build locally

```bash
go get -u github.com/hekmon/transmissionbutler
apt install -y --no-install-recommends debhelper build-essential dh-systemd
cd "$GOPATH/github.com/hekmon/transmissionbutler"
debuild --preserve-envvar PATH --preserve-envvar GOROOT -us -u
```

#### Build with Docker

```bash
go get -u github.com/hekmon/transmissionbutler
cd "$GOPATH/github.com/hekmon/transmissionbutler"
docker build -t go-debian-builder debian/go-debian-builder
docker run --rm -v "$GOPATH/src":/go/src -w /go/src/github.com/hekmon/transmissionbutler go-debian-builder dpkg-buildpackage -us -uc -b
```

#### Install / Configure / Run

```bash
# Install
dpkg -i ../transmissionbutler_X.Y.Z_amd64.deb
# Configure
vim /etc/transmissionbutler/config.json
# Run
systemctl start transmissionbutler.service
systemctl status transmissionbutler.service
```
