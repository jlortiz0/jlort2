package clickart

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
)

type activity struct {
	reminder   string
	minBetween time.Duration
	maxBetween time.Duration
	expected   time.Duration
}

type activeUser struct {
	praise chan struct{}
	*activity
	timer       *time.Timer
	affirmation string
	channelID   string
	guildID     string
	sync.Mutex
	score    int
	total    int
	training bool
}

type affirmationEntry struct {
	common, rare int
}

var activities map[string]*activity = map[string]*activity{
	"saving": {
		reminder:   "Save!",
		minBetween: time.Minute * 4,
		maxBetween: time.Minute * 20,
		expected:   time.Minute / 2,
	},
	"barking": {
		reminder:   "Bark!",
		minBetween: time.Minute * 2,
		maxBetween: time.Minute * 10,
		expected:   time.Minute / 4,
	},
	"meowing": {
		reminder:   "Meow!",
		minBetween: time.Minute * 2,
		maxBetween: time.Minute * 10,
		expected:   time.Minute / 4,
	},
	"hydrating": {
		reminder:   "Hydrate!",
		minBetween: time.Minute * 8,
		maxBetween: time.Minute * 30,
		expected:   time.Minute / 2,
	},
	// "testing": {
	// 	reminder:   "Command!",
	// 	minBetween: time.Second * 20,
	// 	maxBetween: time.Second * 40,
	// 	expected:   time.Minute / 4,
	// },
}
var affirmations map[string]affirmationEntry = map[string]affirmationEntry{
	"cynthia_boy":      {11, 2},
	"cynthia_girl":     {7, 0},
	"cynthia_puppy":    {7, 5},
	"cynthia_kitty":    {11, 2},
	"cynthia_pogchamp": {4, 3},
}

var activeUsers map[string]*activeUser = make(map[string]*activeUser)
var guildUsersMap map[string]string = make(map[string]string)
var activeUsersLock sync.RWMutex

func clickart(ctx *commands.Context) error {
	data := ctx.ApplicationCommandData()
	activity := activities[data.Options[0].StringValue()]
	if activity == nil {
		return ctx.RespondPrivate("Somehow, you sent an invalid activity called " + data.Options[1].StringValue())
	}
	var training bool
	var affirmation string
	for _, opt := range data.Options[1:] {
		if opt.Name == "training" {
			training = opt.BoolValue()
		} else {
			affirmation = opt.StringValue()
			_, ok := affirmations[affirmation]
			if !ok {
				return ctx.RespondPrivate("Somehow, you sent an invalid affirmation called " + affirmation)
			}
		}
	}

	authorVoice, err := ctx.State.VoiceState(ctx.GuildID, ctx.User.ID)
	if err != nil || authorVoice.ChannelID == "" {
		return ctx.RespondPrivate("You must be in a voice channel to use this command.")
	}
	ctx.Bot.RLock()
	_, ok := ctx.Bot.VoiceConnections[ctx.GuildID]
	ctx.Bot.RUnlock()
	if ok {
		return ctx.RespondPrivate("There is already a Clickart session active in this server.")
	}
	ch, err := ctx.Bot.UserChannelCreate(ctx.User.ID)
	if err != nil {
		return fmt.Errorf("failed to create user channel: %w", err)
	}
	_, err = ctx.Bot.ChannelVoiceJoin(ctx.GuildID, authorVoice.ChannelID, false, true)
	if err != nil {
		return fmt.Errorf("failed to connect to voice: %w", err)
	}

	until := activity.minBetween + time.Duration(rand.Int64N(int64(activity.maxBetween-activity.minBetween)))
	activeUsersLock.Lock()
	old, ok := activeUsers[ctx.User.ID]
	if ok && old.timer != nil {
		old.timer.Stop()
	}
	activeUsers[ctx.User.ID] = &activeUser{
		training:    training,
		activity:    activity,
		affirmation: affirmation,
		timer:       time.AfterFunc(until, func() { doClick(ctx.Bot, ctx.User.ID) }),
		channelID:   ch.ID,
		guildID:     ctx.GuildID,
	}
	guildUsersMap[ctx.GuildID] = ctx.User.ID
	activeUsersLock.Unlock()
	if training {
		if affirmation != "" {
			return ctx.RespondPrivate("ClickArt session started. I will DM you reminders to do the activity; when you do it, use /praiseme to get a click and your chosen affirmation.")
		}
		return ctx.RespondPrivate("ClickArt session started. I will DM you reminders to do the activity; when you do it, use /praiseme to get a click.")
	}
	if affirmation != "" {
		return ctx.RespondPrivate("ClickArt session started. When you hear the click, do the activity and use /praiseme to receive an affirmation.")
	}
	return ctx.RespondPrivate("ClickArt session started. When you hear the click, do the activity and use /praiseme or you won't get a star.")
}

func clickoff(ctx *commands.Context) error {
	activeUsersLock.Lock()
	act, ok := activeUsers[ctx.User.ID]
	if ok {
		delete(activeUsers, ctx.User.ID)
		delete(guildUsersMap, act.guildID)
	}
	activeUsersLock.Unlock()
	if ok {
		act.Lock()
		act.timer.Stop()
		if act.praise != nil {
			close(act.praise)
		}
		total := act.total
		score := act.score
		act.Unlock()
		ctx.Bot.RLock()
		vc := ctx.Bot.VoiceConnections[ctx.GuildID]
		ctx.Bot.RUnlock()
		if vc != nil {
			vc.Disconnect()
		}
		percent := float32(score) / float32(total)
		var msg string
		if total < 3 {
			return ctx.RespondPrivate("ClickArt session finished.\nThis session was too short for a star. Try again next time!")
		} else if percent == 1 {
			msg = "You got a foil star! Incredible! "
			if !act.training {
				msg = " How about switching off training mode next time?"
			}
		} else if percent > 0.95 {
			msg = "You got a gold star. Excellent work!"
			if act.training {
				msg += " How about switching off training mode next time?"
			}
		} else if percent > 0.9 {
			msg = "You got a silver star. Good job."
		} else if percent > 0.8 {
			msg = "You got a bronze star."
		} else if percent > 0.2 {
			msg = "You did not get a star."
		} else {
			msg = "You tried... I hope."
		}
		return ctx.RespondPrivate(fmt.Sprintf("ClickArt session finished.\nYour score was %d/%d, or %.0f%%.\n%s", score, total, percent*100, msg))
	}
	return ctx.RespondPrivate("You do not have a ClickArt session.")
}

func praiseme(ctx *commands.Context) error {
	activeUsersLock.RLock()
	act, ok := activeUsers[ctx.User.ID]
	activeUsersLock.RUnlock()
	var ok2, training bool
	if ok {
		act.Lock()
		ok2 = act.praise != nil
		training = act.training
		if ok2 {
			close(act.praise)
			act.praise = nil
		}
		act.Unlock()
	}
	if ok && ok2 {
		if act.training || act.affirmation != "" {
			go clickItGood(ctx.Bot, ctx.GuildID, act.training, act.affirmation)
			return ctx.RespondEmpty()
		}
		return ctx.RespondPrivate("Good work.")
	} else if !ok {
		return ctx.RespondPrivate("You do not have a ClickArt session.")
	} else if !training {
		return ctx.RespondPrivate("You have to wait for the clicker, you know.")
	}
	return ctx.RespondPrivate("You have to wait for me to tell you to do something, you know.")
}

func Init(self *discordgo.Session) {
	activityChoices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(activities))
	for k := range activities {
		activityChoices = append(activityChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  k,
			Value: k,
		})
	}
	affirmationChoices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(affirmations))
	for k := range affirmations {
		affirmationChoices = append(affirmationChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  k,
			Value: k,
		})
	}

	self.AddHandler(cancelOnDc)
	commands.PrepareCommand("clickart", "Get rewarded as a netizen deserves").Guild().Register(clickart, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("activity", "what the hey are you doing?").AsString().Choice(activityChoices).Required().Finalize(),
		commands.NewCommandOption("training", "if true, click as a reward. if false, click as a prompt.").AsBool().Finalize(),
		commands.NewCommandOption("affirmation", "audio affirmations to help you get acquianted").AsString().Choice(affirmationChoices).Finalize(),
	})
	commands.PrepareCommand("clickoff", "Stop your clickart session").Guild().Register(clickoff, nil)
	commands.PrepareCommand("praiseme", "Did you do your task? Get some praise, then!").Register(praiseme, nil)
}

func Cleanup(self *discordgo.Session) {
	activeUsersLock.Lock()
	for k, v := range guildUsersMap {
		delete(activeUsers, v)
		delete(guildUsersMap, k)
		vc := self.VoiceConnections[k]
		if vc != nil {
			vc.Disconnect()
		}
	}
	activeUsersLock.Unlock()
}
