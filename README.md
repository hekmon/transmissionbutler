# TransmissionButler

Automagically manages your torrents seed life !

* Manages/maintains a minimal global ratio
* Does not mess with seeding torrents with custom ratio (skips them)
* (optionnal) Allows a limited period of free seed (no ratio period before switching back to the global ratio or custom ratio mode)
* (optionnal) Auto deletes finished torrents (for stopped torrents with ratio above their configured target)

## Options / Configure

### Config file

```json
{
    "server": {
        "host": "127.0.0.1",
        "port": 9091,
        "https": false,
        "user": "rpcuser",
        "password": "rpcpassword"
    },
    "butler": {
        "check_frequency_minutes": 60,
        "free_seed_days": 90,
        "target_ratio": 4,
        "restore_custom": true,
        "delete_when_done": true
    },
    "pushover": {
        "app_key": null,
        "user_key": null
    }
}
```

### Behavior

(based on previous config file)

Every `60` minutes the butler will check the global ratio setting and will scan each torrent:

* If the torrent is sending:
  * and has a custom ratio, it will be skipped for seeding tests
  * since less than `90` days, ratio will be deactivated for this torrent (free seed period)
  * since more than `90` days:
    * custom ratio will be activated if the torrent has a custom ratio value saved different than the global ratio (because of `restore_custom`)
    * global ratio will be activated as either `restore_custom` is off or the saved custom ratio value matches the global ratio value
* (because of the `delete_when_done`) The torrent will be deleted along with its files if it is completed/stopped and is:
  * on the global ratio mode and have a ratio above the global setting (`4`)
  * on a custom ratio mode and have its current ratio above its custom ratio

Note that you can set `unlimited_seed_days` to `0` in order to deactivate the unlimited seed period.

In order to have [pushover](https://pushover.net/) notifications from the butler, `app_key` and `user_key` must not be `null`.

## Build / Install

Check the [releases](https://github.com/hekmon/transmissionbutler/releases) page !

### Simple / Trying out

```bash
# Build
go get -u github.com/hekmon/transmissionbutler
cd "$GOPATH/src/github.com/hekmon/transmissionbutler"
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
cd "$GOPATH/src/github.com/hekmon/transmissionbutler"
debuild --preserve-envvar PATH --preserve-envvar GOROOT -us -u
```

#### Build with Docker

```bash
go get -u github.com/hekmon/transmissionbutler
cd "$GOPATH/src/github.com/hekmon/transmissionbutler"
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
