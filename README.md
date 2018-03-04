# TransmissionButler

Automagically manages your torrents seed life !

* Manages/maintains a minimal global ratio
* Does not mess with custom ratio torrents (skips them)
* (optionnal) Allows a limited period of unlimited seed (before switching back to the global ratio control)
* (optionnal) Auto deletes finished torrents (for stopped torrents with ratio above the configured target)

## Options / Configure

### Config file
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

### Behavior

Every `60` minutes the butler will scan each torrent :
* If the torrent has a custom ratio, it will be skipped
* If the torrent is seeding since less than `90` days, ratio will be deactivated for this torrent (unlimited seeding)
* If the torrent is seeding since more than `90` days, global ratio will be reactivated for this torrent
    * If its current ratio is above the global `3` ratio, transmission will automatically stop this torrent
    * Else, it will continue to seed until the global ratio is reached
* If the torrent is completed/stopped and have a ratio above the global setting (`3`), it will be deleted along with its files

Note that you can set `unlimited_seed_days` to `0` in order to deactivate the unlimited seed period.

## Build / Install

Check the [releases](https://github.com/hekmon/transmissionbutler/releases) page !

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
