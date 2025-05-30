# jlort jlort 2

A Discord bot for me and my friends to have fun with I guess.

## History

Version 1 was written in January 2018 using discord.py and 8-space indentation. Its only function was to announce when someone joined the call without clogging the chat like FredBoat. It launched with that functionality and a few of the commands in `baseCmd.go`

As time went on, I added more features. Some by request, some to satisfy curiosity, and some to troll certain people. But at some point, I introduced something that caused a memory leak over time. In Python.

I could not track down this memory leak, and given that the codebase was becoming a bit bleh and jlort was using quite a bit of memory in the first place, I decided to remake it in a different language. The initial port was done in a few days, but encountered some problems related to concurrent map accesses since that wasn't something I'd had to account for before.

Then the codebase stagnated for a while. But maybe I'll come up with something sooner or later...

## Usage

This program relies on some files existing before it can run. You must create a file called `key.txt` containing the bot key. This file should not have a trailing newline.

Additionally, the bot requires a database file to function properly. A creation script for this file can be found in `dbGen/dbGen.go`. If you have existing persistent data for an older version of the bot, the script will migrate it.

The clickart module has a feature that requires a folder of sounds to play as affirmations for successfully performing an action, `modules/clickart/affirmations`. The sounds should be in Ogg Opus format with 1 or 2 channels, a bitrate of approximately 64k, and an audio rate of 48k. Another file at `modules/clickart/clicker.ogg` is also required, and should be in the same format.

## Removed features

These commands existed and were removed before the version I uploaded to GitHub. Just for historical reference.

- `~!midi <file/attachment>` - Plays a midi file using Timidity++ and SGM v2.01
- `~!define <word>` - Looks something up on Wikipedia. Had an easter egg where trying to define the word "dead server" would give an invite to a friend's *extremely* dead server.
- `~!booru <tag>` - Would return the most recent image with a given tag from Safebooru. Or Gelbooru, if the channel was configured that way.
- `~!setting queue-length <number>` - A setting for the maximum queue length. I forgot about it and eventually removed it once I realized that it no longer had any effect. Remaining settings got thier own commands, since everybody used those aliases anyway.
- `~!something` - I don't remember what this does, but it was in an old (pre-OMORI) help file and has no description.
- `~!boot <user>` - Kick someone out of a call in case they fell asleep. Turns out that we're not nice people.
- `~!sausage` - Causes a segmentation fault, for debugging.
- `~!ud` - Displays definitions from Urban Dictionary. Made specifically to troll a friend whose full name had an unflattering one.
- `~!relics` - A gacha game that was troubled by the fact that there were only 7 things to collect.
- `~!trade` - A trading system for the above gacha game.
- `~!duel` - A versus "minigame" based completely on chance. Minigame in quotes because it just flipped a coin to determine who won after 30 seconds.
- `~!play` - Part of a larger system of music commands, was removed shortly after the relaunch of Rythm due to the age of music bots being long over.
