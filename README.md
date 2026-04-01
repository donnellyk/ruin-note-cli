# Ruin Note CLI

A Zettelkasten-ish note-taking CLI written in Go. Don't organize; query and compose.

Take small, atomic notes saved as simple markdown files. Later, compose them into different "documents", depending on your needs. This probably has an audience of one (me) or zero (turns out, not me); time will tell.

Eventually, this utility will form the core of a TUI and nice Mac GUI but should be usable* as-is.

## Key Features

### Notes are just markdown files
Your notes are just plain markdown files on a disk. Ruin vaults should _generally_ be compatible with Obsidian vaults (though extensive testing hasn't happened) and similar markdown tools.

### Context-Aware Tags
In Ruin, there are two kinds of tags: inline & global. An inline tag is a tag that is in a line of text. A global tag is a tag thats in the frontmatter, or on its own line, or on a line with only tags (and seperators).

In the following example
```
I went to a very important meeting.

I should follow up with that very important thing #followup

#meeting, #projectA
```

#meeting and #projectA are global tags and #followup in an inline tag. 

`ruin search` to find whole files and `ruin pick` finds specific lines (defaulting to inline tags only).

### Strong date awareness, but no Today note
When you have a Today note, everything you add to your vault comes with the question: should this go into the Today note or its own note? If you add it to the Today note, it might be hard to find later. If you add it to a relevant note or its own, you lose the context of the date. If you are like me, in event thinking about that question, you've forgotten what you wanted to write down in the first place.

Ruin tries to fix this with strong date awareness, date querying, and `ruin today`. This gets even more useful in the TUI, with a dynamically generated Today view (plus Tomorrow, and any other date).

### Zettelkasten-ish atomic notes, composed into larger documents as needed
Don't think about adding a new section to a larger document, just write down what you want to capture and give the note a parent. From there `ruin compose` can build an entire document for easy reading and editing*

*editing is a bit limited for now but I hope to expand it to allow for editing as if it was a single document.


## Other Questions
### Should I Use This Yet?
Maybe. It's under active-development. Due to it just being a folder on your harddrive, data loss is unlikely but possible (back up your data either way). The CLI contract might have breaking changes until 1.0. I will try to minimize breaking changes and allow for migration via `ruin doctor` in those cases. 

### What's With the Name?
The pretentious answer is it's a reference to _The Waste Land_ by T.S. Eliot (that I'm likely misinterpretting). 
> These fragments I have shored against my ruins.

The real answer is it's memorable, short, and aesthetically pleasing.

## Installation
For now, download the repo and `make install`. Eventually, `brew` once I figure out how to do that.

## Get Started
- `ruin help`
- `ruin init`
- `ruin log "A really important thing to remember"` or `cat note.txt | ruin log`
- `ruin today --edit`

See [reference](docs/cli-reference.md) for more info

## See Also
- Raycast Extension (TBD)
- TUI (TBD)
- Mac App (TBD)

## AI
Claude Code was used extensively for this project. All code was tested, reviewed, committed by a human.
