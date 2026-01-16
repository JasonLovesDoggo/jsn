# lazypuffer

`lazypuffer` is a lazygit-style TUI for browsing Turbopuffer namespaces and documents.

## Quick start

```bash
go install pkg.jsn.cam/jsn/cmd/lazypuffer@latest
```

On first run, lazypuffer will prompt for:
- API key
- Region (or Base URL)
- Optional default namespace

Those values are saved to `~/.config/lazypuffer/config.toml`. You can also open the config anytime with `c`.

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
- `v` vector query mode (vector literal or text to embed)
- `g` fetch by document id
- `y` copy selected document detail to clipboard
- `f` edit filters

## What are queries running on?

lazypuffer shows the exact ranking target in the Docs header:
- `rank: bm25(<text_attr>)` for text mode
- `rank: vector(<vector_attr>)` for vector mode
- `rank: id asc` when the query input is empty

`text_attr` and `vector_attr` come from your profile in `config.toml`.

### Vector mode input

When vector mode is active:
- If the query looks like a vector literal (numbers / brackets / commas), lazypuffer uses it directly.
- Otherwise lazypuffer treats the query as text and generates an embedding using `embedding_model`.
- If `embedding_model` is not set, you will be prompted to set it before embedding can run.
- OpenAI auth comes from `openai_api_key` or `OPENAI_API_KEY`.

## Filters

Press `f` to edit filters. Comma or `&` separated filters are ANDed together, `||` groups are ORed. AND binds tighter than OR.

Supported operators:
- `=` `!=` `>` `>=` `<` `<=`
- `in` / `not in`

Examples:
- `status=published, likes>=10`
- `status=published & likes>=10`
- `status=published || status=draft`
- `author_id in 1|2|3`
- `tag in ["ai","infra"]`

To clear filters, submit an empty value.

## Profiles

Profiles let you switch orgs or environments (staging/prod).

- Press `p` with a single profile to create a new profile name.
- lazypuffer will open your config so you can fill in API key/region for the new profile.

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
embedding_model = "text-embedding-3-small"
openai_api_key = "sk-..."

[profile.staging]
api_key = "tp_..."
region = "gcp-us-central1"
namespace = "posts-staging"
```

## Notes

- Vectors are excluded from query results by default to keep the UI fast.
- The Meta pane shows a compact summary (size, rows, timestamps, encryption, index).
- The Schema pane shows the full schema, with scroll.
