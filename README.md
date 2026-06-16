# modpack-installer

A tiny, **self-contained cross-platform installer** (Windows `.exe`, Linux, macOS) that sets up
a player's Minecraft to join your modpack — with **Fabric**, the **modupdater** mod, and your
**staging + prod servers** pre-configured.

It targets the **official Minecraft launcher** when present, and falls back to **Prism Launcher**.
No Java, Python or Node required on the player's machine — each installer is a single native binary.

```
detect launcher ─▶ install Fabric (no Java needed) ─▶ drop modupdater.jar ─▶ pre-fill servers ─▶ create profiles
   official / Prism      via Fabric meta API            in each profile's mods/    in servers.dat     Prod + Staging
```

After install, the player just opens their launcher, picks the **Prod** (or **Staging**) profile,
and plays — **modupdater syncs all the other mods on first launch**. The only coupling with
modupdater is that the installer drops its `.jar` into the profile's `mods/` folder.

---

## How it works

* **Fabric without Java** — instead of running the Fabric installer (which needs a JRE), it fetches
  the loader's *profile JSON* from `meta.fabricmc.net`, writes it into `versions/…`, and adds a
  launcher profile. The official launcher downloads the Fabric libraries itself on first launch.
* **Two profiles**, each with its own game directory (so test/prod mods & worlds never mix):
  `Prod` (branch `main`) and `Staging` (branch `staging`).
* **`servers.dat` is merged**, never overwritten — existing servers the player already has are kept.
* **`launcher_profiles.json` is merged** — existing launcher profiles are preserved.

---

## 1. Configure

Edit **`internal/config/default.json`** (baked into the binaries at build time), or ship a
**`modpack.config.json`** next to the binary (it overrides the embedded default at runtime).

| Field | Meaning |
|---|---|
| `modpackName` | Display name + folder slug. |
| `minecraftVersion` | e.g. `1.21.11`. |
| `fabricLoaderVersion` | empty = latest stable for that MC version. |
| `modUpdaterJarUrl` | **direct URL** to your `modupdater.jar` (the installer downloads it). |
| `modUpdaterJarName` | filename written into `mods/` (default `modupdater.jar`). |
| `servers[]` | `{ name, address }` — added to every profile's server list. |
| `profiles[]` | `{ key, name, icon, branch }` — one launcher profile each. |

Config precedence: `--config <path>`  ›  `modpack.config.json` beside the binary  ›  embedded default.

## 2. Build

Requires [Go](https://go.dev/dl/) ≥ 1.22 (only to *build*; players need nothing).

```bash
./scripts/build.sh        # cross-compiles into ./dist for win/linux/mac (amd64 + arm64)
```

Outputs:
```
dist/installer-windows-amd64.exe
dist/installer-linux-amd64        dist/installer-linux-arm64
dist/installer-macos-intel        dist/installer-macos-apple-silicon
```

## 3. Distribute & run

Ship `dist/` together with `install.sh` / `install.command` (and `modpack.config.json` if you
didn't bake your values into the embedded default).

* **Windows** — double-click `installer-windows-amd64.exe`.
* **Linux** — `./install.sh`
* **macOS** — double-click `install.command` (first time: right-click → *Open* to pass Gatekeeper,
  since the binary is unsigned).

### Flags
```
--config <path>     use a specific modpack.config.json
--launcher <name>   auto | official | prism   (default auto)
--profiles a,b      install only these profile keys (default: all)
--dry-run           preview everything, write nothing
-y                  skip the confirmation prompt
```

Try a safe preview first:
```bash
./install.sh --dry-run
```

---

## Notes & caveats

* **modupdater** is treated as a black box — the installer only places its jar. How it discovers the
  repo/branch is modupdater's concern.
* The **official launcher** path is the primary, fully-tested one. **Prism** support creates an
  instance via `mmc-pack.json` (Prism resolves the loader) — verify on your Prism version.
* macOS/Windows binaries are **unsigned** — expect a Gatekeeper / SmartScreen prompt. Sign them for
  a smoother rollout if you distribute widely.
* `servers.dat` is written as uncompressed NBT and **merged** with any servers already present.

## Tests

```bash
go test ./...     # NBT codec round-trip (the servers.dat writer)
```
The installer was validated end-to-end against a fake `.minecraft`: Fabric resolution, profile
creation (preserving existing profiles), modupdater placement, and an idempotent `servers.dat` merge.
