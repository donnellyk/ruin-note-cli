# Configuration

Ruin stores its configuration in `~/.config/ruin/config.yml`.

## Config file

```yaml
vault_path: ~/notes
versioning: true
tag_inheritance: true
```

## Keys

| Key | Type | Default | Env Override | Description |
|-----|------|---------|-------------|-------------|
| `vault_path` | string | _(none)_ | `RUIN_VAULT` | Path to the notes vault directory |
| `versioning` | bool | `true` | `RUIN_VERSIONING` | Enable git auto-versioning (one commit per command) |
| `tag_inheritance` | bool | `true` | `RUIN_TAG_INHERITANCE` | Enable inherited tags from parent notes |

Environment variables take precedence over the config file. For boolean keys, set to `false` or `0` to disable.

## Managing config

```bash
# Show all config
ruin config

# Show a specific key
ruin config vault_path

# Set a value
ruin config vault_path ~/notes
ruin config tag_inheritance false
```

## Tag inheritance

When enabled (default), child notes automatically inherit global tags from their parent chain. These are stored in the `inherited-tags` frontmatter field and kept in sync when parent tags change.

When disabled (`tag_inheritance: false`), no inherited tags are computed on save, and `ruin doctor` will clear any existing `inherited-tags` fields.
