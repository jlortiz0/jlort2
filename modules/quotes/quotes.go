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

var queryGetLen, queryGetInd *sql.Stmt

// ~!quote [index]
// @GuildOnly
// Gets a random quote
// If you specifiy an index, it will try to get that quote.
// Indices are the numbers beside a quote in ~!quote or the line number of a quote in ~!quotes
func quote(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := queryGetLen.QueryRow(gid)
	var total int
	result.Scan(&total)
	if total == 0 {
		return ctx.Send("There are no quotes. Use ~!addquote to add some.")
	}
	var sel int
	var err error
	if len(args) > 0 {
		sel, err = strconv.Atoi(args[0])
		if err != nil {
			return ctx.Send("Usage: ~!quote <index>")
		}
		if sel < 1 || sel > total {
			return ctx.Send("Index out of bounds, expected 1-" + strconv.Itoa(total))
		}
	} else {
		sel = rand.Intn(total) + 1
	}
	result = queryGetInd.QueryRow(gid, sel)
	var q string
	result.Scan(&q)
	return ctx.Send(fmt.Sprintf("%d. %s", sel, q))
}

// ~!quotes
// @GuildOnly
// Gets all quotes
func quotes(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	results, err := ctx.Database.Query("SELECT ind, quote FROM quotes WHERE gid=?001;", gid)
	if err != nil {
		return err
	}
	if !results.Next() {
		return ctx.Send("There are no quotes. Use ~!addquote to add some.")
	}
	guild, err := ctx.State.Guild(ctx.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}
	output := new(discordgo.MessageEmbed)
	output.Title = "Quotes from " + guild.Name
	builder := new(strings.Builder)
	var i int
	var v string
	for {
		results.Scan(&i, &v)
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(". ")
		builder.WriteString(v)
		builder.WriteByte('\n')
		if !results.Next() {
			break
		}
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
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	ctx.Database.Exec("INSERT INTO quotes (gid, ind, quote) SELECT ?001, COUNT(*) + 1, ?002 FROM quotes WHERE gid=?001;", gid, data[ind:])
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
	if len(args) == 0 {
		return ctx.Send("Usage: ~!delquote <index>\nUse ~!quotes to see indexes.")
	}
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := queryGetLen.QueryRow(gid)
	var total int
	result.Scan(&total)
	if total == 0 {
		return ctx.Send("There are no quotes. Use ~!addquote to add some.")
	}
	if args[0] == "all" {
		perms, err := ctx.State.MessagePermissions(ctx.Message)
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageServer == 0 {
			return ctx.Send("You need the Manage Server permission to clear all quotes.")
		}
		ctx.Database.Exec("DELETE FROM quotes WHERE gid=?001;", gid)
		return ctx.Send("All quotes removed.")
	}
	sel, err := strconv.Atoi(args[0])
	if err != nil {
		return ctx.Send(args[0] + " is not a number.")
	}
	if sel < 1 || sel > total {
		return ctx.Send("Index out of bounds, expected 1-" + strconv.Itoa(total))
	}
	if sel == total {
		ctx.Database.Exec("DELETE FROM quotes WHERE gid = ?001 AND ind = ?002;", gid, sel)
		return ctx.Send("Quote removed.")
	}
	tx, err := ctx.Database.Begin()
	if err != nil {
		return err
	}
	// TODO: There has to be a better way to do this
	_, err = tx.Exec(`
	CREATE TABLE quotes_temp (ind INTEGER, quote VARCHAR(512));
	INSERT INTO quotes_temp SELECT ind - 1, quote FROM quotes WHERE gid = ?001 AND ind > ?002;
	DELETE FROM quotes WHERE gid = ?001 AND ind >= ?002;
	INSERT OR ROLLBACK INTO quotes SELECT ?001, ind, quote FROM quotes_temp;
	DROP TABLE quotes_temp;
	`, gid, sel, gid, sel, gid)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return ctx.Send("Quote removed.")
}

func guildDelete(_ *discordgo.Session, event *discordgo.GuildDelete) {
	gid, _ := strconv.ParseUint(event.ID, 10, 64)
	commands.GetDatabase().Exec("DELETE FROM quotes WHERE gid=?001;", gid)
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also loads the quotes from disk.
func Init(self *discordgo.Session) {
	commands.RegisterCommand(quote, "quote")
	commands.RegisterCommand(quotes, "quotes")
	commands.RegisterCommand(addquote, "addquote")
	commands.RegisterCommand(delquote, "removequote")
	commands.RegisterCommand(delquote, "rmquote")
	commands.RegisterCommand(delquote, "delquote")
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
