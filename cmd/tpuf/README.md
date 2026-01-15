# tpuf

`tpuf` is a lazygit-style TUI for browsing Turbopuffer namespaces and documents.

## Quick start

```bash
go run ./cmd/tpuf
```

On first run, tpuf will prompt for:
- API key
- Region (or Base URL)
- Optional default namespace

Those values are saved to `~/.config/tpuf/config.toml`. You can also open the config anytime with `c`.

## Keybinds

Global:
- `q` quit
- `tab` switch focus (namespaces ↔ right pane)
- `left` / `right` change pane (Docs / Schema / Meta)
- `r` refresh current view
- `p` cycle profiles (or create one if you only have one)
- `c` open config in `$VISUAL` / `$EDITOR`

Docs:
- `/` edit query text
- `t` text query mode (BM25)
- `v` vector query mode
- `g` fetch by document id
- `f` edit filters

## What are queries running on?

tpuf shows the exact ranking target in the Docs header:
- `rank: bm25(<text_attr>)` for text mode
- `rank: vector(<vector_attr>)` for vector mode
- `rank: id asc` when the query input is empty

`text_attr` and `vector_attr` come from your profile in `config.toml`.

## Filters

Press `f` to edit filters. Comma-separated filters are ANDed together.

Supported operators:
- `=` `!=` `>` `>=` `<` `<=`
- `in` / `not in`

Examples:
- `status=published, likes>=10`
- `author_id in 1|2|3`
- `tag in ["ai","infra"]`

To clear filters, submit an empty value.

## Profiles

Profiles let you switch orgs or environments (staging/prod).

- Press `p` with a single profile to create a new profile name.
- tpuf will open your config so you can fill in API key/region for the new profile.

Example config:

```toml
default_profile = "prod"

[profile.prod]
api_key = "tp_..."
region = "aws-us-east-2"
namespace = "posts"
vector_attr = "embedding"
text_attr = "body"
top_k = 50

[profile.staging]
api_key = "tp_..."
region = "gcp-us-central1"
namespace = "posts-staging"
```

## Notes

- Vectors are excluded from query results by default to keep the UI fast.
- The Meta pane shows a compact summary (size, rows, timestamps, encryption, index).
- The Schema pane shows the full schema, with scroll.
