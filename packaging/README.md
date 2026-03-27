# Unified Installer Notes

The release goal is to install the Wails desktop app and the CLI together.

## Native installer strategy

- macOS: signed `.pkg` installs `Logos AI.app` into `/Applications`, installs a `logos-ai` launcher into `/usr/local/bin`, and installs `logos` into `/usr/local/bin`
- Windows: NSIS `.exe` installs `logos-ai.exe` and `logos.exe` into the same install directory
- Linux: `.deb` installs the GUI to `/opt/logos-ai/logos-ai` and the CLI to `/usr/local/bin/logos`

## Power-user installers

- Homebrew cask installs the app and the bundled top-level CLI from the same macOS archive
- Scoop installs the GUI app and the CLI from the same Windows bundle archive
- `scripts/install.sh` installs the macOS or Linux bundle for users who still want a shell installer
  On macOS, the shell install places the app bundle under `~/Applications/logos-ai.app` by default and also installs the `logos-ai` launcher plus the `logos` CLI on `PATH`.

## Release-day manifest update

Homebrew and Scoop both need release-specific checksums. After you build the release assets, render the package
manager manifests with:

```bash
scripts/release/render_manifests.sh \
  v0.1.0 \
  wails/bin/logos-ai-macos-universal.tar.gz \
  wails/bin/logos-ai-windows-amd64-bundle.zip
```

That updates:

- `packaging/homebrew/Casks/logos-ai.rb`
- `packaging/scoop/logos-ai.json`

The macOS tarball intentionally contains both `logos-ai.app` and the `logos` CLI so the same asset works for
Homebrew and for `install.sh`.

## Why not DMG-only on macOS?

A plain `.dmg` is usually a drag-and-drop app container. It is fine for app-only installs, but it does not normally
install a CLI on `PATH`. The `.pkg` path is the correct one-step installer for non-technical macOS users when both
the app and the CLI need to be installed together.
