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

const quotes_max = 200

var quixote map[string][]string
var dirty bool
var quoteLock *sync.RWMutex = new(sync.RWMutex)

// ~!quote [index]
// @GuildOnly
// Gets a random quote
// If you specifiy an index, it will try to get that quote.
// Indices are the numbers beside a quote in ~!quote or the line number of a quote in ~!quotes
func quote(ctx *commands.Context) error {
	quoteLock.RLock()
	qList := quixote[ctx.GuildID]
	quoteLock.RUnlock()
	if len(qList) == 0 {
		return ctx.RespondPrivate("There are no quotes. Use /addquote to add some.")
	}
	var sel int
	args := ctx.ApplicationCommandData().Options
	if len(args) > 0 {
		sel = int(args[0].IntValue())
		if sel < 1 || sel > len(qList) {
			return ctx.RespondPrivate("Index out of bounds, expected 1-" + strconv.Itoa(len(qList)))
		}
		sel--
	} else {
		sel = rand.Intn(len(qList))
	}
	ctx.SetComponents(discordgo.Button{Emoji: discordgo.ComponentEmoji{Name: "\U0001f3b2"}, CustomID: strconv.Itoa(sel)})
	return ctx.Respond(fmt.Sprintf("%d. %s", sel+1, qList[sel]))
}

func quoteReroll(ctx *commands.Context) error {
	quoteLock.RLock()
	qList := quixote[ctx.GuildID]
	quoteLock.RUnlock()
	if len(qList) == 0 {
		return ctx.RespondPrivate("There are no quotes. Use /addquote to add some.")
	}
	sel2, _ := strconv.Atoi(ctx.MessageComponentData().CustomID)
	sel := sel2
	for sel == sel2 {
		sel = rand.Intn(len(qList))
	}
	ctx.SetComponents(discordgo.Button{Emoji: discordgo.ComponentEmoji{Name: "\U0001f3b2"}, CustomID: strconv.Itoa(sel)})
	return ctx.Respond(fmt.Sprintf("%d. %s", sel+1, qList[sel]))
}

const quotes_paginate_amount = 10

// ~!quotes
// @GuildOnly
// Gets all quotes
func quotes(ctx *commands.Context) error {
	var ind int
	if ctx.Type == discordgo.InteractionMessageComponent {
		cid := ctx.MessageComponentData().CustomID
		ind, _ = strconv.Atoi(cid[1:])
		if cid[0] == 'r' {
			ind += quotes_paginate_amount
		} else {
			ind -= quotes_paginate_amount
		}
	}
	quoteLock.RLock()
	defer quoteLock.RUnlock()
	qList := quixote[ctx.GuildID]
	if len(qList) == 0 {
		return ctx.RespondPrivate("There are no quotes. Use /addquote to add some.")
	}
	if ind >= len(qList) {
		ind = len(qList) - quotes_paginate_amount
	} else if ind < 0 {
		ind = 0
	}
	guild, err := ctx.State.Guild(ctx.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}
	output := new(discordgo.MessageEmbed)
	output.Title = "Quotes from " + guild.Name
	output.Description = buildQuotesString(qList, ind)
	output.Color = 0x7289da
	ctx.SetComponents(discordgo.Button{CustomID: "l" + strconv.Itoa(ind), Disabled: ind == 0, Emoji: discordgo.ComponentEmoji{Name: "\u2B05"}, Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: "r" + strconv.Itoa(ind), Emoji: discordgo.ComponentEmoji{Name: "\u27A1"}, Disabled: len(qList) <= ind+quotes_paginate_amount, Style: discordgo.SecondaryButton})
	err = ctx.RespondEmbed(output, false)
	return err
}

func buildQuotesString(qList []string, start int) string {
	if start >= len(qList) {
		return ""
	}
	qList = qList[start:]
	if len(qList) > quotes_paginate_amount {
		qList = qList[:quotes_paginate_amount]
	}
	builder := new(strings.Builder)
	for _, v := range qList {
		start++
		builder.WriteString(strconv.Itoa(start))
		builder.WriteString(". ")
		builder.WriteString(v)
		builder.WriteByte('\n')
	}
	return builder.String()[:builder.Len()-1]
}

// ~!addquote <quote>
// @GuildOnly
// Adds a quote
func addquote(ctx *commands.Context) error {
	dirty = true
	quoteLock.Lock()
	if len(quixote[ctx.GuildID]) >= quotes_max {
		quoteLock.Unlock()
		return ctx.RespondPrivate("Maximum number of quotes reached.")
	}
	quixote[ctx.GuildID] = append(quixote[ctx.GuildID], ctx.ApplicationCommandData().Options[0].StringValue())
	quoteLock.Unlock()
	return ctx.RespondPrivate("Quote added.")
}

// ~!delquote <index>
// @Alias rmquote
// @Alias removequote
// @GuildOnly
// Removes a quote
// If you have Manage Server, you can do ~!delquote all to remove all quotes.
// Indices are the numbers beside a quote in ~!quote or the line number of a quote in ~!quotes
func delquote(ctx *commands.Context) error {
	quoteLock.Lock()
	defer quoteLock.Unlock()
	qList := quixote[ctx.GuildID]
	if len(qList) == 0 {
		return ctx.RespondPrivate("There are no quotes. Use /addquote to add some.")
	}
	sel := int(ctx.ApplicationCommandData().Options[0].IntValue())
	if sel < 0 {
		perms, err := ctx.State.UserChannelPermissions(ctx.User.ID, ctx.ChannelID)
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageMessages == 0 {
			return ctx.RespondPrivate("You need the Manage Messages permission to clear all quotes.")
		}
		delete(quixote, ctx.GuildID)
		dirty = true
		return ctx.RespondPrivate("All quotes removed.")
	}
	if sel == 0 || sel > len(qList) {
		return ctx.RespondPrivate("Index out of bounds, expected 1-" + strconv.Itoa(len(qList)))
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
	return ctx.RespondPrivate("Quote removed.")
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
	commands.PrepareCommand("quote", "Hopefully it's actually funny").Guild().Component(quoteReroll).Register(quote, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("index", "Index of quote to show, default random").AsInt().SetMinMax(1, quotes_max).Finalize(),
	})
	commands.PrepareCommand("quotes", "Show all quotes").Guild().Component(quotes).Register(quotes, nil)
	commands.PrepareCommand("addquote", "Record that dumb thing your friend just said").Guild().Register(addquote, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("quote", "The thing, the funny thing").AsString().Required().Finalize(),
	})
	commands.PrepareCommand("delquote", "Guess it wasn't funny").Guild().Register(delquote, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("index", "Index of quote to remove").AsInt().SetMinMax(1, quotes_max).Required().Finalize(),
	})
	self.AddHandler(guildDelete)
	commands.RegisterSaver(saveQuotes)
}

func saveQuotes() error {
	if !dirty {
		return nil
	}
	quoteLock.RLock()
	err := commands.SavePersistent("quotes", &quixote)
	if err == nil {
		dirty = false
	}
	quoteLock.RUnlock()
	return err
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the quotes to disk.
func Cleanup(_ *discordgo.Session) {
	err := saveQuotes()
	if err != nil {
		log.Error(err)
	}
}
