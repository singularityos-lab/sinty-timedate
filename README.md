# sinty-timedate

A native replacement for `timedatectl` on SintyOS. It queries and sets the
system clock, timezone, hardware clock (RTC), and network-time state, and is a
drop-in for the parts of `timedatectl` people actually use.

## Why it exists

SintyOS does not boot with systemd. PID 1 is `sinit` and services are driven by
`atomctl`. The stock `timedatectl` refuses to do anything in that world:

    System has not been booted with systemd as init system (PID 1). Can't operate.

`sinty-timedate` talks to the kernel, the zoneinfo database, the RTC, and
`chrony`/`atomctl` directly, so it works under `sinit` with no systemd and no
D-Bus dependency. Where systemd's tool leans on `systemd-timedated`, this one:

- reads the clock via `clock_gettime` and sets it via `clock_settime`;
- resolves the timezone from the `/etc/localtime` symlink and `/etc/timezone`;
- reads and writes the RTC with the `RTC_RD_TIME` / `RTC_SET_TIME` ioctls on
  `/dev/rtc0` (falling back to `hwclock` if no RTC device is reachable);
- reports "synchronized" from the kernel's own `adjtimex` status;
- reports and toggles network time by driving `chrony.service` through
  `atomctl` (`atomctl start|stop chrony.service`).

## Usage

    sinty-timedate [status]                  show the current time settings
    sinty-timedate list-timezones            list all known timezones
    sinty-timedate set-timezone ZONE         set the system timezone (root)
    sinty-timedate set-time "Y-M-D H:M:S"     set the system clock and RTC (root)
    sinty-timedate set-ntp true|false         enable/disable network time (root)
    sinty-timedate set-local-rtc true|false   keep the RTC in local time (root)
    sinty-timedate --help

`status` prints the same fields, in the same layout, as `timedatectl`:

    $ sinty-timedate
                   Local time: Sun 2026-07-12 22:00:41 CEST
               Universal time: Sun 2026-07-12 20:00:41 UTC
                     RTC time: Sun 2026-07-12 20:00:41
                    Time zone: Europe/Rome (CEST, +0200)
    System clock synchronized: yes
                  NTP service: active
              RTC in local TZ: no

Privileged verbs check `EUID == 0` and fail with a clear message otherwise.
`set-time` refuses while network time is active, exactly as `timedatectl` does.
`set-ntp` is a no-op with a clear message when `atomctl` is not present (e.g. on
a developer host that is not running `sinit`).

The RTC time policy (UTC vs local) set by `set-local-rtc` is persisted in
`/etc/sinty-timedate.conf`; everything else uses the standard system files.

## Build

    make build            # ./sinty-timedate
    make static           # stripped static binary for the image
    make test
    go build ./...        # plain build

Requires Go >= 1.25 and Linux. The only non-stdlib dependency is
`golang.org/x/sys/unix`, for the RTC ioctls and `adjtimex`.
