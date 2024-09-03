package clickart

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

func doClick(self *discordgo.Session, uid string) {
	activeUsersLock.Lock()
	usr := activeUsers[uid]
	activeUsersLock.Unlock()

	// usr.training and usr.activity aren't modified after intialization, so they are safe to read without a lock
	if usr.training {
		self.ChannelMessageSend(usr.channelID, usr.reminder)
	} else {
		clickItGood(self, usr.guildID, true, "")
	}

	usr.Lock()
	usr.praise = make(chan struct{})
	usr.Unlock()
	t := time.NewTimer(usr.activity.expected)
	defer t.Stop()

	var elapsed bool
	select {
	case <-t.C:
		elapsed = true
	case <-usr.praise:
	}
	// If we got deleted from the map while waiting, we shouldn't queue ourselves again
	activeUsersLock.Lock()
	_, ok := activeUsers[uid]
	activeUsersLock.Unlock()
	if !ok {
		return
	}
	until := usr.activity.minBetween + time.Duration(rand.Int64N(int64(usr.activity.maxBetween-usr.activity.minBetween)))
	if elapsed {
		// Account for delay in message send.
		// We want to set praise to nil before we send, so don't actually send it here.
		until += time.Second * 2
	}

	usr.Lock()
	if !elapsed {
		usr.score += 1
	}
	usr.praise = nil
	usr.total += 1
	usr.timer.Reset(until)
	usr.Unlock()
	if elapsed {
		self.ChannelMessageSend(usr.channelID, "Sorry, you're out of time. No click for you.")
	}
}

func cancelOnDc(self *discordgo.Session, event *discordgo.VoiceStateUpdate) {
	if event.UserID == self.State.User.ID && event.ChannelID == "" {
		uid := event.BeforeUpdate.UserID
		activeUsersLock.Lock()
		act, ok := activeUsers[uid]
		if ok {
			delete(activeUsers, uid)
		}
		activeUsersLock.Unlock()
		if ok {
			act.Lock()
			act.timer.Stop()
			if act.praise != nil {
				close(act.praise)
			}
			act.Unlock()
		}
		return
	}
	vc := self.VoiceConnections[event.GuildID]
	if vc == nil {
		return
	}
	vguild, err := self.State.Guild(event.GuildID)
	if err != nil {
		return
	}
	count := 0
	for _, v := range vguild.VoiceStates {
		if v.ChannelID == vc.ChannelID && !v.Deaf && !v.SelfDeaf && v.UserID != self.State.User.ID {
			count++
		}
	}
	if count == 0 {
		vc.Disconnect()
	}
}

func clickItGood(self *discordgo.Session, gid string, click bool, affirmation string) {
	vc := self.VoiceConnections[gid]
	if vc == nil {
		return
	}
	if click {
		musicStreamer(vc, "modules/clickart/clicker.ogg", false)
	}
	if affirmation != "" {
		possible := affirmations[affirmation]
		var loc string
		if possible == 1 {
			loc = "modules/clickart/affirmations/" + affirmation + ".ogg"
		} else {
			sel := rand.N(possible) + 1
			loc = "modules/clickart/affirmations/" + affirmation + strconv.Itoa(sel) + ".ogg"
		}
		musicStreamer(vc, loc, false)
	}
}

func musicStreamer(vc *discordgo.VoiceConnection, source string, dconend bool) {
	f, err := os.Open(source)
	if err != nil {
		log.Error(err)
		return
	}
	defer f.Close()
	rd := bufio.NewReaderSize(f, 4096)
	header := make([]byte, 4)
	var count byte
Streamer:
	for {
		_, err = io.ReadFull(rd, header)
		if err != nil || header[0] != 'O' || header[1] != 'g' || header[1] != header[2] || header[3] != 'S' {
			break
		}
		_, err := io.CopyN(io.Discard, rd, 22)
		if err != nil {
			log.Error(err)
			break
		}
		count, err = rd.ReadByte()
		if err != nil {
			break
		}
		segtable := make([]byte, count)
		_, err = io.ReadFull(rd, segtable)
		if err != nil {
			break
		}
		size := 0
		for _, v := range segtable {
			size += int(v)
			if v != 255 {
				b := make([]byte, size)
				_, err = io.ReadFull(rd, b)
				if err != nil {
					break Streamer
				}
				vc.OpusSend <- b
				size = 0
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	if dconend {
		vc.Disconnect()
	}
}

type sendPrivateError string

func (s sendPrivateError) Error() string { return string(s) }

func connect(ctx *commands.Context) error {
	authorVoice, err := ctx.State.VoiceState(ctx.GuildID, ctx.User.ID)
	if err != nil || authorVoice.ChannelID == "" {
		return sendPrivateError("You must be in a voice channel to use this command.")
	}
	vc, ok := ctx.Bot.VoiceConnections[ctx.GuildID]
	if ok {
		if vc.ChannelID != authorVoice.ChannelID {
			channel, err := ctx.State.Channel(vc.ChannelID)
			if err != nil {
				return fmt.Errorf("failed to get channel info: %w", err)
			}
			return sendPrivateError("Please move to voice channel " + channel.Name)
		}
	} else {
		_, err = ctx.Bot.ChannelVoiceJoin(ctx.GuildID, authorVoice.ChannelID, false, true)
		if err != nil {
			perms, err2 := ctx.State.UserChannelPermissions(ctx.Me.ID, authorVoice.ChannelID)
			if err2 == nil && perms&discordgo.PermissionVoiceConnect == 0 {
				return sendPrivateError("I need the Connect permission to use this command.")
			}
			return fmt.Errorf("failed to connect to voice: %w", err)
		}
	}
	return nil
}

func outro(ctx *commands.Context) error {
	activeUsersLock.Lock()
	_, ok := activeUsers[ctx.GuildID]
	activeUsersLock.Unlock()
	vc := ctx.Bot.VoiceConnections[ctx.GuildID]
	if vc == nil {
		return ctx.RespondPrivate("Not connected to voice.")
	}
	if ok {
		return ctx.RespondPrivate("Can't play an outro during a ClickArt session.")
	}
	name := ctx.ApplicationCommandData().Options[0].StringValue()
	pth := "outro" + string(os.PathSeparator) + name + ".ogg"
	_, err := os.Stat(pth)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ctx.RespondPrivate("That outro does not exist.")
		}
		return err
	}
	go musicStreamer(vc, pth, true)
	return ctx.RespondEmpty()
}
