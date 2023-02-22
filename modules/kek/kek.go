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
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var kekData struct {
	Guilds map[string]struct{}
	Users  map[string]map[string]int
}
var dirty bool
var kekLock *sync.RWMutex = new(sync.RWMutex)

// ~!kekage [user]
// Checks someone's kekage
// If not specified, gives the kekage of the command runner.
func kekage(ctx *commands.Context) error {
	target := ctx.User
	if len(ctx.ApplicationCommandData().Options) > 0 && ctx.GuildID != "" {
		target = ctx.ApplicationCommandData().Options[0].UserValue(ctx.Bot)
	}
	if target.Bot {
		return ctx.RespondPrivate("Bots can't be kek.")
	}
	name := target.Username
	kekI := 0
	kekLock.RLock()
	for _, v := range kekData.Users[target.ID] {
		kekI += v
	}
	kekLock.RUnlock()
	kekI *= 50
	var msg string
	if kekI == 0 {
		return ctx.Respond(name + " is in perfect harmony between kek and cringe.\nAll they have to do now is turn off thier computer and get a life.")
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
	return ctx.RespondPrivate(fmt.Sprintf(msg, name, convertKek(kekI)))
}

// ~!kekReport
// @GuildOnly
// Gets the kekage of everyone
func kekReport(ctx *commands.Context) error {
	guild, err := ctx.State.Guild(ctx.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}
	output := new(strings.Builder)
	kekLock.RLock()
	for _, mem := range guild.Members {
		if mem.User.Bot {
			continue
		}
		name := commands.DisplayName(mem)
		kekI := 0
		for _, v := range kekData.Users[mem.User.ID] {
			kekI += v
		}
		kekI *= 50
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
	kekLock.RUnlock()
	if output.Len() == 0 {
		output.WriteString("All keks are zero.")
	}
	return ctx.RespondPrivate(output.String())
}

// ~!kekOn
// @GuildOnly
// @ManageServer
// Toggles kekage on a server
// You must have Manage Server to do this.
func kekOn(ctx *commands.Context) error {
	dirty = true
	kekLock.Lock()
	defer kekLock.Unlock()
	if ctx.ApplicationCommandData().Options[0].BoolValue() {
		kekData.Guilds[ctx.GuildID] = struct{}{}
		return ctx.RespondPrivate("Kek enabled on this server.")
	}
	delete(kekData.Guilds, ctx.GuildID)
	return ctx.RespondPrivate("Kek disabled on this server.")
}

func onMessageKek(self *discordgo.Session, event *discordgo.MessageCreate) {
	if _, ok := kekData.Guilds[event.GuildID]; !ok || event.Author.Bot {
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
	if event.Emoji.Name[:3] != "\u2b06" && event.Emoji.Name[:3] != "\u2b07" {
		return
	}
	if _, ok := kekData.Guilds[event.GuildID]; !ok || event.UserID == self.State.User.ID {
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
		if emoji.Emoji.Name[:3] == "\u2b06" {
			total += emoji.Count
		} else if emoji.Emoji.Name[:3] == "\u2b07" {
			total -= emoji.Count
		}
	}
	dirty = true
	kekLock.Lock()
	if kekData.Users[msg.Author.ID] == nil {
		kekData.Users[msg.Author.ID] = make(map[string]int)
	}
	kekData.Users[msg.Author.ID][msg.ID] = total
	kekLock.Unlock()
}

func onReactionRemoveWrapper(self *discordgo.Session, event *discordgo.MessageReactionRemove) {
	onReactionAdd(self, &discordgo.MessageReactionAdd{MessageReaction: event.MessageReaction})
}

func onReactionRemoveAllWrapper(self *discordgo.Session, event *discordgo.MessageReactionRemoveAll) {
	onReactionAdd(self, &discordgo.MessageReactionAdd{MessageReaction: event.MessageReaction})
}

func onGuildRemoveKek(self *discordgo.Session, event *discordgo.GuildDelete) {
	kekLock.Lock()
	delete(kekData.Guilds, event.ID)
	dirty = true
	kekLock.Unlock()
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
	err := commands.LoadPersistent("kek", &kekData)
	if err != nil {
		log.Error(err)
		return
	}
	cutoff := time.Now()
	for _, keks := range kekData.Users {
		total := 0
		for k, v := range keks {
			if k == "locked" {
				continue
			}
			ts, _ := discordgo.SnowflakeTimestamp(k)
			if ts.AddDate(0, 0, 4).Before(cutoff) {
				total += v
				delete(keks, k)
				dirty = true
			}
		}
		keks["locked"] += total
	}
	commands.PrepareCommand("kek", "Kek or cringe with "+self.State.Application.Name).Register(kekage, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("user", "Person to check the kekage of, default you").AsUser().Finalize(),
	})
	commands.PrepareCommand("kekreport", "Reddit Recap for everyone").Guild().Register(kekReport, nil)
	commands.PrepareCommand("kekenabled", "Enable or disable kek on this server").Guild().Perms(
		discordgo.PermissionManageServer).Register(kekOn, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("enable", "Should kek be enabled on this server?").AsBool().Required().Finalize()})
	commands.RegisterSaver(saveKek)
	self.AddHandler(onMessageKek)
	self.AddHandler(onReactionAdd)
	self.AddHandler(onReactionRemoveWrapper)
	self.AddHandler(onReactionRemoveAllWrapper)
	self.AddHandler(onGuildRemoveKek)
}

func saveKek() error {
	if !dirty {
		return nil
	}
	kekLock.Lock()
	for _, keks := range kekData.Users {
		for k, v := range keks {
			if k == "locked" {
				continue
			}
			if v == 0 {
				delete(keks, k)
			}
		}
	}
	err := commands.SavePersistent("kek", &kekData)
	if err == nil {
		dirty = false
	}
	kekLock.Unlock()
	return err
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the kek data to disk.
func Cleanup(_ *discordgo.Session) {
	err := saveKek()
	if err != nil {
		log.Error(err)
	}
}
