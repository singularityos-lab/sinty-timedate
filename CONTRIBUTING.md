# Contributing to sinty-timedate

Thanks for your interest in contributing!

## Quick Start

```bash
git clone https://github.com/singularityos-lab/sinty-timedate
cd sinty-timedate
go build ./...
go test ./...
```

## Guidelines

- Keep changes additive and tested. sinty-timedated runs as root and sets the system clock and time zone;
  prefer small, verifiable commits over large rewrites.
- Match the surrounding style. Comments explain WHY, not WHAT.
- By submitting a contribution you agree to the [CLA](CLA.md).

## License

GPL-3.0-only.
