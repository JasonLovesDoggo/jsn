# scriptkit (`sk`)

`sk` is a fuzzy-searchable terminal UI for launching small scripts and toggles. Running `sk` opens a Bubbletea view with a search box, arrow-key navigation, `enter` to run the highlighted script, `ctrl+r` to rerun the previous selection, and `?` to toggle the output pane. Press `→` to enter action mode where `e` edits the manifest, `o` opens the underlying script for the current platform (or lets you choose enable/disable for toggles), `n` creates a new script using a slug prompt, and `d` deletes the selection (with confirmation). Hit `h` to view run history (persisted for toggles, in-memory for others). Scripts live under `SKIT_HOME/scripts` (default `~/.skit/scripts`) while toggle state stays in `SKIT_HOME/state`, so each machine keeps its own toggle history.

Optional git syncing:

- Set `SKIT_SYNC_REPO` to a clone URL (and `SKIT_SYNC_BRANCH` if you do not want the default branch). On startup, `sk` will clone the repo into the scripts directory if needed.
- Run `sk sync` to pull with rebase, commit any pending file changes, and push back to origin. This makes it trivial to sync a script collection via GitHub or any other remote.

Bundled scripts (copied the first time the scripts directory is created):

- `cloudflare-dns` – toggles Cloudflare DNS on/off. Honors `SKIT_SERVICE` for the network service name.
- `flush-dns` – flushes resolver caches and prints `scutil --dns`.
- `ip-info` – displays IPv4/IPv6 details for each hardware port.

## Script layout

Scripts live under `SKIT_HOME/scripts/<slug>/` (default `~/.skit/scripts/<slug>`). Each directory contains one manifest named `skit.toml` and whatever executable files it references (shell, Python, PowerShell, etc.). Example run script:

```toml
name = "Hello There"
description = "Demonstrates a simple one-off script."
tags = ["example"]
type = "run"

[exec]
default = "./hello.sh"
```

Example toggle script:

```toml
name = "DNS Toggle"
type = "toggle"
state_hint = "dns"

[toggle.enable]
default = "./enable.sh"

[toggle.disable]
default = "./disable.sh"
```

You can override commands per platform (`darwin`, `linux`, `windows`) by adding extra keys next to `default`. Toggle runs are recorded in `SKIT_HOME/state/<slug>.json` so the TUI can show and flip the next pending action. Prefix a command with `!` to run it inline via the system shell without creating a script file (e.g. `darwin = "!networksetup -setdnsservers Wi-Fi 1.1.1.1 1.0.0.1"`).
