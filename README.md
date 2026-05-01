# Ruin Note CLI

[![CI](https://github.com/donnellyk/ruin-note-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/donnellyk/ruin-note-cli/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/donnellyk/ruin-note-cli)](https://github.com/donnellyk/ruin-note-cli/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/donnellyk/ruin-note-cli)](go.mod)
[![License](https://img.shields.io/github/license/donnellyk/ruin-note-cli)](LICENSE)

A Zettelkasten-ish note-taking CLI written in Go. Don't organize; compose.

Take small, atomic notes saved as simple markdown files. Later, compose them into different "documents", depending on your needs. This probably has an audience of one (me) or zero (turns out, not me); time will tell.

This CLI powers [lazyruin](https://github.com/donnellyk/lazyruin) and be the core of a Mac/iOS app but should be usable as-is.

## Installation

### Homebrew (recommended)

```
brew install donnellyk/ruin/ruin-cli
```

### `go install`

Requires Go 1.26+:

```
go install github.com/donnellyk/ruin-note-cli/cmd/ruin@latest
```

### From a local checkout

Requires Go 1.26+ and [mise](https://mise.jdx.dev). After checking out the repo:

```
mise run install
```

## Get Started
- `ruin help`
- `ruin init`
- `ruin log "A really important thing to remember"` or `cat note.txt | ruin log`
- `ruin today --edit`

See [reference](docs/cli-reference.md) for more info. If you are coming from Obsidian or wish to use Ruin alongside Obsidian, see the [compatibility doc](docs/obsidian-compatibility.md).

## Key Features

### Notes are just markdown files
Your notes are just plain markdown files on a disk. Ruin vaults should _generally_ be compatible with Obsidian vaults (though extensive testing hasn't happened) and similar markdown tools.

### Context-Aware Tags
In Ruin, there are two kinds of tags: inline & global. An inline tag is a tag that is in a line of text. A global tag is a tag that's in the frontmatter, or on its own line, or on a line with only tags (and separators).

In the following example
```
I went to a very important meeting.

I should follow up with that very important thing #followup

#meeting, #projectA
```

#meeting and #projectA are global tags and #followup in an inline tag. 

Use `ruin search` to find whole files and `ruin pick` finds & extracts specific lines (based on inline tags and other parameters).

### Strong date awareness, but no daily note
When you have a daily note, everything you add to your vault comes with the question: should this go into the daily note or its own note? If you add it to the Today note, it might be hard to find later. If you add it to a relevant note or its own, you lose the context of the date. If you are like me, in event thinking about that question, you've forgotten what you wanted to write down in the first place.

Ruin tries to fix this with strong date awareness, date querying, and `ruin today`. This gets even more useful in the [TUI](https://github.com/donnellyk/lazyruin), with a dynamically generated Today view (plus Tomorrow, and any other date).

### Zettelkasten-ish atomic notes, composed into larger documents as needed
Don't think about adding a new section to a larger document, just write down what you want to capture and give the note a parent. From there `ruin compose` can build an entire document for easy reading and editing*

*editing is a bit limited for now but I hope to expand it to allow for editing as if it was a single document.


## Other Questions
### Should I Use This Yet?
Maybe. It's under active-development. Due to it just being a folder on your hard drive, data loss is unlikely but possible (back up your data either way). The CLI contract might have breaking changes until 1.0. I will try to minimize breaking changes and allow for migration via `ruin doctor` where I can.

### What's your roadmap?
I want to use/explore the feature set in the CLI/TUI for now. Some of these ideas might be too abstract/complex for day-to-day use, and significantly changing or removing them will be necessary. That's fastest when there is just a core CLI and TUI to update.

From there, an iOS and Mac app focused on quickly capturing notes will be next, reading and interacting with a full vault after that.

Finally, the intention is to have a polished, native experience on both Mac and iOS, with all the querying and editing capabilities of a modern notes app. We'll see if we get there.

The CLI and TUI will always be free and open-source. The iOS and Mac apps will be closed source and come with a small subscription, offering a limited vault size as a free tier.

Linux, Windows, and Android support are not the focus at this time; the CLI and TUI might work with them, but compatibility has not been rigorously tested.

### What's With the Name?
The pretentious answer is it's a reference to _The Waste Land_ by T.S. Eliot (that I'm likely misinterpreting). 
> These fragments I have shored against my ruins.

The real answer is it's memorable, short, and aesthetically pleasing.

### Doesn't <OrgMode|Obsidian|Lotus Notes|Notion>  do this already?
Yes. Nevertheless, here we are. 

## See Also
- [TUI](https://github.com/donnellyk/lazyruin)
- Mac App (TBD)

## AI
Claude Code was used extensively for this project. All code was read, tested, reviewed, and committed by a human.
