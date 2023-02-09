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

package kek

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var botId string
var kekEnabledQuery *sql.Stmt
var kekMsgSet *sql.Stmt
var kekGet *sql.Stmt

// ~!kekage [user]
// Checks someone's kekage
// If not specified, gives the kekage of the command runner.
func kekage(ctx commands.Context, args []string) error {
	target := ctx.Member
	var err error
	if len(args) > 0 && ctx.GuildID != "" {
		name := strings.Join(args, " ")
		target, err = commands.FindMember(ctx.Bot, name, ctx.GuildID)
		if err != nil {
			return err
		}
		if target == nil {
			return ctx.Send("No such member " + name)
		}
	}
	if target.User.Bot {
		return ctx.Send("Bots can't be kek.")
	}
	name := commands.DisplayName(target)
	var kekI int
	uid, _ := strconv.ParseUint(ctx.Author.ID, 10, 64)
	result := kekGet.QueryRow(uid)
	result.Scan(&kekI)
	kekI *= 50
	var msg string
	if kekI == 0 {
		return ctx.Send(name + " is in perfect harmony between kek and cringe.\nAll they have to do now is turn off thier computer and get a life.")
	} else if kekI < 0 {
		if kekI > -1000 {
			msg = "%s is at %s cringe.\nThey should be wary, lest they falter further."
		} else if kekI > -1000000 {
			msg = "%s is at %s cringe.\nEvil spirits are strengthening from their presence."
		} else {
			msg = "%s is at %s cringe.\nAnton Chigurh has offered to kill them for free."
		}
	} else {
		if kekI < 1000 {
			msg = "%s is at %s kek.\nThey are but starting on the path to enlightenment."
		} else if kekI < 1000000 {
			msg = "%s is at %s kek.\nSomething great stirs within them."
		} else {
			msg = "%s is at %s kek.\nThey are blessed with the power of good vibes."
		}
	}
	return ctx.Send(fmt.Sprintf(msg, name, convertKek(kekI)))
}

// ~!kekReport
// @GuildOnly
// Gets the kekage of everyone
func kekReport(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	guild, err := ctx.State.Guild(ctx.GuildID)
	if err != nil {
		return fmt.Errorf("Failed to get guild: %w", err)
	}
	output := new(strings.Builder)
	tx, err := ctx.Database.Begin()
	if err != nil {
		return err
	}
	defer tx.Commit()
	stmt := tx.Stmt(kekGet)
	defer stmt.Close()
	for _, mem := range guild.Members {
		if mem.User.Bot {
			continue
		}
		name := commands.DisplayName(mem)
		var kekI int
		uid, _ := strconv.ParseUint(ctx.Author.ID, 10, 64)
		result := kekGet.QueryRow(uid)
		result.Scan(&kekI)
		if kekI != 0 {
			output.WriteString(name)
			output.WriteString(": ")
			if kekI < 0 {
				output.WriteByte('-')
			}
			output.WriteString(convertKek(kekI))
			output.WriteByte('\n')
		}
	}
	return ctx.Send(output.String())
}

// ~!kekOn
// @GuildOnly
// Toggles kekage on a server
// You must have Manage Server to do this.
func kekOn(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	perms, err := ctx.State.MessagePermissions(ctx.Message)
	if err != nil {
		return fmt.Errorf("Failed to get permissions: %w", err)
	}
	if perms&discordgo.PermissionManageServer == 0 {
		return ctx.Send("You need the Manage Server permission to toggle kek.")
	}
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	result := kekEnabledQuery.QueryRow(gid)
	if result.Scan() != nil {
		ctx.Database.Exec("INSERT INTO kekGuilds VALUES (?);", gid)
		return ctx.Send("Kek enabled on this server.")
	}
	ctx.Database.Exec("DELETE FROM kekGuilds WHERE gid=?;", gid)
	return ctx.Send("Kek disabled on this server.")
}

func onMessageKek(self *discordgo.Session, event *discordgo.MessageCreate) {
	gid, _ := strconv.ParseUint(event.GuildID, 10, 64)
	if event.Author.Bot || kekEnabledQuery.QueryRow(gid).Scan() != nil {
		return
	}
	perms, err := self.State.UserChannelPermissions(self.State.User.ID, event.ChannelID)
	if err != nil || perms&discordgo.PermissionAddReactions == 0 {
		return
	}
	vote := false
	for _, embed := range event.Embeds {
		if embed.Image != nil || embed.Video != nil {
			vote = true
			break
		}
	}
	if !vote {
		for _, attach := range event.Attachments {
			if attach.Height > 0 {
				vote = true
				break
			}
		}
	}
	if vote {
		//kekData.Users[event.Author.ID][event.ID] = 0
		self.MessageReactionAdd(event.ChannelID, event.Message.ID, "\u2b06")
		self.MessageReactionAdd(event.ChannelID, event.Message.ID, "\u2b07")
	}
}

func onReactionAdd(self *discordgo.Session, event *discordgo.MessageReactionAdd) {
	if event.Emoji.Name != "\u2b06" && event.Emoji.Name != "\u2b07" {
		return
	}
	gid, _ := strconv.ParseUint(event.GuildID, 10, 64)
	if event.UserID == botId || kekEnabledQuery.QueryRow(gid).Scan() != nil {
		return
	}
	msg, err := self.ChannelMessage(event.ChannelID, event.MessageID)
	if err != nil {
		return
	}
	if msg.Timestamp.AddDate(0, 0, 4).Before(time.Now()) {
		return
	}
	total := 0
	for _, emoji := range msg.Reactions {
		if emoji.Emoji.Name == "\u2b06" {
			total += emoji.Count
		} else if emoji.Emoji.Name == "\u2b07" {
			total -= emoji.Count
		}
	}
	uid, _ := strconv.ParseUint(msg.Author.ID, 10, 64)
	mid, _ := strconv.ParseUint(msg.ID, 10, 64)
	kekMsgSet.Exec(uid, mid, total)
}

func onReactionRemoveWrapper(self *discordgo.Session, event *discordgo.MessageReactionRemove) {
	onReactionAdd(self, &discordgo.MessageReactionAdd{MessageReaction: event.MessageReaction})
}

func onReactionRemoveAllWrapper(self *discordgo.Session, event *discordgo.MessageReactionRemoveAll) {
	onReactionAdd(self, &discordgo.MessageReactionAdd{MessageReaction: event.MessageReaction})
}

func onGuildRemoveKek(self *discordgo.Session, event *discordgo.GuildDelete) {
	gid, _ := strconv.ParseUint(event.ID, 10, 64)
	commands.GetDatabase().Exec("DELETE FROM kekGuilds WHERE gid=?;", gid)
}

func convertKek(kek int) string {
	if kek < 0 {
		kek = -kek
	}
	kekF := float64(kek)
	if kekF < 1000 {
		return strconv.Itoa(kek)
	} else if kek < 1000000 {
		kekF /= 1000
		return fmt.Sprintf("%.1fK", kekF)
	} else {
		kekF /= 1000000
		return fmt.Sprintf("%.1fM", kekF)
	}
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also initializes the cooldown and duel maps and loads the kek data from disk, as well as collapsing old kek data.
func Init(self *discordgo.Session) {
	commands.RegisterCommand(kekage, "kekage")
	commands.RegisterCommand(kekOn, "kekOn")
	commands.RegisterCommand(kekOn, "kekOff")
	commands.RegisterCommand(kekReport, "kekReport")
	self.AddHandler(onMessageKek)
	self.AddHandler(onReactionAdd)
	self.AddHandler(onReactionRemoveWrapper)
	self.AddHandler(onReactionRemoveAllWrapper)
	self.AddHandler(onGuildRemoveKek)
	u, err := self.User("@me")
	if err != nil {
		log.Error(err)
	}
	botId = u.ID

	db := commands.GetDatabase()
	kekEnabledQuery, err = db.Prepare("SELECT gid FROM kekGuilds WHERE gid=?;")
	if err != nil {
		log.Error(err)
		return
	}
	kekMsgSet, err = db.Prepare(`
	INSERT OR IGNORE INTO kekUsers (uid) VALUES (?001);
	INSERT INTO kekMsgs (uid, mid, score) VALUES (?001, ?002, ?003)
	ON CONFLICT DO UPDATE SET score=excluded.score;
	`)
	if err != nil {
		log.Error(err)
	}
	kekGet, err = db.Prepare("SELECT u.score + SUM(m.score) FROM kekUsers u, kekMsgs m WHERE u.uid = ?001 AND u.uid = m.uid;")
	if err != nil {
		log.Error(err)
	}
	snowflake := uint64(time.Now().AddDate(0, 0, -4).UnixMilli()) - 1420070400000
	snowflake <<= 22
	db.Exec(`
	UPDATE kekUsers SET score = score + m.total FROM (
		SELECT uid, SUM(score) total FROM kekMsgs
		WHERE mid < ?001
		GROUP BY uid
	) m WHERE m.uid = kekUsers.uid;
	DELETE FROM kekMsgs WHERE mid < ?001;
	`, snowflake)
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the kek data to disk.
func Cleanup(_ *discordgo.Session) {
	commands.GetDatabase().Exec("DELETE FROM kekMsgs WHERE score=0;")
	kekEnabledQuery.Close()
	kekMsgSet.Close()
	kekGet.Close()
}
