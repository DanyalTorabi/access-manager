# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-03-27

### Added

- Go HTTP service under `go/` with domain-scoped access control (users, groups, resources, access-type bits, permissions, authorization checks) and SQLite persistence
- Configuration via optional `CONFIG_PATH` YAML and environment variable overrides
- Optional static Bearer token for `/api/v1/*`; `/health` remains public
- Dockerfile, compose file, and Make targets for local container runs
- GitHub Actions workflow: Go test, vet, lint; Docker build and compose health smoke; container image publish to GHCR on pushes to `main`

### Security

- Logged startup warning when the listen address is not loopback and no API bearer token is configured

[Unreleased]: https://github.com/DanyalTorabi/access-manager/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/DanyalTorabi/access-manager/releases/tag/v0.1.0
