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
// To clear the queue, do ~!remove all. You must have permission over all songs to do so.
func remove(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!remove <index>\nGet indices with ~!queue")
	}
	ls := streams[ctx.GuildID]
	if ls == nil || ls.Len() < 2 {
		return ctx.Send("Nothing in the queue.")
	}
	if args[0] == "all" || args[0] == "all-q" {
		perms, _ := ctx.State.UserChannelPermissions(ctx.Author.ID, ctx.ChanID)
		permitted := perms&discordgo.PermissionManageServer != 0
		if !permitted {
			djLock.RLock()
			DJ := djRoles[ctx.GuildID]
			djLock.RUnlock()
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
				if ctx.Author.ID != current.Author {
					ls.RUnlock()
					return ctx.Send("You do not have permission to clear the queue.")
				}
			}
			ls.RUnlock()
		}
		ls.Lock()
		for ls.Len() > 1 {
			ls.Remove(ls.Tail())
		}
		ls.Unlock()
		if args[0] == "all" {
			return ctx.Send("Queue cleared.")
		}
		return nil
	}
	ind, err := strconv.Atoi(args[0])
	if err != nil {
		return ctx.Send(args[0] + " is not a number.")
	}
	if ind < 1 || ind >= ls.Len() {
		return ctx.Send("Index out of bounds, expected 1-" + strconv.Itoa(ls.Len()-1))
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, ind) {
		return ctx.Send("You do not have permission to modify that stream.")
	}
	ls.Lock()
	elem := ls.Head()
	for i := 0; i < ind; i++ {
		elem = elem.Next()
	}
	ls.Remove(elem)
	ls.Unlock()
	obj := elem.Value
	msg, err := ctx.Bot.ChannelMessageSend(ctx.ChanID, "Removed "+obj.Info.Title+".")
	if err != nil {
		return fmt.Errorf("Could not send message: %w", err)
	}
	time.AfterFunc(2*time.Second, func() { ctx.Bot.ChannelMessageDelete(ctx.ChanID, msg.ID) })
	return nil
}

// ~!loop
// @GuildOnly
// Toggles loop
// You must have permission over the current stream to do so.
func loop(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.Send("Nothing is playing.")
	}
	elem := ls.Head()
	if elem == nil {
		return ctx.Send("Nothing is playing.")
	}
	strm := elem.Value
	if strm.Special {
		return ctx.Send("Nothing is playing.")
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.Send("You do not have permission to modify the current song.")
	}
	strm.Loop = !strm.Loop
	if strm.Loop {
		return ctx.Send("Loop enabled.")
	}
	return ctx.Send("Loop disabled.")
}

// ~!pause
// @Alias unpause
// @GuildOnly
// Toggles pause
// Yes, it's the same command for pausing and unpausing.
// You must have permission over the current stream to do so.
func pause(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.Send("Nothing is playing.")
	}
	elem := ls.Head()
	if elem == nil {
		return ctx.Send("Nothing is playing.")
	}
	strm := elem.Value
	if strm.Special {
		return ctx.Send("Nothing is playing.")
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.Send("You do not have permission to modify the current song.")
	}
	strm.Paused = !strm.Paused
	if strm.Paused {
		return ctx.Send("Song paused.")
	}
	return ctx.Send("Song unpaused.")
}

// ~!skip
// @GuildOnly
// Skips the stream
// If you don't have permission over the stream, casts a skip vote.
// To skip a song, at least half the non-deafened and non-muted users in the channel must vote to skip.
// Bots may still be counted in the channel count if they are not server deafened. For best results, server deafen bots.
func skip(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.Send("Nothing is playing.")
	}
	elem := ls.Head()
	if elem == nil {
		return ctx.Send("Nothing is playing.")
	}
	obj := elem.Value
	if obj.Special {
		return ctx.Send("Nothing is playing.")
	}
	vstate, err := ctx.State.VoiceState(ctx.GuildID, ctx.Author.ID)
	if err != nil {
		return fmt.Errorf("Failed to get voice state: %w", err)
	}
	mystate, err := ctx.State.VoiceState(ctx.GuildID, ctx.Me.ID)
	if err != nil {
		return fmt.Errorf("Failed to get voice state: %w", err)
	}
	if vstate.ChannelID != mystate.ChannelID {
		return ctx.Send("You have to be in the channel with me to cast a skip vote.")
	}
	log.Debug("skip: checked voice states")
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		log.Debug("skip: checked perms")
		if !obj.Skippers[ctx.Author.ID] {
			obj.Skippers[ctx.Author.ID] = true
			err = ctx.Send("Skip vote cast.")
			if err != nil {
				return err
			}
		}
		log.Debug("skip: checked obj.skippers")
		guild, err := ctx.State.Guild(ctx.GuildID)
		if err != nil {
			return fmt.Errorf("Failed to get guild: %w", err)
		}
		count := 0
		for _, v := range guild.VoiceStates {
			if v.ChannelID == mystate.ChannelID && !v.Mute && !v.Deaf && !v.SelfMute && !v.SelfDeaf && v.UserID != ctx.Me.ID {
				count++
			}
		}
		if len(obj.Skippers) < count/2 {
			return ctx.Send(fmt.Sprintf("Still need %d more votes to skip.", count/2-len(obj.Skippers)))
		}
	}
	log.Debug("skip: skipping")
	ls.Lock()
	obj.Stop <- true
	obj.Paused = false
	// streams[ctx.GuildID].Remove(elem)
	ls.Unlock()
	return ctx.Send("Skipped.")
}

// ~!np
// @Alias playing
// @GuildOnly
// Info about what's playing
func np(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.Send("Not connected to voice.")
	}
	ls.RLock()
	if ls.Len() == 0 {
		since := int(lastPlayed[ctx.GuildID].Sub(time.Now().Add(dcTimeout)).Seconds())
		ls.RUnlock()
		if since > 59 {
			return ctx.Send(fmt.Sprintf("Nothing is playing, will disconnect in %d minutes.", since/60))
		}
		return ctx.Send(fmt.Sprintf("Nothing is playing, will disconnect in %d seconds.", since))
	}
	elem := ls.Head().Value
	ls.RUnlock()
	embed := new(discordgo.MessageEmbed)
	elapsed := time.Since(elem.StartedAt).Round(time.Second)
	var timeFld string
	if elem.Info != nil {
		embed.Title = elem.Info.Title
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: elem.Info.Thumbnail}
		embed.URL = elem.Info.Webpage
		timeFld = fmt.Sprintf("%01d:%02d/%01d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60, int(elem.Info.Duration)/60, int(elem.Info.Duration)%60)
	} else if elem.Special {
		embed.Title = "???"
		timeFld = fmt.Sprintf("%01d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
	} else {
		embed.Title = "Uploaded File"
		embed.URL = elem.Source
		timeFld = fmt.Sprintf("%01d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
	}
	var footer string
	if elem.Loop && elem.Paused {
		footer = "Currently looping, if it weren't paused."
	} else if elem.Paused {
		footer = "Currently paused."
	} else if elem.Loop {
		footer = "Currently looping."
	}
	if footer != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: footer}
	}
	author, err := ctx.State.Member(ctx.GuildID, elem.Author)
	if err != nil {
		return fmt.Errorf("Failed to get member: %w", err)
	}
	footer = commands.DisplayName(author)
	embed.Author = &discordgo.MessageEmbedAuthor{Name: footer, IconURL: author.User.AvatarURL("")}
	embed.Fields = []*discordgo.MessageEmbedField{{Name: "Elapsed", Value: timeFld}}
	embed.Color = 0x992d22
	_, err = ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
	return err
}

// ~!queue
// @GuildOnly
// Info about the queue
func queue(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	ls := streams[ctx.GuildID]
	if ls == nil || ls.Len() < 2 {
		return ctx.Send("Nothing in the queue.")
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
	_, err := ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
	return err
}
