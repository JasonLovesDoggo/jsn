# scriptkit (`sk`)

`sk` is a fuzzy-searchable terminal UI for launching small scripts and toggles. Running `sk` opens a Bubbletea view with a search box, arrow-key navigation, `enter` to run the highlighted script, `ctrl+r` to rerun the previous selection, and `?` to toggle the output pane. Press `→` to enter action mode where `e` edits the manifest, `o` opens the underlying script for the current platform (or lets you choose enable/disable for toggles), `n` creates a new script using a slug prompt, and `d` deletes the selection (with confirmation).

## Script layout

Scripts live under `var/skit/scripts/<slug>/`. Each directory contains one manifest named `skit.toml` and whatever executable files it references (shell, Python, PowerShell, etc.). Example run script:

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

You can override commands per platform (`darwin`, `linux`, `windows`) by adding extra keys next to `default`. Toggle runs are recorded in `var/skit/state/<slug>.json` so the TUI can show and flip the next pending action.
