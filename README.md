# ⚓ Belfast

Belfast is a private server reimplementation for the mobile game [Azur Lane](https://en.wikipedia.org/wiki/Azur_Lane), written in [Go](https://go.dev/) using [Iris](https://www.iris-go.com/) and [Gorm](https://gorm.io). It targets iOS and Android clients without requiring jailbreak or root access.

# Production instance

You can connect to a production instance of Belfast (EN region) by pointing `blhxusgate.yo-star.com` to `belfast-gateway-euw.molly.sh` (or `35.180.116.88`).

Once your account is created (onboarding is skipped!) you can then [register your account](https://belfast-euw.molly.sh) to manage your resources, ships, skins, name, ...

> [!WARNING]
> Traffic is logged and stored for debugging purposes. Log off from your main account for security reasons.

# 📊 Packet Progress

![Packet progress](https://cdn.molly.sh/belfast/implem.png)

# 🌟 Features

Belfast currently has:

- A low-level multiplexed TCP server, which allows multiple connections at once.
- The ability of following game updates, along with importing ship, items, ... data automatically (US version).
- A small API that allows you to quickly implement new game messages without head scratching.
- A great dissection tool in which every packet is stored, along with a `protobuf` -> `json` deserializer.
- A REST API with Swagger docs and admin endpoints for server tooling.
- A web UI in development: https://github.com/ggmolly/belfast-web.
- Config-driven packet response hydration for rapid prototyping.
- Packet progress tooling and webhook-based status updates.
- Runtime config toggles (maintenance mode, host/port overrides).

# ⚙️ Config

- `cmd/belfast` defaults to `server.toml` (game server config).
- `cmd/gateway` defaults to `gateway.toml` (gateway config).
- Region is configured via `[region].default` (`CN`, `EN`, `JP`, `KR`, `TW`) and defaults to `EN`.
- Gateway server list is defined in `[[servers]]`; set optional `name` per server for display text, and gateway probes each game server over the game protocol (`CS_10022` -> `SC_10023`) to resolve server state and load.
- To embed the git commit in status, build with `-ldflags "-X github.com/ggmolly/belfast/internal/buildinfo.Commit=$(git rev-parse --short HEAD)"`.

# 🐛 Reporting Issues

- Use the GitHub issue forms for bug reports and feature requests.
- Bug reports support region selection and optional debugging attachments:
  - `.pcap` captures
  - ADB logcat output from the ADB watcher (`-a` / `--adb`)
- Useful local command when collecting ADB logs:
  - `go run ./cmd/belfast -a`

# 🌠 State

Belfast reimplements all features from the game (except for background tasks).

# 🚀 Roadmap

1. Clean up the code
2. Reach 100% coverage on packet reimplementation
3. Implement game tracking (opt-in in server config for administrators)
4. Maintain more [belfast-web](https://github.com/ggmolly/belfast-web)
