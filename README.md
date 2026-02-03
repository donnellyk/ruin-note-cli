# Ruin Note CLI

A Zettelkasten-ish note-taking CLI written in Go. Don't organize; query.

Take small, atomic notes saved as simple markdown files. Later, compose them into different "documents", depending on your needs. This probably has an audience of one (me) or zero (turns out, not me); time will tell.

Eventually, this utility will form the core of a TUI and nice Mac GUI but should be usable* as-is.

## What Does This Do?!
I like the idea of a daily note. A default place to put all your thoughts. My struggle with the concept is that once a piece of information is in a daily note, its stuck there. You have some options. You can move it out of the daily note, thus reducing utility of the note. You can duplicate the info. Or you can forego the daily note entirely but then you have to think about 'Where this goes?' everytime to want to get something out of your head.

This tool tries to solve that by letting everything you create or update in a day be part of your Daily Note, automatically. Via searching & saved queries, you can extend that idea to have custom notes (daily, weekly, everything) based on anything in your vault. You can compose a document of every thing you ever wrote on a `#important-project`, or just the in-line ideas you need to follow up on for that project. You can make edits to that document and the changes are automatically reflected in the individual markdown files, regardless of location and organization.

Like I said, this will either have an audience of one or zero...

### Should I Use This Yet?
Probably not, its under active development. Data loss is highly likely and the CLI contract will change.

### What's With the Name?
The pretentious answer is it's a reference to _The Waste Land_ by T.S. Eliot (that I'm likely misinterpretting). 
> These fragments I have shored against my ruins.

The real answer is its memorable, short, and a beautiful word.

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
