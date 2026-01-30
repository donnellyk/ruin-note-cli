# Ruin Note CLI

A Zettelkasten-ish note-taking CLI written in Go. Don't organize; query.

Take small, atomic notes. Later, compose them into different "documents", depending on your needs. This probably has an audience of one (me) or zero (turns out, not me); time will tell.

Eventually, this utility will form the core of a TUI and nice Mac GUI but should be usable as-is.

### What's With the Name?
The pretentious answer is it's a reference to _The Waste Land_ by T.S. Eliot.
> These fragments I have shored against my ruins.

The real answer is its memorable, short, and I find the word very beautiful.

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
