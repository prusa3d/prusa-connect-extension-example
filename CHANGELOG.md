# Changelog

What an operator sees change when they upgrade this module, newest release
first. `version:` in `module.yaml` is the newest heading below.

Write the entry in the same change as the code. One bullet per behaviour a
person outside this repository can observe: a route, a setting, a screen, a
refusal. Name the route and the setting key, never a function or a file.

## 1.0.0 - 2026-11-17

### Added

- `GET /hello` answers one line and the principal the host authorized.
- The setting `message` sets the line that route answers with.
- Hello: a page of its own, with an entry in the sidebar, showing what `GET /hello` answers:
  the message and who asked.
