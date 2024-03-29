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

package music

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

// ~!remove <queue index>
// @Alias rm
// @GuildOnly
// Removes from the queue
// You must have permission over the stream to do so. Get stream indices with ~!queue.
// To clear the queue, do ~!remove all. You must have permission over all streams to do so.
func remove(ctx *commands.Context) error {
	ls := streams[ctx.GuildID]
	if ls == nil || ls.Len() < 2 {
		return ctx.RespondPrivate("Nothing in the queue.")
	}
	ind := int(ctx.ApplicationCommandData().Options[0].IntValue())
	if ind < 0 {
		perms, _ := ctx.State.UserChannelPermissions(ctx.User.ID, ctx.ChannelID)
		permitted := perms&discordgo.PermissionManageServer != 0
		if !permitted {
			gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
			result := queryDj.QueryRow(gid)
			var DJ string
			result.Scan(&DJ)
			if DJ != "" {
				for _, v := range ctx.Member.Roles {
					if v == DJ {
						permitted = true
						break
					}
				}
			}
		}
		if !permitted {
			ls.RLock()
			for e := ls.Head(); e != nil; e = e.Next() {
				current := e.Value
				if current.Flags&strflag_noskip != 0 || ctx.User.ID != current.Author {
					ls.RUnlock()
					return ctx.RespondPrivate("You do not have permission to clear the queue.")
				}
			}
			ls.RUnlock()
		}
		ls.Lock()
		for ls.Len() > 1 {
			ls.Remove(ls.Tail())
		}
		ls.Unlock()
		if ind != -5738 {
			return ctx.Respond("Queue cleared.")
		}
		return nil
	}
	if ind < 1 || ind >= ls.Len() {
		return ctx.RespondPrivate("Index out of bounds, expected 1-" + strconv.Itoa(ls.Len()-1))
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, ind) {
		return ctx.RespondPrivate("You do not have permission to modify that stream.")
	}
	ls.Lock()
	elem := ls.Head()
	for i := 0; i < ind; i++ {
		elem = elem.Next()
	}
	ls.Remove(elem)
	ls.Unlock()
	obj := elem.Value
	return ctx.Respond("Removed " + obj.Info.Title + ".")
}

// ~!loop <enabled>
// @GuildOnly
// Toggles loop
// You must have permission over the current stream to do so.
func loop(ctx *commands.Context) error {
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	elem := ls.Head()
	if elem == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	strm := elem.Value
	if strm.Flags&(strflag_special|strflag_noskip) != 0 {
		return ctx.RespondPrivate("This stream cannot be modified.")
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.RespondPrivate("You do not have permission to modify the current stream.")
	}
	strm.Flags ^= strflag_loop
	if strm.Flags&strflag_loop != 0 {
		return ctx.Respond("Loop enabled.")
	}
	return ctx.Respond("Loop disabled.")
}

// ~!pause
// @Alias unpause
// @GuildOnly
// Toggles pause
// Yes, it's the same command for pausing and unpausing.
// You must have permission over the current stream to do so.
func pause(ctx *commands.Context) error {
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	elem := ls.Head()
	if elem == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	strm := elem.Value
	if strm.Flags&(strflag_special|strflag_noskip) != 0 {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.RespondPrivate("You do not have permission to modify the current stream.")
	}
	ctx.SetComponents(discordgo.Button{
		Emoji: discordgo.ComponentEmoji{Name: "\u23EF"},
	})
	strm.Flags ^= strflag_paused
	if strm.Flags&strflag_paused != 0 {
		strm.PauseTs = time.Since(strm.StartedAt)
		return ctx.Respond("Song paused.")
	}
	strm.StartedAt = time.Now().Add(-strm.PauseTs)
	return ctx.Respond("Song unpaused.")
}

// ~!skip
// @GuildOnly
// Skips the stream
// If you don't have permission over the stream, casts a skip vote.
// To skip a stream, at least half the non-deafened and non-muted users in the channel must vote to skip.
// Bots may still be counted in the channel count if they are not server deafened. For best results, server deafen bots.
func skip(ctx *commands.Context) error {
	if ctx.Type == discordgo.InteractionMessageComponent {
		ctx.FollowupPrepare()
	}
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	elem := ls.Head()
	if elem == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	obj := elem.Value
	if obj.Flags&(strflag_special|strflag_noskip) != 0 {
		return ctx.RespondPrivate("This stream cannot be skipped.")
	}
	vstate, err := ctx.State.VoiceState(ctx.GuildID, ctx.User.ID)
	if err != nil {
		return fmt.Errorf("failed to get voice state: %w", err)
	}
	mystate, err := ctx.State.VoiceState(ctx.GuildID, ctx.Me.ID)
	if err != nil {
		return fmt.Errorf("failed to get voice state: %w", err)
	}
	if vstate.ChannelID != mystate.ChannelID {
		return ctx.RespondPrivate("You have to be in the channel with me to cast a skip vote.")
	}
	log.Debug("skip: checked voice states")
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		log.Debug("skip: checked perms")
		if _, ok := obj.Skippers[ctx.User.ID]; !ok {
			obj.Skippers[ctx.User.ID] = struct{}{}
		} else {
			return ctx.RespondPrivate("You have already voted.")
		}
		log.Debug("skip: checked obj.skippers")
		guild, err := ctx.State.Guild(ctx.GuildID)
		if err != nil {
			return fmt.Errorf("failed to get guild: %w", err)
		}
		count := 0
		for _, v := range guild.VoiceStates {
			if v.ChannelID == mystate.ChannelID && !v.Mute && !v.Deaf && !v.SelfMute && !v.SelfDeaf && v.UserID != ctx.Me.ID {
				count++
			}
		}
		if len(obj.Skippers) < count/2 {
			ctx.FollowupDestroy()
			ctx.SetComponents(discordgo.Button{Emoji: discordgo.ComponentEmoji{Name: "\u23ED"}, Style: discordgo.SecondaryButton})
			return ctx.Respond(fmt.Sprintf("Still need %d more vote(s) to skip.", count/2-len(obj.Skippers)))
		}
	}
	log.Debug("skip: skipping")
	ls.Lock()
	obj.Stop <- struct{}{}
	obj.Flags &= ^strflag_paused
	// streams[ctx.GuildID].Remove(elem)
	ls.Unlock()
	ctx.FollowupDestroy()
	return ctx.Respond("Skipped.")
}

// ~!np
// @Alias playing
// @GuildOnly
// Info about what's playing
func np(ctx *commands.Context) error {
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.RespondPrivate("Not connected to voice.")
	}
	ls.RLock()
	if ls.Len() == 0 {
		since := int(lastPlayed[ctx.GuildID].Sub(time.Now().Add(dcTimeout)).Seconds())
		ls.RUnlock()
		if since > 59 {
			return ctx.Respond(fmt.Sprintf("Nothing is playing, will disconnect in %d minutes.", since/60))
		}
		return ctx.Respond(fmt.Sprintf("Nothing is playing, will disconnect in %d seconds.", since))
	}
	elem := ls.Head().Value
	ls.RUnlock()
	embed := new(discordgo.MessageEmbed)
	elapsed := elem.PauseTs
	paused := elem.Flags&strflag_paused != 0
	if !paused {
		elapsed = time.Since(elem.StartedAt)
	}
	elapsed = elapsed.Round(time.Second)
	var timeFld string
	if elem.Info != nil {
		embed.Title = elem.Info.Title
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: elem.Info.Thumbnail}
		embed.URL = elem.Info.Webpage
		timeFld = fmt.Sprintf("%01d:%02d/%01d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60, int(elem.Info.Duration)/60, int(elem.Info.Duration)%60)
	} else if elem.Flags&strflag_special != 0 {
		embed.Title = "???"
		if elem.Flags&strflag_dconend != 0 {
			embed.Title = "Outro"
		}
		timeFld = fmt.Sprintf("%01d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
	} else {
		embed.Title = "Uploaded File"
		embed.URL = elem.Source
		timeFld = fmt.Sprintf("%01d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
	}
	var footer string
	loop := elem.Flags&strflag_loop != 0
	if loop && paused {
		footer = "Currently looping, if it weren't paused."
	} else if paused {
		footer = "Currently paused."
	} else if loop {
		footer = "Currently looping."
	}
	if footer != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: footer}
	}
	author, err := ctx.State.Member(ctx.GuildID, elem.Author)
	if err != nil {
		return fmt.Errorf("failed to get member: %w", err)
	}
	footer = commands.DisplayName(author)
	embed.Author = &discordgo.MessageEmbedAuthor{Name: footer, IconURL: author.User.AvatarURL("")}
	embed.Fields = []*discordgo.MessageEmbedField{{Name: "Elapsed", Value: timeFld}}
	embed.Color = 0x992d22
	err = ctx.RespondEmbed(embed, true)
	return err
}

// ~!queue
// @GuildOnly
// Info about the queue
func queue(ctx *commands.Context) error {
	ls := streams[ctx.GuildID]
	if ls == nil || ls.Len() < 2 {
		return ctx.RespondPrivate("Nothing in the queue.")
	}
	ls.RLock()
	output := make([]string, ls.Len()-1)
	elem := ls.Head().Next()
	var i int
	for elem != nil {
		title := "Uploaded File"
		val := elem.Value
		if val.Info != nil {
			title = val.Info.Title
		}
		output[i] = fmt.Sprintf("%d. %s", i+1, title)
		elem = elem.Next()
		i++
	}
	ls.RUnlock()
	embed := new(discordgo.MessageEmbed)
	embed.Title = "Queue"
	embed.Description = strings.Join(output, "\n")
	embed.Color = 0x992d22
	return ctx.RespondEmbed(embed, true)
}
