/*
Copyright (C) 2021-2022 jlortiz

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package quotes

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var quixote map[string][]string
var dirty bool
var quoteLock *sync.RWMutex = new(sync.RWMutex)

// ~!quote [index]
// @GuildOnly
// Gets a random quote
// If you specifiy an index, it will try to get that quote.
// Indices are the numbers beside a quote in ~!quote or the line number of a quote in ~!quotes
func quote(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	quoteLock.RLock()
	qList := quixote[ctx.GuildID]
	quoteLock.RUnlock()
	if len(qList) == 0 {
		return ctx.Send("There are no quotes. Use ~!addquote to add some.")
	}
	var sel int
	var err error
	if len(args) > 0 {
		sel, err = strconv.Atoi(args[0])
		if err != nil {
			return ctx.Send("Usage: ~!quote <index>")
		}
		if sel < 1 || sel > len(qList) {
			return ctx.Send("Index out of bounds, expected 1-" + strconv.Itoa(len(qList)))
		}
		sel--
	} else {
		sel = rand.Intn(len(qList))
	}
	return ctx.Send(fmt.Sprintf("%d. %s", sel+1, qList[sel]))
}

// ~!quotes
// @GuildOnly
// Gets all quotes
func quotes(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	quoteLock.RLock()
	defer quoteLock.RUnlock()
	qList := quixote[ctx.GuildID]
	if len(qList) == 0 {
		return ctx.Send("There are no quotes. Use ~!addquote to add some.")
	}
	guild, err := ctx.State.Guild(ctx.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}
	output := new(discordgo.MessageEmbed)
	output.Title = "Quotes from " + guild.Name
	builder := new(strings.Builder)
	for i, v := range qList {
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(". ")
		builder.WriteString(v)
		builder.WriteByte('\n')
	}
	output.Description = builder.String()[:builder.Len()-1]
	output.Color = 0x7289da
	_, err = ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, output)
	return err
}

// ~!addquote <quote>
// @GuildOnly
// Adds a quote
func addquote(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!addquote <quote>")
	}
	data, err := ctx.Message.ContentWithMoreMentionsReplaced(ctx.Bot)
	if err != nil {
		data = ctx.Message.ContentWithMentionsReplaced()
	}
	ind := strings.IndexByte(data, ' ') + 1
	dirty = true
	quoteLock.Lock()
	quixote[ctx.GuildID] = append(quixote[ctx.GuildID], data[ind:])
	quoteLock.Unlock()
	return ctx.Send("Quote added.")
}

// ~!delquote <index>
// @Alias rmquote
// @Alias removequote
// Removes a quote
// If you have Manage Server, you can do ~!delquote all to remove all quotes.
// Indices are the numbers beside a quote in ~!quote or the line number of a quote in ~!quotes
func delquote(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	quoteLock.Lock()
	defer quoteLock.Unlock()
	qList := quixote[ctx.GuildID]
	if len(qList) == 0 {
		return ctx.Send("There are no quotes. Use ~!addquote to add some.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!delquote <index>\nUse ~!quotes to see indexes.")
	}
	if args[0] == "all" {
		perms, err := ctx.State.MessagePermissions(ctx.Message)
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageServer == 0 {
			return ctx.Send("You need the Manage Server permission to clear all quotes.")
		}
		delete(quixote, ctx.GuildID)
		dirty = true
		return ctx.Send("All quotes removed.")
	}
	sel, err := strconv.Atoi(args[0])
	if err != nil {
		return ctx.Send(args[0] + " is not a number.")
	}
	if sel < 1 || sel > len(qList) {
		return ctx.Send("Index out of bounds, expected 1-" + strconv.Itoa(len(qList)))
	}
	sel--
	if len(qList) == 1 {
		delete(quixote, ctx.GuildID)
	} else {
		if sel < len(qList)-1 {
			copy(qList[sel:], qList[sel+1:])
		}
		quixote[ctx.GuildID] = qList[:len(qList)-1]
	}
	dirty = true
	return ctx.Send("Quote removed.")
}

func guildDelete(_ *discordgo.Session, event *discordgo.GuildDelete) {
	quoteLock.Lock()
	delete(quixote, event.ID)
	dirty = true
	quoteLock.Unlock()
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also loads the quotes from disk.
func Init(self *discordgo.Session) {
	err := commands.LoadPersistent("quotes", &quixote)
	if err != nil {
		log.Error(err)
		return
	}
	commands.RegisterCommand(quote, "quote")
	commands.RegisterCommand(quotes, "quotes")
	commands.RegisterCommand(addquote, "addquote")
	commands.RegisterCommand(delquote, "removequote")
	commands.RegisterCommand(delquote, "rmquote")
	commands.RegisterCommand(delquote, "delquote")
	self.AddHandler(guildDelete)
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the quotes to disk.
func Cleanup(_ *discordgo.Session) {
	if dirty {
		quoteLock.RLock()
		err := commands.SavePersistent("quotes", &quixote)
		if err != nil {
			log.Error(err)
		}
		quoteLock.RUnlock()
	}
}
