package clickart

import (
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/music"
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
			music.SetClickArt(event.BeforeUpdate.GuildID, false)
		}
	}
}

func clickItGood(self *discordgo.Session, gid string, click bool, affirmation string) {
	if click {
		music.PlaySpecialSound(self, gid, "modules/clickart/clicker.ogg")
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
		music.PlaySpecialSound(self, gid, loc)
	}
}
