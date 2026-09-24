# Running Pi in a contagent container

A reusable, operator-oriented workflow for running [`pi`](https://github.com/earendil-works/pi)
inside [contagent](https://github.com/kanaka/contagent) so an agent can inspect, edit,
build, test, and run repository-local development commands in a container, while the
host resources visible to it stay narrow and intentional.

`grr-fyi` is the worked example; the pattern applies to any repository, and especially
to the mise-based Go/Elixir projects in this fleet.

* **Audience:** the human operator who sets this up and grants or revokes access.
  The agent running inside the container is *not* the audience.
* **Committed artifacts in this repository:** this document, `.contagent.yaml`
  (project override, safe-by-default), and the per-project checks in
  [Acceptance checklist](#59-acceptance-checklist).
* **Nothing here changes the application.** The deployed image is built from the
  repository `Dockerfile`; the contagent development image is a completely separate
  artifact. See [Relationship to the deployed image](#25-relationship-to-the-deployed-image).

---

## 1. Workflow contract and security matrix

### 1.1 What this workflow is, and what it is not

contagent "reduces exposure; it is not a hard security sandbox" (its own README). This
workflow inherits that limit, so read the boundary as:

* **Defends against:** an agent accidentally or carelessly touching host files, host
  credentials, or host services it has no reason to use; the whole class of "the agent
  ran a command in the wrong directory / with the wrong environment" incidents.
* **Does not defend against:** a determined malicious agent that has been granted a
  mount or a credential, the Docker daemon, the host kernel, or the host user account
  running the launcher. A container escape or a granted Docker socket is host root.
* **Trust anchor:** the contagent checkout, its launcher, and anything passed through
  `--docker-args` are trusted operator tooling. The launcher appends those arguments
  verbatim to `docker run`, so a config or shell function can silently widen the
  boundary. Review them as code.

Every decision below is therefore about making each *capability* visible, deliberate,
and revocable rather than about achieving isolation strength the platform cannot
provide.

### 1.2 Host prerequisites

| Prerequisite | Why | Verify |
|---|---|---|
| Docker Engine (Docker Desktop on macOS, or native `dockerd` on Linux) | Runs the development image | `docker version` |
| Node.js + npm on the host | contagent's launcher and `build-contagent` are Node scripts; `npm install` installs its single YAML dependency. Node is **not** an application runtime dependency of `grr-fyi` | `node --version` |
| `curl`, `jq`, `gzip` | Used by `build-contagent.yaml` version resolvers when a feature version is not pinned | `which curl jq gzip` |
| A contagent checkout, managed by the operator | Do **not** vendor or fork contagent into an application repository | `git -C <contagent> rev-parse HEAD` |
| Outbound network on first build | Resolves pinned/unpinned feature versions, pulls `node:26-slim`, installs Go / mise / psql / pi into the image | `docker pull node:26-slim` |
| Operator access to this repository (already cloned, branch chosen) | Cloning and branch selection are operator actions, never agent actions | `git -C <repo> status` |

### 1.3 Baseline feature set

Built into the development image and enabled at runtime:

| Feature | Provides | Notes |
|---|---|---|
| `pi` | The Pi coding agent (npm `@earendil-works/pi-coding-agent`) | Its embedded volume default (`~/.pi`) is **overridden** — see [1.4](#14-mount-table-baseline-after-overrides) and [Dedicated Pi profile](#3-dedicated-pi-profile-bootstrap) |
| `go` | Go toolchain at `/usr/local/go` (`go`, `gofmt` on `PATH`) | Image-provided fallback; `.tool-versions` + mise remain authoritative for the project version |
| `mise` | mise binary at `/usr/local/bin/mise` | Its three embedded volume defaults are **replaced** by container-local data dirs — see [1.4](#14-mount-table-baseline-after-overrides) |
| `psql` | `postgresql-client` | Its `~/.supabase` volume default is **dropped**; `grr-fyi` does not use Supabase |
| `build` | `build-essential`, `make`, `libssl-dev`, `pkg-config` | **Required by this repository**: `make` ships *only* with this feature (the `node:26-slim` base has no `make`), and the project workflow is Make-based. It also lets mise build source-only tools. See [2.2](#22-build-the-development-image) |
| `base` (required) | Debian trixie + git, ripgrep, fd, jq, yq, python3, tmux, tini, dbus-daemon, openssh-client, sudo, strace, socat | Base image `node:26-slim` |
| `runtime` (required) | `entrypoint.sh` identity mapping, MOTD, plus contagent state/cache volumes | Not optional: it is what maps host UID/GID and keeps `~/.cache/contagent` |

Deliberately **not** built into the image: `docker`, `gh`, `aws`, `keyring`,
`hostbridge`, `agent-browser`, and every non-Go language toolchain. In contagent, a
feature that is not built cannot be enabled by a runtime `--<feature>` flag (the
launcher rejects unknown feature names), so this is a build-time guarantee, not a
convention. Adding one later is a [credential/capability grant](#45-checklist-for-adding-a-credential-or-mount).

`build` is the one *tool* feature in the baseline beyond the plan's four
(`pi`, `go`, `psql`, `mise`). It is not a credential or host-access feature — it only
adds Debian packages to the image — and it is unavoidable here: the repository's
entry points are Make targets, and `make` is provided by `build` and by nothing else.
Recorded as a deviation in [Validation record](#validation-record). `grr-fyi` itself
needs no C compilation (pure-Go `modernc.org/sqlite`), so for a repository that does
not use `make` you can drop `--build` and shrink the image.

### 1.4 Mount table (baseline, after overrides)

`path` is the container destination; contagent mounts the same absolute path on the
host unless `source` says otherwise. `~` expands to the mapped host home. Modes:
**RW** read-write, **RO** read-only.

| # | Source (host) | Destination (container) | Mode | Secret-bearing | Controlled by | Purpose |
|---|---|---|---|---|---|---|
| 1 | launch directory (`$PWD`) | identical absolute path | RW | **repo data + gitignored local files**; see the `.envrc-priv` mask below | agent (writes land on host) | Primary working surface: edit, build, test |
| 2 | `~/.pi-contagent` | `~/.pi-contagent` | RW | **yes, once authenticated** (holds `auth.json` for the container profile only) | operator curates & authenticates; agent writes its own session state | Dedicated Pi profile: [section 3](#3-dedicated-pi-profile-bootstrap) |
| 3 | `/var/cache/contagent/mise` (host `~/.cache/contagent/mise`) | same container path | RW | no | agent-managed, host-persisted | Linux mise data/cache/state/shims, kept off the host profile |
| 4 | `/var/cache/contagent/go` (host `~/.cache/contagent/go`) | same container path | RW | no | agent-managed, host-persisted | `GOPATH`: module cache + `make install` tooling binaries |
| 5 | `~/.contagent` | `~/.contagent` | RW | no (contagent state) | runtime feature (required) | contagent state |
| 6 | `~/.local/state/contagent` | same | RW | low — contains `bash_history`, which can hold typed secrets | runtime feature (required) | per-launch state, shell history |
| 7 | `~/.cache/contagent` (host) | `/var/cache/contagent` | RW | no | runtime feature (required) | cache root; also symlinked to `$HOME/.cache`, so Go's build cache persists |
| 8 | image layers: `/usr/local/go`, `/usr/local/bin/mise`, psql, pi | — | RO by construction (image, not a mount) | no | operator (image rebuild) | toolchains |
| 9 | provider env vars declared in `.contagent.yaml` | — | RW (process env) | **potentially yes** | operator | Pi model/provider auth; currently **none declared** — see [3.4](#34-authentication) |

The launcher additionally mounts, at runtime, whatever the effective config enables,
and prints each mount as `[volumes] src -> tgt (ro)` before the container starts.

### 1.5 Explicitly denied by default

| Resource | How it is denied | Why |
|---|---|---|
| Broad host `$HOME` | Launcher never mounts `$HOME`; only the listed paths mount | The whole point of the allowlist model |
| Host `~/.pi` / `~/.pi/agent` (auth, sessions, run history, caches, model store, MCP state, extensions) | `pi` feature `volumes` replaced with `~/.pi-contagent`; the default mount would otherwise appear in `--show-config` | Plan non-goal: never mount the complete host profile |
| Host mise dirs `~/.config/mise`, `~/.local/share/mise`, `~/.local/state/mise` | `mise` feature `volumes: []` | Two reasons: the host toolchain cache holds **macOS (darwin-arm64) binaries** that cannot run in the Linux container, and writing Linux binaries into the host cache would corrupt host tool installs |
| `~/.supabase` | `psql` feature `volumes: []` | Not used by this project; an unused credential-shaped mount is pure risk |
| `.envrc-priv` contents | Masked by a nested RO zero-byte file mount over `${PWD}/.envrc-priv` (see [1.6](#16-the-envrc-priv-problem-and-its-mask)); the repository rule that no agent may read or write it still applies on the host | Contains Litestream/S3 credentials, lives *inside* the RW project mount |
| Docker daemon socket | `docker` feature is not built into the image | Socket access is effectively host root and would dissolve the boundary |
| `~/.config/gh`, `~/.aws`, `~/.config/gcloud`, `~/.docker`, `~/.ssh` | Their features are not built; none is referenced in any volume entry | No cloud or git credentials in the baseline |
| Host environment variables in general | The launcher passes **only** identity vars (`CONTAGENT_*`), `TERM`/`COLORTERM`, and feature-declared `environment` entries | `ANTHROPIC_API_KEY` and friends do not leak in implicitly |
| Arbitrary extra host paths | Not mountable without editing `.contagent.yaml` or `--docker-args` | Every mount is reviewable in one file |
| SSH agent socket (`SSH_AUTH_SOCK`) | **Not disabled by contagent** — the launcher auto-forwards it whenever the host variable points at a socket. Baseline policy: launch with it unset (`env -u SSH_AUTH_SOCK …`), which produces a visible `WARN: SSH agent not available; SSH auth forwarding disabled` | Forwarded keys are usable private-key credentials (git push, scp). Plan requires this to be an explicit, selected capability, so the default is off and the warning is the proof |

### 1.6 The `.envrc-priv` problem, and its mask

`.envrc-priv` is gitignored but physically present inside the project directory, and
the project directory is the one mount that must be read-write. Dropping it from the
mount list is not possible: it travels with mount #1.

The baseline masks it instead — a nested read-only bind of an empty file over the
credential path:

```yaml
- name: pi                      # any enabled feature can carry the extra mount
  volumes:
    - { path: ~/.pi-contagent }
    - { path: ${PWD}/.envrc-priv, source: ~/.cache/contagent/empty-file, read_only: true, file: true }
```

Because Docker applies mounts ordered by destination depth, the more specific file
mount wins over the project mount. Inside the container the path exists but contains
nothing.

All four properties below were observed during the acceptance run (see
[5.9](#59-acceptance-checklist)):

* The host source is auto-created as a **zero-byte file** by the launcher (it lies
  under the mapped `$HOME`), so this works on a fresh machine with no operator action.
* Content confidentiality is preserved while existence is not: in-container the file
  reads as **0 bytes**, so `[ -f .envrc-priv ] && source_env .envrc-priv` in `.envrc`
  succeeds harmlessly, and an in-container write is rejected with
  `Read-only file system`. Masking never modifies the host file — it is a bind mount,
  not a copy.
* The mask is scoped to `${PWD}/.envrc-priv` — the project root only. A repository
  with nested credential files needs its own mask entries. (`../.envrc`, also sourced by
  this project's `.envrc`, lives outside every mount and is simply absent in-container.)
* The mask is defense-in-depth against casual/accidental reads, not against an
  operator who relaunches the container without `.contagent.yaml` in scope — verified:
  in a repository lacking the override, the effective config reverts to mounting the
  host `~/.pi`. Always confirm the `[volumes]` lines
  ([5.7](#57-inspect-before-every-agent-session)).

### 1.7 Platform support expectations

| Platform | `host.docker.internal` | `--network host` | Notes |
|---|---|---|---|
| macOS, Docker Desktop (reference host for this doc) | Provided automatically. **Verified:** resolving to `192.168.65.254`, and reaching both a host service bound to `*` (HTTP 200) *and* one bound only to `127.0.0.1` (see the loopback caveat in [4.1](#41-default-route-hostdockerinternal)) | **Requires Docker Desktop ≥ 4.34 with *Enable host networking* in Settings → Network.** Incompatible with Enhanced Container Isolation. Published ports are ignored in host mode. **Crucially, host networking attaches the *Linux VM's* namespace, not macOS's** — so it does *not* improve reachability of macOS services, and is not a fix for anything `host.docker.internal` cannot already do | Bind-mount file I/O is slower than native; `go build` caches belong on `/var/cache/contagent` (they are) |
| Linux, native Docker Engine | **Not defined by default.** Add `--docker-args "--add-host=host.docker.internal:host-gateway"`, or use the gateway IP from `ip route` | Works, and shares the *real* host namespace, so `127.0.0.1` services become reachable — this is the one case where host networking is genuinely useful for loopback services | Native performance; UID/GID mapping is exact. With `host-gateway` only, a loopback-bound service is **not** reachable — bind address and host `pg_hba`/firewall matter |
| Validation performed for this document | Engine `24.0.2`, Docker Desktop, `linux/arm64` → host networking **unsupported**, so the fallback is documented but not exercised here | | Recorded as a partial validation; see [Validation record](#validation-record) |

### 1.8 Edge cases in mount creation

contagent creates missing mount sources itself, and the failure modes are easy to
misread. These are the behaviours to expect (all read from the launcher source):

* A missing source **under the mapped `$HOME` or under the launch directory** is
  created before `docker run`: directories via `mkdir -p`, `file: true` entries as a
  **zero-byte file**. It is created by the host user before the container starts, so
  ownership is correct — but a typo in a `path` silently produces an empty directory
  or empty file with no error.
* A missing source **anywhere else** is a hard failure: `ERROR: no source for mount <target>`.
  This is the desirable behaviour for credential files: a credential mount can never
  silently materialise outside `$HOME`/project scope.
* An **empty dedicated Pi profile** is not an error: Pi creates a fresh default profile
  there. The container therefore boots happily with none of the operator's curated
  settings — which is why the [bootstrap verification step](#33-sync-procedure-operator-run-allowlist-only) checks
  for a marker setting rather than trusting that the copy happened.
* A `file: true` entry pointing at an existing **directory** (or vice versa) is a type
  mismatch that Docker reports at `docker run` time, after the launcher has already
  printed its `[volumes]` lines. Check `--show-config` and `--dry-run`-style review
  before assuming a launch succeeded.
* Never give a credential-bearing path `file: true` unless the host file really is a
  file: an auto-created zero-byte `.envrc-priv` inside a project is exactly how an
  empty-but-present credential path is born. The mask in [1.6](#16-the-envrc-priv-problem-and-its-mask)
  is the one place where this behaviour is used deliberately.

### 1.9 Actions reserved for the operator

Pi inside the container must not automate these; they either widen the boundary or
mutate host/git state that a human owns:

1. Cloning a repository, and choosing where it lives or which branch/worktree to use.
2. First-time authentication of the dedicated Pi profile (or approving any
   credential-bearing file into it).
3. Copying host Pi configuration into the dedicated profile.
4. Adding a service credential, a new mount, or a new environment variable.
5. Enabling `--network host`, `--docker-args`, or any extra capability.
6. Building or rebuilding the image (needs host Docker/npm and network).
7. `git push`, publishing a container image, pushing a Helm chart, deploying.
8. Destructive cleanup: removing volumes, `docker system prune`, deleting profile data.

Everything inside the container that does **not** touch the above — editing files,
`make test`, `make check`, `go build`, `mise install`, running `psql` against a
granted endpoint — is normal agent work.

### 1.10 Matrix self-check

The review required by the plan, against contagent's own feature definitions
(`build-contagent.yaml`) as of revision [`53e4a99`](https://github.com/kanaka/contagent/commit/53e4a99137bb0e9ec4089f3dcf3b7e79336ddfa8)
(release/commit recorded rather than floating; re-inspect after any upgrade):

| Embedded feature default | Baseline disposition | Intentional? |
|---|---|---|
| `pi` → `~/.pi` | replaced by `~/.pi-contagent` | yes — plan non-goal |
| `mise` → `~/.config/mise`, `~/.local/share/mise`, `~/.local/state/mise` | replaced by `volumes: []` + container-local data dirs | yes — cross-arch binaries, host cache pollution |
| `psql` → `~/.supabase` | replaced by `volumes: []` | yes — unused |
| `runtime` → `~/.contagent`, `~/.local/state/contagent`, `/var/cache/contagent`←`~/.cache/contagent` | kept | yes — required feature, state/cache only, no credentials |
| `base`, `go` | no volumes | yes — nothing to review |
| `build` | no volumes; adds Debian packages (`make`, compiler) to the image | yes — `make` is required to run this repository's own workflow |
| launcher → `SSH_AUTH_SOCK` auto-forward | disabled by launching with the variable unset | yes — see [1.5](#1.5-explicitly-denied-by-default) |
| launcher → project mount | kept RW, with `.envrc-priv` masked | yes |

---

## 2. Image and project configuration convention

### 2.1 Obtain a known contagent revision (operator)

contagent stays an external, operator-managed tool: this repository never vendors,
forks, or reimplements it. Clone it wherever you keep your tools, then pin your
attention to one revision:

```bash
git clone https://github.com/kanaka/contagent ~/repos/contagent   # operator chooses the path
cd ~/repos/contagent && git fetch --all --tags
git log -1 --format='%H %ci %s'      # record this revision + date
npm install                          # YAML dependency for the launcher/build script
```

`npm install` is mandatory: the launcher `require`s the `yaml` package, and it is not
resolvable globally. It also pulls the hostbridge dependencies (`ws`, `glimpseui`);
their postinstall scripts are irrelevant unless hostbridge is later enabled.

**Pin policy:** the revision this workflow was validated against is recorded in
[Validation record](#validation-record) (Task 1's matrix self-check cites it too).
After any contagent upgrade, re-run `--show-config` and re-read the diff: feature
volume defaults are the part of contagent most likely to change under you — the
baseline in `.contagent.yaml` survives that drift precisely because it *replaces*
volume lists rather than merely adding to them.

### 2.2 Build the development image

```bash
cd ~/repos/contagent
PI_VERSION=0.87.1 GO_VERSION=1.27.1 MISE_VERSION=2026.9.13 BUILD_ESSENTIAL_VERSION=12.12 \
  ./build-contagent --pi --go --mise --psql --build
```

| Pin | Set via | Purpose |
|---|---|---|
| `PI_VERSION` | env var consumed by `build-contagent` (npm `latest` is the default; it is resolved *at build time* and is **not** reproducible) | Pi version the agent runs |
| `GO_VERSION` | env var, same mechanism | image fallback Go (see [2.4](#24-version-reconciliation-and-the-known-discrepancy)) |
| `MISE_VERSION` | env var (GitHub release tag) | mise, which stays authoritative for project tool versions |
| `BUILD_ESSENTIAL_VERSION` | env var; `12.12` is the Debian trixie version resolved at validation time | provides `make` (see below) + C toolchain |
| `psql` | intentionally **unpinned** — the part installs `postgresql-client` from the Debian trixie index, which is already frozen by the base image digest (resolved to 17.11 at validation time) | PostgreSQL client |

Notes on the mechanism (from `build-contagent` source):

* Omitting the version env vars makes the build resolve `latest` live via curl/jq —
  fine for exploration, unacceptable for a reproducible example, hence the explicit
  pins above.
* The build writes `.Dockerfile.generated`, `.contagent-default.yaml.generated`, and
  `.contagent-motd.generated` **into the contagent checkout** (gitignored there).
  Never copy them into an application repository.
* Two tags are produced: a deterministic voom tag
  (`contagent:<YYYYMMDD_HHMMSS>-g<sha>`) and `contagent:latest`. Launch by voom tag
  when you want a specific build; `latest` is the launcher default.
* **`--build` is required for a Make-based repository.** The `node:26-slim` base has
  no `make`; `make` is installed only by the `build` Dockerfile part. Without it,
  `./contagent -- make test` fails with `env: 'make': No such file or directory`
  (verified). The `build` part adds only Debian packages — no mounts, no credentials —
  so it does not weaken the boundary in [1.5](#15-explicitly-denied-by-default).
* `--pi --go --mise --psql --build` is the full baseline. `base` and `runtime` are
  `required` and always included. `docker`, `gh`, `aws`, `hostbridge`, `keyring`, and
  the other language toolchains are **not built in**, and a runtime `--<feature>` flag
  for an unbuilt feature is rejected by the launcher — so "accidentally enabled" is not
  a failure mode here.

### 2.3 Launch

From the repository root, with the SSH agent deliberately unexported
([why](#15-explicitly-denied-by-default)):

```bash
cd /path/to/grr-fyi
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent --show-config        # inspect first
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- pi               # interactive Pi
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- make test         # one-shot
```

**PATH caveat, verified:** the injected `PATH` (mise shims + `GOBIN`) *does* apply to
one-shot commands and to the non-login shells Pi spawns for its `bash` tool, but
`bash -l` (the launcher's default interactive command) sources Debian's `/etc/profile`,
which reassigns `PATH` and drops the injected entries. In an interactive session, run
`eval "$(mise env)"` (or invoke `mise exec --`) after the shell opens; `go`, `mise`,
`psql`, and `pi` themselves still resolve because they live in `/usr/local/bin`.

`--update-config --<feature>` rewrites the config file to contain **only the diff**
from the image's embedded defaults (verified) — use it to keep the file minimal after
an image rebuild rather than hand-editing stale volumes.

The launcher reads `.contagent.yaml` **from the launch directory** by default (or
`CONTAGENT_CONFIG`, or `-c`). That is why this file is committed at the repository
root: it makes the reviewed baseline fail-safe for anyone who launches contagent in
this checkout, instead of depending on an operator-specific environment variable that,
if forgotten, silently reverts to the embedded default of mounting the host's entire
`~/.pi`. For a repository that must not carry the file, set
`CONTAGENT_CONFIG=~/.config/contagent/<repo>.yaml` instead — but then every launch
diagnostic (`--show-config`) matters more, because the safety lives outside the repo.

**What may be committed** (this repository's choice, and the reusable rule):

* `.contagent.yaml` — deviations only, portable `~`/`${PWD}` paths, zero credentials.
* `docs/contagent-pi-workflow.md` — this document.

**What must never be committed:** real secret values, operator home paths, machine
specific GIDs, or any generated contagent file.

### 2.4 Version reconciliation and the known discrepancy

Three different Go versions legitimately coexist; the workflow surfaces rather than
hides them:

| Source | Version | Role |
|---|---|---|
| `GO_VERSION` (image) | `1.27.1` at build time | fallback `go` on `PATH` (`/usr/local/go`) before `mise install` has run |
| `.tool-versions` | `golang 1.25.6` | what mise installs when the project's toolchain is activated (`mise install`) |
| `go.mod` | `go 1.25.11` | the language/toolchain level the *module* requires |

Consequences to expect and record during validation:

* Because `go.mod` requires 1.25.11, a Go 1.25.6 binary will not build the module
  directly; with `GOTOOLCHAIN=auto` (the default, and intentionally not overridden)
  the toolchain auto-provisions 1.25.11 on first build. That download needs network
  and lands in the persisted `GOMODCACHE` (`GOPATH` → `/var/cache/contagent/go`), so
  it happens once per machine, not once per launch.
  **Observed:** after `mise install`, bare `go version` in the repository printed
  `go: downloading go1.25.11` then `go version go1.25.11 linux/arm64` — the
  discrepancy resolving itself, visibly, exactly as intended.
* The same mechanism applies to the `make install` tools, which are newer than the
  pinned language level: **observed** `go: golang.org/x/tools@v0.50.0 requires go >=
  1.26.0; switching to go1.26.8` for goimports/revive/staticcheck/govulncheck/gopls.
* Before `mise install` has ever run, the image's `GO_VERSION` (1.27.1) satisfies
  `go.mod` directly and `make test` passes with no toolchain download at all.
* **mise's Go backend exports `GOBIN`** pointing inside its own install directory
  (`<MISE_DATA_DIR>/installs/go/<ver>/bin`), which is *not* on the container `PATH`.
  Without intervention `make install` appears to succeed while `make check` then fails
  with `goimports: command not found` (both verified in-container). The baseline sets
  `MISE_GO_SET_GOBIN: "false"` so `go install` falls back to `$GOPATH/bin`, which *is*
  on `PATH`; after that, `make check` passes cleanly.
* The discrepancy is pre-existing on the host too; this workflow does not resolve it.
  The right fix (aligning `.tool-versions`, or lowering the `go.mod` directive) is an
  application-repository decision outside this plan's scope, deliberately left visible
  here.
* `.tool-versions` also pins `lua 5.1.4`, whose mise backend compiles from source and
  needs `libreadline-dev` + `libncurses-dev` — packages **not** provided even by the
  `build` feature (it installs build-essential, make, libssl-dev, pkg-config). Verified
  in-container: `fatal error: readline/readline.h: No such file or directory` →
  `mise ✗ lua@5.1.4 failed: Failed to build Lua`, while `golang@1.25.6` installed fine.
  `make install`'s `|| true` absorbs it, and lua is not used by any Make target, so the
  baseline deliberately does *not* add those packages. Either drop the lua pin for
  container use, or extend the image with a project-specific Dockerfile part.
* `mise exec --` re-runs the whole install step, so on this repository it re-attempts
  (and re-fails) the lua build; prefer bare commands with the injected `PATH`.

### 2.5 Relationship to the deployed image

The repository `Dockerfile` (multi-stage Go build for the `grr-fyi` application, used
by `make docker-push` and the Helm chart) is **not** the contagent image and must not
be repurposed for it. The contagent image is an operator-side development host built
from `build-contagent` + `Dockerfile-parts/` and is never published from this
repository. Nothing in this workflow touches `Dockerfile`, `Makefile`,
`grr-fyi-chart/`, migrations, or application code.

---

## 3. Dedicated Pi profile bootstrap

The `pi` feature's embedded default mounts the **whole host `~/.pi`**, which on a
worked host contains provider credentials, every session transcript, run history, the
model store, MCP caches, and installed extensions. That is precisely what this workflow
must not hand to a containerised agent. The baseline instead mounts a **dedicated
profile** (`~/.pi-contagent`, with `PI_CODING_AGENT_DIR=~/.pi-contagent/agent`) whose
contents are chosen file by file by the operator.

### 3.1 What the dedicated profile contains

Because `PI_CODING_AGENT_DIR` is the *only* agent directory Pi reads, everything
normally under `~/.pi/agent` — `auth.json`, `settings.json`, `sessions/`, `trust.json`,
`extensions/`, caches — resolves inside `~/.pi-contagent/agent` in the container. Nothing
is inherited from the host profile, and no host path is reachable.

| Category | Examples (host `~/.pi/agent`) | Copy into dedicated profile? | Why |
|---|---|---|---|
| Declarative settings | `settings.json`, `keybindings.json` | **Yes, after review** | Plain preferences. Review first: this host's `settings.json` also carries `packages` (npm specs) and `subagents.modelScope`, which steer what gets *installed* and which models are allowed |
| Declarative content | `instructions/*.md`, `agents/*.md`, `prompts/`, `prompt_sources/`, `workflows/`, `subagent-workflows/`, `docs/` | **Yes, after review** | Markdown/config that shapes behaviour; review for host-specific paths and absolute home paths that are meaningless in-container |
| Model/provider definitions | `models.json` | **Review carefully — often no** | Shape looks declarative, but custom providers embed `apiKey`/`headers`. Check each value: this host's entries are env-var *references* (`$VLLM…`), not literals, which is the safe pattern; a pasted literal key is a secret copy |
| MCP configuration | `mcp.json`, `web-search.json` | **Generally no** | Servers are launched commands (executable) and their `env`/`headers` blocks commonly hold API keys. Re-declare only the servers you actually want in-container, per [4.5](#45-checklist-for-adding-a-credential-or-mount) |
| Executable extensions | `extensions/*.ts`, `extensions/*/`, `npm/` (`node_modules`) | **Only with explicit code review** | An extension runs with Pi's full permissions inside the container. Many host extensions are also host-specific (permission gates, keychain helpers, native browser/LSP binaries) and will simply fail or mislead in Linux |
| Skills | `~/.agents/skills/` on this host | **Copy the markdown; skip the scripts unless reviewed** | Skill bodies are instructions, but several ship helper scripts (Python/shell) that are code and some reference host-only tools (Obsidian vault, `hslink`, macOS paths). `~/.agents/skills` is a host path outside every mount, so copy the wanted skills into `<agent-dir>/skills/`, which Pi also discovers |
| Credentials | `auth.json`, any literal key | **Never by copy** | Authenticate the dedicated profile separately — [3.4](#34-authentication) |
| State / history / cache | `sessions/`, `run-history.jsonl`, `missions/`, `models-store.json`, `mcp-cache.json`, `mcp-npx-cache.json`, `web-search-cache/` | **Never** | Contains past prompts, tool output, and session transcripts. Copying it leaks host history into the container and defeats the point of a separate profile |

### 3.2 Why the host layout complicates a naive copy

On this host several profile entries are **symlinks into a dotfiles repository**
(`settings.json`, `keybindings.json`, `models.json`, `mcp.json`, `prompts`,
`subagent-workflows`, `web-search.json` → `~/.homesick/repos/dotfiles/…`). Two
consequences:

* `cp -a` (or `rsync -a`) would reproduce *dangling* symlinks in the dedicated profile,
  because the dotfiles repo is not mounted in the container. Dereference instead
  (`cp -L`/`cp --dereference`, or `rsync -aL`).
* Dereferencing means the copy captures a snapshot of the dotfiles, not the live
  symlink — so the dedicated profile drifts and needs the explicit re-sync in
  [3.5](#35-keeping-the-dedicated-profile-current).

### 3.3 Sync procedure (operator-run, allowlist only)

Run on the host. It creates the profile if the launcher has not already done so, copies
**only allowlisted paths** with symlinks resolved, and prints what it did.

```bash
SRC="$HOME/.pi/agent"
DST="$HOME/.pi-contagent/agent"
mkdir -p "$DST"

# Declarative, review-then-copy. Trim `packages`/`subagents` from settings.json first
# if you do not want those installs/limits inside the container.
for f in settings.json keybindings.json models.json; do
  [ -e "$SRC/$f" ] && cp -L "$SRC/$f" "$DST/$f" && echo "copied $f"
done

# Declarative content directories (Markdown/config).
for d in instructions agents prompts prompt_sources workflows subagent-workflows docs; do
  [ -d "$SRC/$d" ] && rsync -aL --delete "$SRC/$d/" "$DST/$d/" && echo "synced $d/"
done

# Skills: markdown now, scripts only if you have read them.
[ -d "$HOME/.agents/skills" ] && rsync -aL --exclude 'scripts/' \
  "$HOME/.agents/skills/" "$DST/skills/" && echo "synced skills (bodies only)"

# Anything executable: review each item explicitly, then copy by name.
#   cp -L "$SRC/extensions/interesting.ts" "$DST/extensions/"
#   cd "$DST" && pi -p 'ok' # confirm the extension behaves in Linux
```

Do **not** replace the loop bodies with a blanket `rsync -aL ~/.pi/agent/ …` — that
reintroduces `auth.json`, `sessions/`, and the caches, i.e. exactly the host state this
section exists to exclude.

**Verification that a copied file is actually live in the container.** An empty
`~/.pi-contagent` is not an error — Pi silently creates a fresh default profile there
(verified: it wrote `auth.json`, `models-store.json`, `sessions/` on first launch).
Absence of an error therefore proves nothing. After syncing, confirm from inside the
container that a copied artefact is loaded — for example run
`pi -p 'hi'` and check the reported model/provider matches your copied `settings.json`,
or load a reviewed extension that prints at startup (a startup `console.error` probe
was observed firing from `~/.pi-contagent/agent/extensions/` during validation, then
removed).

### 3.4 Authentication

The dedicated profile starts unauthenticated. Verified behaviour: `pi -p 'hi'` inside
the container responds `No API key found for the selected model. Use /login to log into
a provider via OAuth or API key.` — a clear stop, not a fallback to host credentials.

Authenticate it as a deliberate operator action, in one of two ways:

1. **Interactive login (default).** Start `pi` in the container and run `/login`. The
   credential is written to `~/.pi-contagent/agent/auth.json`, which exists only in the
   dedicated profile.
2. **Per-run environment variable.** Declare the key in `.contagent.yaml` under the
   relevant feature's `environment` map (e.g. `ANTHROPIC_API_KEY`). The launcher passes
   only variables named in the config; nothing leaks implicitly. Prefer a
   **low-privilege, revocable, usage-capped** key: variables declared this way are
   visible to every process in the container, including model-generated commands, and
   appear in `--show-config`.

Never copy the host `auth.json`, and never point `PI_CODING_AGENT_DIR` back at the host
profile to "reuse" a login. If the containerised profile needs a different trust level
than the host, that difference is the point.

### 3.5 Keeping the dedicated profile current

Re-run [3.3](#33-sync-procedure-operator-run-allowlist-only) after deliberately changing
host settings. Because it is an allowlist copy, the dedicated profile only ever contains
what you chose to put there, and re-running it is how a host change is *reviewed* rather
than silently propagated.

Deliberately **not** supported: automatic sync, `--delete` across the whole profile, or
file-watchers. A hook that mirrors the host profile into the container would recreate
the exposure this section removes.

### 3.6 Reset and revocation

| Goal | Action (host) | Effect |
|---|---|---|
| Log the container out, keep settings | `rm ~/.pi-contagent/agent/auth.json` | Container must `/login` again; host profile untouched |
| Revoke a copied extension/skill | delete that file/dir under `~/.pi-contagent/agent/` | Removes executable behaviour without touching credentials |
| Full reset | `rm -rf ~/.pi-contagent` | Destroys container credentials, settings, sessions, and trust decisions. The next launch recreates an empty, unauthenticated profile |
| Revoke the mount itself | remove the `pi` feature volumes from `.contagent.yaml` (or `--no-pi`) | Container loses all Pi profile persistence |

`~/.pi-contagent` contains **no** host credential material, so it is safe to back up or
share; still treat it as secret-bearing once [3.4](#34-authentication) has been done.

### 3.7 Edge cases

* **Missing dedicated profile** → the launcher creates `~/.pi-contagent` before
  `docker run` (sources under `$HOME` are auto-created), so launch succeeds with an
  empty profile rather than failing. This is convenience at the cost of a loud signal:
  a typo'd profile path creates the wrong directory silently. Verify the mount list
  (`[volumes]` lines) at first launch in a new repo.
* **Profile path differs per OS** — `~` expands to the *mapped* host home, so the same
  `.contagent.yaml` works on macOS and Linux for the same username, and does not travel
  between different usernames.
* **Host profile is never a fallback**: there is no code path that mounts `~/.pi` while
  `pi` volumes are overridden. If `--show-config` ever shows `~/.pi`, the override was
  lost (usually an image rebuild changed the feature name) — treat as a boundary
  regression and fix before launching.
* **Stale copied state**: because copies are dereferenced snapshots, remove files here
  when you remove the corresponding host feature, or the container keeps a obsolete
  settings file.

---

## 4. Host services and future credential extensions

### 4.1 Default route: `host.docker.internal`

Ordinary Docker networking is the baseline. The container reaches host services through
`host.docker.internal`, which Docker Desktop defines automatically (verified: it
resolves to `192.168.65.254` here). On native Linux it is undefined until you add it:

```bash
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent \
  --docker-args "--add-host=host.docker.internal:host-gateway" -- make test
```

**Verified reachability results on the reference host (macOS / Docker Desktop, engine 24.0.2):**

| Host service | Host bind | From container via `host.docker.internal` |
|---|---|---|
| nginx (safe non-sensitive probe) | `*:8080` | HTTP `200` ✅ |
| Postgres.app | `127.0.0.1:5432` **only** | **Connected** ✅ — `current_user = postgres`, `inet_client_addr() = 127.0.0.1` |
| nothing listening | port 59999 | clean `Connection refused` ✅ (the documented failure mode) |

**The loopback caveat — read this before trusting "it only listens on localhost".**
On Docker Desktop the `host.docker.internal` path is served by a proxy that runs *on the
host* and dials the service from the host's own loopback. So a macOS service bound only
to `127.0.0.1` is still reachable from the container, and it appears to the service as a
local connection — which means any host auth rule of the form "trust anything from
127.0.0.1" applies to the agent too. The Postgres.app result above is exactly that case:
the container connected as the **superuser with no password**. Do not treat loopback
binding, or a `trust`/`md5`-free local `pg_hba` rule, as a boundary against the
container. (This differs from native Linux, where a loopback-bound service is not
reachable through `host-gateway` at all.)

Practical consequences for this repository:

* `grr-fyi` itself uses SQLite + Litestream, so no host database is required for
  `make test` / `make check`; the Postgres client is in the image for the cases where a
  developer points a tool at one.
* When you do expose a host database, create a **dedicated low-privilege role scoped to
  a development database** and connect with it, rather than riding on a loopback-trusted
  superuser. That is also what makes the grant revocable.

### 4.2 Specifying service endpoints inside the container

The launcher passes **no host environment variables** except identity, `TERM`/
`COLORTERM`, and what a feature's `environment` map declares. Verified in-container:
`SITE_URL`, `DB_PATH`, `REPLICATION_ENABLED`, `LITESTREAM_REPLICA_URL` are all unset, and
`direnv` is **not installed in the image**, so `.envrc` is inert there — it is not
silently sourced, and `.envrc-priv` is both masked ([1.6](#16-the-envrc-priv-problem-and-its-mask))
and unreachable.

So endpoints are declared explicitly, in one of two places:

```yaml
# .contagent.yaml — persistent, reviewable in `--show-config` and in git
- name: runtime
  environment:
    SITE_URL: http://host.docker.internal:30666/
    PGHOST: host.docker.internal
    PGPORT: "5432"
    PGUSER: grr_dev_readonly        # dedicated low-privilege role, not `postgres`
```

```bash
# or per-invocation, for something that should not persist
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent \
  --docker-args "-e SITE_URL=http://host.docker.internal:30666/" -- make run
```

Use host-side service ports that are actually published, and remember that a service
listening only inside another container needs that container's network, not the host's.

For MCP servers, point the client at the host URL explicitly
(`http://host.docker.internal:<port>/…`) rather than assuming a socket or `localhost`
will follow it into the container. One caution for `localhost`: inside the container
`localhost` is the container, so a config copied verbatim from the host will point at
nothing.

### 4.3 Deciding whether a service is reachable at all

Check the host bind first, then probe from the container:

```bash
# host: what is bound where
lsof -nP -iTCP -sTCP:LISTEN | rg '5432|8080'        # 127.0.0.1 vs * vs a specific interface

# container: name resolution, then a real request
getent hosts host.docker.internal
curl -sv -m 5 -o /dev/null http://host.docker.internal:8080/
psql "host=host.docker.internal port=5432 dbname=dev user=grr_dev_readonly connect_timeout=5" -c 'select 1'
```

| Symptom | Likely cause | Action |
|---|---|---|
| name does not resolve | native Linux without `host-gateway` | add the `--add-host` docker-arg, or use the bridge gateway IP from `ip route` |
| resolves, `Connection refused` | nothing bound on that port/interface, or the service binds only to an interface the route cannot reach | fix the service's bind address, or publish the port from whatever does the listening |
| resolves, hangs then times out | host firewall, or (Docker Desktop) a service that never answers the probe protocol | allow the Docker subnet, or probe with a protocol the service speaks |
| auth error from the service | reachable, but credentials/`pg_hba` reject this user | provision a dedicated role — do not weaken host auth to "make it work" |
| works for `postgres`/admin with no password | loopback `trust` is being inherited — see [4.1](#41-default-route-hostdockerinternal) | treat as a finding: add an explicit low-privilege role |

### 4.4 Opt-in fallback: `--network host`

Only when ordinary networking genuinely cannot reach the service.

```bash
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent \
  --docker-args "--network host" -- make test
```

Prerequisites and consequences, all of which apply to this repository's macOS host:

* **Linux:** works as expected, sharing the *real* host network namespace — which also
  means the container can reach every host-local service, including loopback-only ones
  and anything guarded solely by "not exposed on an interface".
* **Docker Desktop:** requires **≥ 4.34** with **Settings → Network → Enable host
  networking**, and is **unavailable when Enhanced Container Isolation is on**. The
  reference host here runs engine `24.0.2`, i.e. pre-4.34 support, so this path was
  **documented but not exercised**; the acceptance checklist marks it as unverified.
* **Docker Desktop caveat:** host networking joins the Linux **VM's** namespace, not
  macOS's. It therefore does *not* reach macOS loopback services any better than
  `host.docker.internal` already does — on this platform it is almost never the right
  answer.
* **`--publish` is ignored in host mode**, so do not combine them expecting port maps.
* Trust impact: it removes network isolation between the container and the host's
  listeners, and lets the agent bind privileged host ports.

**Rollback** is by removing the argument (or dropping it from `CONTAGENT_DOCKER_ARGS`
/ the wrapper), then confirming the boundary is back:

```bash
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- bash -c 'hostname; ip -br addr'
# compare against the host's own hostname/interfaces: they must differ
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- bash -c 'curl -m 5 -sS http://127.0.0.1:8080/ >/dev/null && echo STILL-REACHABLE || echo boundary-restored'
```

Validate the fallback **service by service**: if one MCP endpoint needed host mode, do
not leave host mode on for every subsequent launch.

### 4.5 Checklist for adding a credential or mount

Any new capability — a token, a cloud profile, an MCP server, a directory — goes through
this table *before* it is added, and the completed table is the commit message for the
change (or the note in your operator log):

| Question | Satisfy it by |
|---|---|
| What concrete workflow needs this? | a command that fails without it, not "might be useful" |
| Can the same outcome be reached with a dedicated low-privilege account or a host-side proxy? | prefer this over handing out the real credential |
| Narrowest form: single file < directory; one env var < inherited environment; read-only < writable | choose the top of that ladder that works |
| Exact source and destination paths, and `read_only`? | write them into `.contagent.yaml` so the grant is reviewable in one file |
| Is it secret-bearing? | mark it in the comment above the entry |
| Who can see it? (`--show-config`, `docker inspect`, `/proc/1/environ`, any command the agent runs) | remember env vars are visible to model-generated commands; a file at least needs a read |
| Lifetime? | per-invocation flag > project config > image rebuild |
| How is it revoked? | the exact `rm`/edit lines, verified by a relaunch that now fails |
| Validation? | the positive probe and the negative probe after revocation |

Preference order, most preferred first: **host-side proxy or low-privilege service
account → single read-only file → narrowly scoped environment variable → `.envrc`-based
loading (only when its sourcing and secret files are explicitly understood) → contagent
feature/config override**. In all cases the grant must be visible in `--show-config` or
in the `[volumes]` line, so that a later reader can see what the boundary allowed.

Mechanisms that are **documented but deliberately not enabled** — each is a complete,
working example; adding any one of them is a reviewed decision, not a configuration
tidy-up:

```yaml
# (a) A single read-only credential file, auto-created empty if the source is missing —
#     so verify non-emptiness before trusting it (`[ -s ~/.config/svc/token ]`).
- name: runtime
  volumes:
    - { path: ~/.config/svc/token, source: ~/secrets/svc-token, read_only: true, file: true }

# (b) A narrowly scoped environment variable (visible to every process in the container).
- name: psql
  environment:
    PGPASSWORD: "not-in-git-use-a-dedicated-role"

# (c) A two-step capability grant, decided partly at launch. contagent only embeds
#     features that were selected at build time, so a feature absent from the image
#     CANNOT be enabled by a runtime flag (the launcher rejects unknown names). Granting
#     `docker` therefore takes both steps, and stays visible in --show-config:
#       ./build-contagent --pi --go --mise --psql --build --docker   # step 1: put it in the image
#       env -u SSH_AUTH_SOCK ~/repos/contagent/contagent --docker -- make test   # step 2: enable it
#     (`docker` carries `default: false`, so step 2 is required per run, and the socket
#     mount only appears when it is given. This remains prohibited in the baseline —
#     see 1.5 — it is shown here only as the shape a reviewed grant takes.)
#     A built feature can equally be switched OFF for one run:  --no-psql
```

And the standing prohibitions:

* **Never** source, mount, or copy `.envrc-priv`. It stays host-only, and the mask in
  [1.6](#16-the-envrc-priv-problem-and-its-mask) makes the repository's "agents must not
  read `.envrc-priv`" rule structurally true in-container rather than merely instructed.
* **Never** mount `~/.ssh`, `~/.aws`, `~/.config/gh`, `~/.config/gcloud`, `~/.docker`,
  or the Docker socket into the baseline.
* **Never** add `--docker-args` that widen access without the same checklist — those
  arguments go verbatim to `docker run` and are invisible to `--show-config`, which is
  precisely why they are the riskiest place to put a grant.

### 4.6 SSH agent forwarding, final policy

The launcher forwards `SSH_AUTH_SOCK` **automatically** whenever the host variable points
at a live socket — there is no contagent flag to disable it. Verified both directions on
this host: launching normally mounts `/private/tmp/com.apple.launchd.*/Listeners` and
exports `SSH_AUTH_SOCK` in the container; launching with `env -u SSH_AUTH_SOCK` produces
`WARN: SSH agent not available; SSH auth forwarding disabled` and forwards nothing.

Baseline policy therefore: **always launch with `env -u SSH_AUTH_SOCK`** (as every
command in this document does). The consequence is that git operations needing keys fail
inside the container — which is intended, since pushing is an operator action
([1.9](#19-actions-reserved-for-the-operator)). Forwarding is a *selected* capability:
to enable it, drop the `env -u`, and accept that every key currently loaded in the host
agent (and whatever they authenticate to) is usable by the agent inside the container.
Load only the keys you intend to expose, `ssh-add -D` afterwards, and revoke by relaunching
without forwarding — the mount exists only for the lifetime of the launch.

---

## 5. Operator lifecycle, troubleshooting, and acceptance

### 5.1 First-time setup (operator, one machine)

```bash
# 1. Prerequisites
docker version && node --version && which curl jq gzip

# 2. contagent checkout + its host deps + record the revision
git clone https://github.com/kanaka/contagent ~/repos/contagent
cd ~/repos/contagent && npm install && git log -1 --format='%H %ci %s'

# 3. Build the image with explicit pins (section 2.2)
PI_VERSION=0.87.1 GO_VERSION=1.27.1 MISE_VERSION=2026.9.13 BUILD_ESSENTIAL_VERSION=12.12 \
  ./build-contagent --pi --go --mise --psql --build

# 4. Give the repository its baseline config (section 2.3) — grr-fyi already carries it

# 5. Curate the dedicated Pi profile (section 3.3), then authenticate it (3.4)

# 6. Inspect BEFORE the first agent session (section 5.7)
cd /path/to/grr-fyi && env -u SSH_AUTH_SOCK ~/repos/contagent/contagent --show-config

# 7. First launch
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- pi
```

On first Pi launch inside the container you will be asked to trust the project (the
trust decision is written to the **dedicated** profile's `trust.json`, never the host
profile), and then to authenticate if you have not done step 5's login.

### 5.2 Everyday use

| Task | Command (from the repository root) |
|---|---|
| Interactive Pi | `env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- pi` |
| One-shot Pi prompt | `env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- pi -p 'summarise the failing tests'` |
| Run a Make target | `env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- make test` |
| Plain shell (one-shot) | `env -u SSH_AUTH_SOCK ~/repos/contagent/contagent -- bash -c 'go version'` |
| Interactive shell | `env -u SSH_AUTH_SOCK ~/repos/contagent/contagent` (default command is `bash -l`; run `eval "$(mise env)"` for shims — see [2.3](#23-launch)) |
| Inspect effective config | `env -u SSH_AUTH_SOCK ~/repos/contagent/contagent --show-config` |
| Exec into a running session | `docker exec -it <container> /entrypoint.sh bash -lc 'id && whoami'` |
| Pin a specific image | `CONTAGENT_IMAGE=contagent:<voom-tag> env -u SSH_AUTH_SOCK … -- pi` |

Containers run with `--rm`, so exiting the shell removes them; nothing to tidy per run.

### 5.3 Image rebuild and upgrade

1. Bump the pins you intend to move (`PI_VERSION`, `GO_VERSION`, `MISE_VERSION`,
   `BUILD_ESSENTIAL_VERSION`) and re-run the build command from [2.2](#22-build-the-development-image).
2. Re-run `--show-config` and **diff it against the previous output**: contagent feature
   volume defaults are what change between revisions, and the baseline survives drift
   only where it replaces volume lists explicitly.
3. Re-read [1.10](#110-matrix-self-check) against the new `build-contagent.yaml`.
4. Record the contagent revision and the resolved component versions alongside the
   change (`docker inspect --format '{{json .Config.Labels}}' contagent:latest | jq` shows
   `io.contagent.component.<feature>.version` and `.mounts` for every built feature).
5. Discard the old build once the new one is validated: `docker rmi contagent:<old-voom-tag>`.

### 5.4 Cleanup

| What | Command | Notes |
|---|---|---|
| Stopped containers | none needed | launcher always uses `--rm` (verified: `docker ps -a` empty for the image after a killed run) |
| Development images | `docker rmi contagent:<tag>` … | never the application's `grr-fyi` images |
| Caches (mise tools, Go modules, build cache) | `rm -rf ~/.cache/contagent` | recreated on next launch; this is also the fix for a poisoned toolchain cache |
| Dedicated profile | [3.6](#36-reset-and-revocation) | `rm -rf ~/.pi-contagent` for a full reset |
| contagent state | `rm -rf ~/.contagent ~/.local/state/contagent` | rarely needed |
| Generated build files | live in the **contagent** checkout (`.Dockerfile.generated`, `.contagent-*.generated`) and are gitignored there | never copy them into an application repo |
| Repository | `git status` must show only `.contagent.yaml` and `docs/` from this workflow | the workflow creates **no** build artifacts in the project |

### 5.5 Steps the agent must never automate

Restating [1.9](#19-actions-reserved-for-the-operator) as an operational rule, because
these are the actions that widen the boundary or touch host state a human owns:

1. Cloning a repository or choosing its location / branch / worktree.
2. Authenticating the dedicated profile, or copying any credential-bearing file into it.
3. Any host→profile sync beyond what the operator has already allowlisted
   ([3.3](#33-sync-procedure-operator-run-allowlist-only)).
4. Adding a mount, a service credential, or an environment variable
   ([4.5](#45-checklist-for-adding-a-credential-or-mount)).
5. Enabling `--network host`, `--docker-args`, SSH forwarding, or an unbuilt feature.
6. Building/rebuilding the image, `npm install` in the contagent checkout, `docker rmi`.
7. `git push`, `make docker-push` / `helm-push`, any deployment.
8. Destructive cleanup (`rm -rf` of profile or caches, `docker system prune`, volume removal).

Everything else — editing the project, `make test` / `make check` / `build` / `run`,
`mise install` of project tools, querying a granted service endpoint — is ordinary agent
work inside the container.

### 5.6 Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `env: 'make': No such file or directory` | image built without `--build` (the base image has no `make`) | rebuild per [2.2](#22-build-the-development-image) |
| `make install` succeeds but `make check` reports `goimports: command not found` | mise's Go backend exported `GOBIN` into its own install dir, off `PATH` | restore `MISE_GO_SET_GOBIN: "false"` in `.contagent.yaml`, re-run `make install`; confirm with `go env GOBIN` (empty) and `ls $GOPATH/bin` |
| `go`/`goimports` missing in an **interactive** shell but present for `contagent -- make test` | `bash -l` sources `/etc/profile`, which reassigns `PATH` and drops injected dirs | `eval "$(mise env)"` in the session, or use the one-shot form |
| Host-edited files show up as owned by the wrong user, or writes are denied | UID/GID mapping mismatch, or an earlier `docker exec` ran as root and chowned them | check `id` inside vs `id -u` outside; exec through `/entrypoint.sh` (not bare `bash`); repair ownership on the host |
| Container group named `dialout` where the host says `staff` | the entrypoint renames the container group to match the host **GID** (20 here) | cosmetic; ignore unless group-based file permissions actually matter |
| `ERROR: no source for mount <path>` | a mount source outside `$HOME`/project does not exist | create it deliberately, or remove the config entry — this failure is correct behaviour, do not "fix" it by pointing at something broader |
| An unexpected empty file or directory appeared in a project or in `$HOME` | the launcher auto-creates missing sources under `$HOME`/project, and Docker creates a missing *file* mountpoint inside the project mount | check for a typo in a `path`; note that the `.envrc-priv` mask intentionally relies on this (see the caveat below) |
| `mise` fails with `Exec format error`, or installs macOS binaries | host mise directory got mounted | re-add `volumes: []` to the `mise` feature; confirm with `--show-config` |
| `mise ✗ lua@5.1.4 … Failed to build Lua` / `readline/readline.h: No such file or directory` | lua's mise backend compiles from source and needs `libreadline-dev`/`libncurses-dev`, which `--build` does not provide | drop the lua pin for container use, or add a project-specific Dockerfile part; `make test`/`make check` do not need lua |
| `go: downloading go1.25.11` / `requires go >= 1.26.0; switching to go1.2x` on first build | `.tool-versions` pins below the toolchain that `go.mod`/the tools require; `GOTOOLCHAIN=auto` provisions it | expected (see [2.4](#24-version-reconciliation-and-the-known-discrepancy)); needs network once, then cached in `/var/cache/contagent/go` |
| Host service unreachable from the container | bind address, firewall, or missing `host.docker.internal` on Linux | work through the [4.3](#43-deciding-whether-a-service-is-reachable-at-all) table |
| `--network host` unrecognised / ignored | Docker Desktop < 4.34, host networking not enabled, or Enhanced Container Isolation on | stay on ordinary networking; see [4.4](#44-opt-in-fallback---network-host) prerequisites |
| SSH auth unexpectedly **works** | launched without `env -u SSH_AUTH_SOCK` | relaunch with it; the absence of the `WARN: SSH agent not available…` line is the tell |
| `pi` says `No API key found for the selected model` | the dedicated profile is unauthenticated (correct, expected state) | authenticate per [3.4](#34-authentication) |
| Copied settings/extensions silently do nothing in the container | the copy preserved **dangling symlinks** (this host's profile entries are dotfiles symlinks) | re-copy with `cp -L` / `rsync -aL` per [3.2](#32-why-the-host-layout-complicates-a-naive-copy) |
| Container Pi behaves oddly after host profile changes | the dedicated profile is a stale snapshot | re-run [3.3](#33-sync-procedure-operator-run-allowlist-only), or reset per [3.6](#36-reset-and-revocation) |
| A killed/interrupted session left confusion | — | no recovery action needed: `--rm` removes the container and bind-mount writes are already durable on the host (verified in [Validation record](#validation-record)) |

**The mask's visible side effect.** In a repository that does *not* already contain
`.envrc-priv`, the mask still mounts a zero-byte file at that path — and because the
target sits inside the project bind mount, an empty `.envrc-priv` **appears on the host**
(verified on a second repository). It is harmless (the project's own `direnv` logic
sources an empty file, and the file is gitignored in this repository's convention), but
it is a real artefact of using this config elsewhere. Either keep the mask line only in
repositories that have the credential file, or delete the empty file afterwards — it will
simply be recreated on the next launch.

### 5.7 Inspect before every agent session

```bash
cd /path/to/repo
env -u SSH_AUTH_SOCK ~/repos/contagent/contagent --show-config          # intended features & mounts
git -C ~/repos/contagent log -1 --format='contagent %H %ci'            # what the image was built from
```

Confirm all of these in the `--show-config` output (and again in the `[volumes]` lines the
launcher prints at start):

* enabled features are exactly `base, go, psql, mise, pi, runtime, build` — nothing else;
* the `pi` volumes list `~/.pi-contagent` and **never** `~/.pi`;
* `mise` and `psql` show `volumes: []`;
* the `${PWD}/.envrc-priv` mask entry is present and `read_only: true`;
* no `~/.ssh`, `~/.aws`, `~/.config/gh`, `~/.docker`, or `/var/run/docker.sock` anywhere.

Then, once inside the container, confirm the negative facts:

```bash
ls -d ~/.pi ~/.ssh ~/.aws ~/.config/gh 2>&1 | grep -c 'No such file'   # expect 4
wc -c < .envrc-priv                                                     # expect 0
env | grep -iE 'key|token|secret|password'                              # expect nothing
env -u SSH_AUTH_SOCK … -- bash -c 'id'                                  # uid/gid match the host user
```

If any expectation fails, **stop and fix the config before starting the agent** — the
rest of the workflow assumes this boundary.

### 5.8 Security review checklist for workflow changes

Apply to any change in `.contagent.yaml`, this document's mount table, or a build command:

- [ ] Every added/changed volume has a comment naming its purpose and whether it is secret-bearing.
- [ ] New mounts are read-only unless write is provably required.
- [ ] `--show-config` after the change still shows no host `~/.pi`, no `~/.ssh`/`~/.aws`/`~/.config/gh`, no Docker socket.
- [ ] The `.envrc-priv` mask entry survived (it is easy to lose when `runtime` volumes are re-stated).
- [ ] No secret values, operator home paths, or machine-specific GIDs are committed.
- [ ] Capability additions use the [4.5](#45-checklist-for-adding-a-credential-or-mount) table, completed.
- [ ] `--docker-args` additions reviewed as code — they bypass `--show-config` visibility.
- [ ] Launch commands still carry `env -u SSH_AUTH_SOCK`, or forwarding is an explicit, documented decision.
- [ ] After a contagent upgrade: embedded defaults re-inspected and [1.10](#110-matrix-self-check) re-verified.
- [ ] Non-built features were not added "just in case" (`--docker`, `--gh`, `--aws`, `--hostbridge`).
- [ ] Application code, `Dockerfile`, `Makefile`, Helm chart, and migrations are untouched by the change.

### 5.9 Acceptance checklist

Run against `grr-fyi` on the reference host (macOS, Docker Desktop engine 24.0.2,
linux/arm64). ✅ = executed and observed; ⚠️ = documented, not executable here.

**Boundary**

- ✅ `--show-config` enables exactly `base, go, psql, mise, pi, runtime, build`; no `docker`/`gh`/`aws`/`hostbridge`.
- ✅ Host `~/.pi` absent in-container; `~/.pi-contagent` mounted instead (`ls ~/.pi` → No such file).
- ✅ `~/.ssh`, `~/.aws`, `~/.config/gh` all absent in-container.
- ✅ Host mise/Supabase dirs not mounted (`volumes: []`).
- ✅ `.envrc-priv` reads as 0 bytes; an in-container write fails with `Read-only file system`.
- ✅ Host environment not inherited: no credential-shaped vars in the container; `SITE_URL`/`DB_PATH`/`LITESTREAM_*` unset; `direnv` absent so `.envrc` is inert.
- ✅ SSH agent not forwarded with `env -u SSH_AUTH_SOCK` (`WARN: SSH agent not available…` printed); confirmed forwarded when the variable *is* exported, which is why the unset is mandatory.
- ✅ Launch diagnostics print mount **paths** only — no secret values.
- ✅ A mount source outside `$HOME`/project fails loudly: `ERROR: no source for mount /definitely-missing-outside-home`.
- ✅ Without a project override the embedded default really does mount `~/.pi` (verified in a repository lacking `.contagent.yaml`) — the committed baseline is what prevents it.

**Developer experience**

- ✅ Image builds from pinned versions; `pi 0.87.1`, `go1.27.1 linux/arm64`, `mise 2026.9.13 linux-arm64`, `psql 17.11` all report inside the container.
- ✅ Identity mapping correct: `uid=501(nietaki)` in-container, workdir = the host absolute project path.
- ✅ Project is editable and writes land on the host (probe file created in-container, read back host-side).
- ✅ `make test` passes in-container (all 12 packages).
- ✅ `make install` + `make check` pass in-container after the `MISE_GO_SET_GOBIN` fix — goimports, coverage (77.4%), vet, staticcheck.
- ✅ No tracked file was modified by the workflow (`git status` clean apart from `.contagent.yaml` and `docs/`).
- ✅ mise honours `.tool-versions` per project (`go` → 1.25.6 in a second repository), and the `go.mod` 1.25.11 requirement surfaces as a visible `downloading go1.25.11` toolchain provisioning rather than a silent fix.
- ✅ Interrupted run recovery: `docker kill` during a session → container auto-removed, project writes durable, profile intact, relaunch fine.
- ✅ Portability: the same `.contagent.yaml` works unchanged in a second mise-based repository.

**Host services**

- ✅ `host.docker.internal` resolves (`192.168.65.254`); a host HTTP endpoint bound to `*:8080` returns 200 from the container.
- ✅ Unavailable-service failure mode is a clean `Connection refused` on a closed port.
- ⚠️ **Security finding instead of the expected failure:** the host's loopback-only Postgres.app was reachable **as superuser `postgres` with no password**, because Docker Desktop's proxy connects from the host's own loopback ([4.1](#41-default-route-hostdockerinternal)). Connectivity was verified with `select current_user, inet_server_addr(), inet_client_addr()`; no data was read or written. Requires an operator follow-up with a dedicated low-privilege role.
- ⚠️ `--network host` fallback not exercised: this host's engine (24.0.2) predates Docker Desktop 4.34 host networking. Prerequisites, caveats, and rollback commands are documented in [4.4](#44-opt-in-fallback---network-host) and the rollback probe was verified to report the ordinary boundary correctly.
- ⚠️ MCP request validation skipped: no safe test endpoint/credentials available, per the plan's own condition. The mechanism (explicit host URL, credentials via [4.5](#45-checklist-for-adding-a-credential-or-mount)) is documented instead.
- ⚠️ Linux behaviour documented from Docker's platform notes, not executed here (single macOS host available).

**Profile**

- ✅ Empty dedicated profile bootstraps: Pi creates its own `auth.json`, `models-store.json`, `sessions/` under `~/.pi-contagent/agent` and stops with a clear "No API key found … Use /login" rather than falling back to host state.
- ✅ A file placed in the dedicated profile is genuinely loaded — verified with a startup-probe extension that printed its path from `~/.pi-contagent/agent/extensions/` (removed afterwards).
- ✅ Authentication is separate (or per-run via declared environment variables); no host credential was copied.

---

## Validation record

| Item | Value |
|---|---|
| contagent revision used | `53e4a99137bb0e9ec4089f3dcf3b7e79336ddfa8` ("Print mounted volumes at startup"), checked out at `~/repos/contagent` |
| Image | `contagent:20260924_215221-g63ad5d2_DIRTY` + `contagent:latest` — the voom tag derives from a **disposable working copy** in the session temp directory, not the operator's checkout (the agent's sandbox denies writes outside the workspace), so the tag is provenance-synthetic while the *content* is that revision |
| Feature versions built | pi `0.87.1` · go `1.27.1` · mise `2026.9.13` · build-essential `12.12` · postgresql-client `17.11` (Debian index, unpinned by policy) |
| Host | macOS, Docker Desktop, engine `24.0.2`, `linux/arm64` |
| Repository validation | `make test` ✅, `make check` ✅ (in-container, existing targets); application `Dockerfile`, `Makefile`, Helm chart, migrations, and Go code **unmodified** |
| Repository diff | added `docs/contagent-pi-workflow.md`, `.contagent.yaml`, and a README pointer; no other files touched |

### Deviations from the plan

1. **`--build` added to the baseline image.** The plan's minimum feature set
   (`pi go psql mise`) cannot run this repository's workflow: `make` is supplied only by
   the `build` feature, and the plan itself requires validation through `make test` /
   `make check`. `build` adds Debian packages only — no mount, no credential — so the
   security boundary in [1.5](#15-explicitly-denied-by-default) is unchanged. Recorded in
   [1.3](#13-baseline-feature-set) and [2.2](#22-build-the-development-image).
2. **`MISE_GO_SET_GOBIN: "false"` added to the mise feature environment.** Without it,
   mise hides the `make install` tools off-`PATH` and `make check` fails; the plan did not
   anticipate this interaction. See [2.4](#24-version-reconciliation-and-the-known-discrepancy).
3. **Documentation and config placed in this repository** (`docs/`, `.contagent.yaml`),
   resolving the plan's open question. The agent sandbox denies writes outside the
   workspace, so the operator's dotfiles/contagent repositories were never candidates for
   this change; the plan explicitly permits committed repo-local config provided it holds
   no secrets or operator home paths, and a repo-local `.contagent.yaml` is also the
   fail-safe option (the acceptance checklist shows the embedded default mounting `~/.pi`
   whenever no override is in scope).
4. **`.envrc-priv` is masked rather than excluded.** The plan's non-goal "do not mount
   `.envrc-priv`" cannot be met literally, because the file lives inside the read-write
   project mount that the workflow requires. The nested read-only empty-file mount
   ([1.6](#16-the-envrc-priv-problem-and-its-mask)) is the strongest available mechanism
   and was verified to hide contents and reject writes.
5. **Loopback reachability finding contradicts a plan assumption.** The plan expected
   host-loopback-bound services to be unreachable via `host.docker.internal` on Docker
   Desktop. Measured behaviour is the opposite (the proxy dials from the host's own
   loopback), and the local Postgres.app consequently accepted a **passwordless
   superuser** connection from the container. Documented as a security caveat with
   mitigation rather than as a networking limitation
   ([4.1](#41-default-route-hostdockerinternal)).
6. **Second-repository acceptance used a disposable mise fixture** in the session temp
   directory plus a read-only `--show-config` probe in a real mise-based repository,
   instead of running the full suite in an unrelated operator repository — that would
   have mounted and exercised writes in a project outside this task's scope.
7. **TDD adapted to a documentation/workflow deliverable.** The plan changes no
   application code, so red-green-refactor cycles do not apply. Every factual claim in
   this document was instead established by executing the corresponding command in a real
   container and recording the output, and the acceptance checklist in
   [5.9](#59-acceptance-checklist) marks each item ✅ (executed) or ⚠️ (documented, not
   executable on this host) so no unverified claim is presented as tested.
