# Obsidian Compatibility

Ruin was built with Obsidian-compatibility in mind. While Ruin is a WIP and stuck in the terminal, some users might find Obsidian a good companion tool. Others might be coming to Ruin with an existing, extensive Obsidian vault.

## Maximum Capability

> [!NOTE]
> - Set `tag_frontmatter: false` in `~/.config/ruin/config.yml`.
> - Don't use Ruin's `#spaced tags#` format.

On an existing Obsidian doc, `ruin doctor` will set up necessary indices and frontmatter ruin needs to function. **Do this on a copy of your vault first to ensure compatibility**

## Differences

### Indices
Both Ruin and Obsidian use indices to speed up search and other operations. Obsidian's indices exist in memory when the application is open and are available via the plugin API and CLI. Ruin's indices are persistent and stored in plaintext files within the vault in the `.ruin` directory, as yml files. Every write to Ruin will update these indices as necessary and the `ruin doctor` command ensures they are up-to-date with the current vault. These indices are safe to read directly and useful for third-party tooling (ie. if you want a list of every tag used in the vault for a Raycast extension).

### UUID
Ruin generates a UUID for every file logged with the tool. This UUID is stored in the note's frontmatter with the key `uuid` and referenced in the plaintext indices. Obsidian does not do this.

### Tags
Ruin's tag syntax is a superset of Obsidian's. Both Obsidian and Ruin support `#tags`, `#underscored_tags`, `#dashed-tags`, and `#nested/tags`. In both applications, tags are case-insensitive.

Ruin supports tags with spaces in them as `#tag with space#` (credit where due, taken from Bear). If strict Obsidian compatibility is important, don't use this tag format.

As a convenience, Ruin extracts tags from the body and stores them in the frontmatter (under `tags`, `inline-tags`, and `inherited-tags`). This somewhat clashes with Obsidian's use of the `tags` field, which has no relation to the tags in the note content's body and is just another way to tag a file. **By default, on write, ruin will remove tags in frontmatter that are not present in the body**.

If you use Obsidian frontmatter tags, you likely want to set `tag_frontmatter: false` in `~/.config/ruin/config.yml`. This will disable `tags` and `inline-tags` from being written. This will have no impact on how Ruin works. Existing `tags` will remain but will be ignored by `ruin search` and other tooling. `inherited-tags` will still be written (unless `tag_inheritance: false`, which will impact how `ruin search` works).

### Frontmatter
Broadly, Ruin uses frontmatter as a cache / index and Obsidian uses it as a convenience.

In addition to the fields mentioned above, Ruin writes `linked-cards`, `updated`, and `dates` in the frontmatter to help speed up search. If these fields are missing or malformed, they might be missed in search or extraction. `ruin doctor` deterministically rebuilds these fields and is always safe to run.

With an existing vault, things should just work. Running `ruin doctor` will generate the necessary indices and bring the vault in line with Ruin's expectations.

### Ruin Formatting Obsidian Doesn't Support
- `@date` values are not supported or rendered in Obsidian by default. Certain plugins might use `@` as a special character / tag, so keep that in mind.


### Obsidian Formatting Ruin Doesn't Support
- **Frontmatter Aliases** (`aliases`)
- **Inline metadata** (`key:: value` in body content)
- **Block references** (`^block-id`)

These are ignored and have no impact on Ruin functionality. Aliases and Block references are on the roadmap and will be supported prior to 1.0. 

_Check this space_, as they say.
