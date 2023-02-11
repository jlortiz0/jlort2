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
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var voiceCooldown map[string]time.Time = make(map[string]time.Time)
var voicePrevious map[string]string = make(map[string]string)
var voiceAnnounce map[string]string
var dirty bool
var voiceStateLock *sync.Mutex = new(sync.Mutex)
var voiceSettingLock *sync.RWMutex = new(sync.RWMutex)

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
	mem, err := self.State.Member(event.GuildID, event.UserID)
	if err != nil {
		log.Warn(fmt.Sprintf("voice member not cached: %s", err.Error()))
		mem, err = self.GuildMember(event.GuildID, event.UserID)
		if err != nil {
			log.Error(fmt.Errorf("failed to get voice member: %w", err))
			return
		}
	}
	if mem.User.Bot {
		return
	}
	guild, err := self.State.Guild(event.GuildID)
	if err != nil {
		log.Error(fmt.Errorf("failed to get voice guild: %w", err))
		return
	}
	voiceSettingLock.RLock()
	output, ok := voiceAnnounce[event.GuildID]
	voiceSettingLock.RUnlock()
	_, err = self.State.Channel(output)
	if !ok || err != nil {
		return
	}
	displayname := commands.DisplayName(mem)
	var msg *discordgo.Message
	if event.ChannelID == guild.AfkChannelID && event.BeforeUpdate != nil {
		if event.UserID == self.State.User.ID {
			self.VoiceConnections[event.GuildID].Disconnect()
			return
		}
		voicePrevious[event.UserID] = event.BeforeUpdate.ChannelID
		msg, err = self.ChannelMessageSend(output, displayname+" is now AFK")
	} else {
		old, ok := voicePrevious[event.UserID]
		if event.BeforeUpdate != nil && event.BeforeUpdate.ChannelID == guild.AfkChannelID && ok && event.ChannelID == old {
			msg, err = self.ChannelMessageSend(output, displayname+" is no longer AFK")
		} else {
			var vch *discordgo.Channel
			vch, err = self.State.Channel(event.ChannelID)
			if err != nil {
				log.Error(fmt.Errorf("failed to get current voice channel: %w", err))
				return
			}
			msg, err = self.ChannelMessageSend(output, displayname+" joined "+vch.Name)
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
// Allows you to see or change the channel where voice joins will be announced.
// Only people with the Manage Server permission can change the voice announcement channel.
// You must mention the channel to change the setting because I am lazy.
// You can disable voice join annoucements by setting it to "none" without quotes or pound.
func vachan(ctx commands.Context) error {
	args := ctx.ApplicationCommandData().Options
	if len(args) != 0 {
		perms, err := ctx.State.UserChannelPermissions(ctx.User.ID, ctx.ChannelID)
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageServer == 0 {
			return ctx.Respond("You need the Manage Server permission to change this setting.")
		}
		voiceSettingLock.Lock()
		defer voiceSettingLock.Unlock()
		arg := args[0].ChannelValue(ctx.Bot)
		if arg.Type != discordgo.ChannelTypeGuildText {
			delete(voiceAnnounce, ctx.GuildID)
			dirty = true
			return ctx.Respond("Voice announcements disabled.")
		}
		voiceAnnounce[ctx.GuildID] = arg.ID
		dirty = true
		err = ctx.Respond(fmt.Sprintf("Voice joins will be announced in <#%s>", arg.ID))
		return err
	}
	chID, ok := voiceAnnounce[ctx.GuildID]
	if !ok {
		return ctx.Respond("Voice announcements are disabled on this server")
	}
	return ctx.Respond(fmt.Sprintf("Voice joins will be announced in <#%s>", chID))
}

func newGuild(self *discordgo.Session, event *discordgo.GuildCreate) {
	self.State.GuildAdd(event.Guild)
	if _, ok := notForThisOne[event.ID]; ok {
		self.RequestGuildMembers(event.ID, "", 250, "", false)
		return
	}
	time.Sleep(10 * time.Millisecond)
	// event.Guild, _ = self.State.Guild(event.ID)
	notForThisOne[event.ID] = struct{}{}
	var chanID string
	for _, v := range event.Channels {
		if v.Type == discordgo.ChannelTypeGuildText {
			perms, err := self.State.UserChannelPermissions(self.State.User.ID, v.ID)
			if err == nil && perms&discordgo.PermissionSendMessages != 0 {
				chanID = v.ID
				break
			}
		}
	}
	if chanID != "" {
		_, err := self.ChannelMessageSend(chanID, "Hello!\nTo manage automatic voice announcements, do /vachan\nTo set the DJ role, do /dj")
		if err != nil {
			log.Error(err)
			return
		}
	}
}

func saveVoice() error {
	if !dirty {
		return nil
	}
	voiceSettingLock.RLock()
	err := commands.SavePersistent("vachan", &voiceAnnounce)
	if err == nil {
		dirty = false
	}
	voiceSettingLock.RUnlock()
	return err
}
