/*
Copyright (C) 2021-2023 jlortiz

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

package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var voiceCooldown map[string]time.Time = make(map[string]time.Time)
var voicePrevious map[string]string = make(map[string]string)
var voiceStatement *sql.Stmt
var voiceStateLock sync.Mutex

const plusd = 3 * time.Second

func voiceStateUpdate(self *discordgo.Session, event *discordgo.VoiceStateUpdate) {
	voiceStateLock.Lock()
	defer voiceStateLock.Unlock()
	if event.ChannelID == "" || (event.BeforeUpdate != nil && event.BeforeUpdate.ChannelID == event.ChannelID) {
		if event.ChannelID == "" {
			delete(voicePrevious, event.UserID)
		}
		voiceCooldown[event.UserID] = time.Now().Add(plusd)
		return
	}
	if tim := voiceCooldown[event.UserID]; tim.After(time.Now()) {
		voiceCooldown[event.UserID] = time.Now().Add(plusd)
		return
	}
	mem := event.Member
	if mem.User.Bot {
		return
	}
	guild, err := self.State.Guild(event.GuildID)
	if err != nil {
		log.Error(fmt.Errorf("failed to get voice guild: %w", err))
		return
	}
	row := voiceStatement.QueryRow(event.GuildID, event.ChannelID)
	var output string
	specificVc := true
	err = row.Scan(&output)
	if err == sql.ErrNoRows {
		row = voiceStatement.QueryRow(event.GuildID, 0)
		err = row.Scan(&output)
		specificVc = false
	}
	if err != nil {
		return
	}
	_, err = self.State.Channel(output)
	if err != nil {
		if specificVc {
			commands.GetDatabase().Exec("DELETE FROM vachan WHERE gid=?001 AND vid=?002;", event.GuildID, event.ChannelID)
		} else {
			commands.GetDatabase().Exec("DELETE FROM vachan WHERE gid=?001 AND vid=?002;", event.GuildID, 0)
		}
		return
	}
	var msg *discordgo.Message
	if event.ChannelID == guild.AfkChannelID && event.BeforeUpdate != nil {
		if event.UserID == self.State.User.ID {
			self.VoiceConnections[event.GuildID].Disconnect()
			return
		}
		voicePrevious[event.UserID] = event.BeforeUpdate.ChannelID
		// TODO: Maybe don't do this if the channels aren't the same
		// Or add special handling to redirect to whatever the origin channel is if !specificVc?
		msg, err = self.ChannelMessageSend(output, event.Member.DisplayName()+" is now AFK")
	} else {
		old, ok := voicePrevious[event.UserID]
		if event.BeforeUpdate != nil && event.BeforeUpdate.ChannelID == guild.AfkChannelID && ok && event.ChannelID == old {
			msg, err = self.ChannelMessageSend(output, event.Member.DisplayName()+" is no longer AFK")
		} else {
			var vch *discordgo.Channel
			vch, err = self.State.Channel(event.ChannelID)
			if err != nil {
				log.Error(fmt.Errorf("failed to get current voice channel: %w", err))
				return
			}
			msg, err = self.ChannelMessageSend(output, event.Member.DisplayName()+" joined "+vch.Name)
		}
	}
	if err != nil {
		log.Error(fmt.Errorf("voice message failed: %w", err))
		return
	}
	voiceCooldown[event.UserID] = time.Now().Add(plusd)
	time.AfterFunc(2*time.Second, func() { self.ChannelMessageDelete(output, msg.ID) })
}

// ~!vachan [#channel]
// @GuildOnly
// Change voice join announcements
// Only people with the Manage Server permission can change the voice announcement channel.
// You must mention the channel to change the setting because I am lazy.
// You can disable voice join annoucements by setting it to "none" without quotes or pound.
func vachan(ctx *commands.Context) error {
	args := ctx.ApplicationCommandData().Options
	ch := args[0].ChannelValue(ctx.Bot)
	if len(args) == 1 {
		if ch.Type != discordgo.ChannelTypeGuildText {
			ctx.Database.Exec("DELETE FROM vachan WHERE gid=?;", ctx.GuildID)
			return ctx.RespondPrivate("Voice announcements disabled on this server.")
		}
		ctx.Database.Exec("INSERT OR REPLACE INTO vachan (gid, vid, cid) VALUES(?001, 0, ?002);", ctx.GuildID, ch.ID)
		return ctx.RespondPrivate("Voice joins will be announced in <#" + ch.ID + "> by default")
	}
	vc := ctx.ApplicationCommandData().Options[1].ChannelValue(ctx.Bot)
	if ch.Type != discordgo.ChannelTypeGuildText {
		ctx.Database.Exec("DELETE FROM vachan WHERE gid=?001 AND vid=?002;", ctx.GuildID, vc.ID)
		return ctx.RespondPrivate("Voice announcements disabled for <#" + vc.ID + ">")
	}
	ctx.Database.Exec("INSERT OR REPLACE INTO vachan (gid, vid, cid) VALUES(?001, ?002, ?003);", ctx.GuildID, vc.ID, ch.ID)
	return ctx.RespondPrivate("Voice joins for <#" + vc.ID + "> will be announced in <#" + ch.ID + ">")
}

// TODO: Do I even need this anymore?
func newGuild(self *discordgo.Session, event *discordgo.GuildCreate) {
	self.State.GuildAdd(event.Guild)
	self.RequestGuildMembers(event.ID, "", 250, "", false)
}

func oldGuild(self *discordgo.Session, event *discordgo.GuildDelete) {
	if !event.Unavailable {
		gid, _ := strconv.ParseUint(event.ID, 10, 64)
		commands.GetDatabase().Exec("DELETE FROM vachan WHERE gid=?;", gid)
	}
}
