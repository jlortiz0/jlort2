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
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

const quotes_max = 200
const quotes_paginate_amount = 10

var queryGetLen, queryGetInd *sql.Stmt

// ~!quote [index]
// @GuildOnly
// Gets a random quote
// If you specifiy an index, it will try to get that quote.
// Indices are the numbers beside a quote in ~!quote or the line number of a quote in ~!quotes
func quote(ctx *commands.Context) error {
	tx, err := ctx.Database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := tx.Stmt(queryGetLen).QueryRow(gid)
	var total int
	result.Scan(&total)
	if total == 0 {
		return ctx.RespondPrivate("There are no quotes. Use /addquote to add some.")
	}
	var sel int
	args := ctx.ApplicationCommandData().Options
	if len(args) > 0 {
		sel = int(args[0].IntValue())
		if sel < 1 || sel > total {
			return ctx.RespondPrivate("Index out of bounds, expected 1-" + strconv.Itoa(total))
		}
	} else {
		sel = rand.Intn(total) + 1
	}
	ctx.SetComponents(discordgo.Button{Emoji: discordgo.ComponentEmoji{Name: "\U0001f3b2"}, CustomID: strconv.Itoa(sel)})
	result = tx.Stmt(queryGetInd).QueryRow(gid, sel)
	var q string
	result.Scan(&q)
	return ctx.Respond(fmt.Sprintf("%d. %s", sel, q))
}

func quoteReroll(ctx *commands.Context) error {
	tx, err := ctx.Database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := tx.Stmt(queryGetLen).QueryRow(gid)
	var total int
	result.Scan(&total)
	if total == 0 {
		return ctx.RespondPrivate("There are no quotes. Use /addquote to add some.")
	}
	sel2, _ := strconv.Atoi(ctx.MessageComponentData().CustomID)
	if total == 1 && sel2 == 1 {
		return ctx.RespondEmpty()
	}
	sel := sel2
	for sel == sel2 {
		sel = rand.Intn(total) + 1
	}
	ctx.SetComponents(discordgo.Button{Emoji: discordgo.ComponentEmoji{Name: "\U0001f3b2"}, CustomID: strconv.Itoa(sel)})
	result = tx.Stmt(queryGetInd).QueryRow(gid, sel)
	var q string
	result.Scan(&q)
	return ctx.Respond(fmt.Sprintf("%d. %s", sel, q))
}

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
	tx, err := ctx.Database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := tx.Stmt(queryGetLen).QueryRow(gid)
	var total int
	result.Scan(&total)
	if total == 0 {
		return ctx.RespondPrivate("There are no quotes. Use /addquote to add some.")
	}
	if ind >= total {
		ind = total - quotes_paginate_amount
	} else if ind < 0 {
		ind = 0
	}
	guild, err := ctx.State.Guild(ctx.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}
	results, err := tx.Query("SELECT ind, quote FROM quotes WHERE gid=?001 ORDER BY ind LIMIT ?002 OFFSET ?003;", gid, quotes_paginate_amount, ind)
	if err != nil {
		return err
	}
	builder := new(strings.Builder)
	var i int
	var v string
	for results.Next() {
		results.Scan(&i, &v)
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(". ")
		builder.WriteString(v)
		builder.WriteByte('\n')
	}
	output := new(discordgo.MessageEmbed)
	output.Title = "Quotes from " + guild.Name
	output.Description = builder.String()[:builder.Len()-1]
	output.Color = 0x7289da
	if total > quotes_paginate_amount {
		ctx.SetComponents(discordgo.Button{CustomID: "l" + strconv.Itoa(ind), Disabled: ind == 0, Emoji: discordgo.ComponentEmoji{Name: "\u2B05"}, Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: "r" + strconv.Itoa(ind), Emoji: discordgo.ComponentEmoji{Name: "\u27A1"}, Disabled: total <= ind+quotes_paginate_amount, Style: discordgo.SecondaryButton})
	}
	err = ctx.RespondEmbed(output, false)
	return err
}

// ~!addquote <quote>
// @GuildOnly
// Adds a quote
func addquote(ctx *commands.Context) error {
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := queryGetLen.QueryRow(gid)
	var total int
	result.Scan(&total)
	if total >= quotes_max {
		return ctx.RespondPrivate("Maximum number of quotes reached.")
	}
	ctx.Database.Exec("INSERT INTO quotes (gid, ind, quote) SELECT ?001, COUNT(*) + 1, ?002 FROM quotes WHERE gid=?001;", gid, ctx.ApplicationCommandData().Options[0].StringValue())
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
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := queryGetLen.QueryRow(gid)
	var total int
	result.Scan(&total)
	if total == 0 {
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
		ctx.Database.Exec("DELETE FROM quotes WHERE gid=?001;", gid)
		return ctx.RespondPrivate("All quotes removed.")
	}
	if sel == 0 || sel > total {
		return ctx.RespondPrivate("Index out of bounds, expected 1-" + strconv.Itoa(total))
	}
	if sel == total {
		ctx.Database.Exec("DELETE FROM quotes WHERE gid = ?001 AND ind = ?002;", gid, sel)
		return ctx.RespondPrivate("Quote removed.")
	}
	tx, err := ctx.Database.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT OR REPLACE INTO quotes SELECT ?001, ind - 1, quote FROM quotes WHERE gid=?001 AND ind > ?002 ORDER BY ind ASC;", gid, sel)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.Exec("DELETE FROM quotes WHERE gid = ?001 AND ind = ?002;", gid, total)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return ctx.RespondPrivate("Quote removed.")
}

func guildDelete(_ *discordgo.Session, event *discordgo.GuildDelete) {
	if !event.Unavailable {
		gid, _ := strconv.ParseUint(event.ID, 10, 64)
		commands.GetDatabase().Exec("DELETE FROM quotes WHERE gid=?001;", gid)
	}
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also loads the quotes from disk.
func Init(self *discordgo.Session) {
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
	db := commands.GetDatabase()
	var err error
	queryGetLen, err = db.Prepare("SELECT COUNT(*) FROM quotes WHERE gid=?001;")
	if err != nil {
		log.Error(err)
		return
	}
	queryGetInd, err = db.Prepare("SELECT quote FROM quotes WHERE gid=?001 AND ind=?002;")
	if err != nil {
		log.Error(err)
	}
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the quotes to disk.
func Cleanup(_ *discordgo.Session) {
	queryGetLen.Close()
	queryGetInd.Close()
}
