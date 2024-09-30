package clickart

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/log"
)

func doClick(self *discordgo.Session, uid string) {
	activeUsersLock.RLock()
	usr := activeUsers[uid]
	activeUsersLock.RUnlock()

	// usr.training and usr.activity aren't modified after intialization, so they are safe to read without a lock
	if usr.training {
		self.ChannelMessageSend(usr.channelID, usr.reminder)
	} else {
		clickItGood(self, usr.guildID, true, "")
	}

	usr.Lock()
	praise := make(chan struct{})
	usr.praise = praise
	usr.Unlock()
	t := time.NewTimer(usr.activity.expected)
	defer t.Stop()

	var elapsed bool
	select {
	case <-t.C:
		elapsed = true
	case <-praise:
	}
	// If we got deleted from the map while waiting, we shouldn't queue ourselves again
	activeUsersLock.RLock()
	_, ok := activeUsers[uid]
	activeUsersLock.RUnlock()
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
	if event.BeforeUpdate == nil || event.ChannelID == event.BeforeUpdate.ChannelID {
		return
	}
	self.RLock()
	vc := self.VoiceConnections[event.BeforeUpdate.GuildID]
	self.RUnlock()
	uid := event.UserID
	activeUsersLock.Lock()
	if uid == self.State.User.ID {
		uid = guildUsersMap[event.BeforeUpdate.GuildID]
	}
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
		if vc != nil {
			vc.Disconnect()
		}
	}
}

func clickItGood(self *discordgo.Session, gid string, click bool, affirmation string) {
	vc := self.VoiceConnections[gid]
	if vc == nil {
		return
	}
	if click {
		musicStreamer(vc, "modules/clickart/clicker.ogg")
	}
	if affirmation != "" {
		possible := affirmations[affirmation]
		var loc string
		if possible.rare != 0 && rand.N(64) == 0 {
			loc = fmt.Sprintf("modules/clickart/affirmations/%s_rare_%d.ogg", affirmation, rand.N(possible.rare)+1)
		} else {
			loc = fmt.Sprintf("modules/clickart/affirmations/%s_%d.ogg", affirmation, rand.N(possible.common)+1)
		}
		musicStreamer(vc, loc)
	}
}

func musicStreamer(vc *discordgo.VoiceConnection, source string) {
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
}
