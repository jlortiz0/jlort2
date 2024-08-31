package clickart

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/music"
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

var activities map[string]*activity = map[string]*activity{
	"test": {
		reminder:   "Quickly, send the command!",
		minBetween: time.Second * 20,
		maxBetween: time.Second * 40,
		expected:   time.Second * 10,
	},
}
var affirmations map[string]int = map[string]int{
	"unya": 3,
}

var activeUsers map[string]*activeUser = make(map[string]*activeUser)
var activeUsersLock sync.Mutex

func clickart(ctx *commands.Context) error {
	data := ctx.ApplicationCommandData()
	activity := activities[data.Options[0].StringValue()]
	if activity == nil {
		return ctx.RespondPrivate("Somehow, you sent an invalid activity called " + data.Options[1].StringValue())
	}
	var training bool
	var affirmation string
	if len(data.Options) > 1 {
		if data.Options[1].Type == discordgo.ApplicationCommandOptionBoolean {
			training = data.Options[1].BoolValue()
		} else {
			affirmation = data.Options[1].StringValue()
			_, ok := affirmations[affirmation]
			if !ok {
				return ctx.RespondPrivate("Somehow, you sent an invalid affirmation called " + affirmation)
			}
		}
	}
	if len(data.Options) > 2 {
		affirmation = data.Options[2].StringValue()
		_, ok := affirmations[affirmation]
		if !ok {
			return ctx.RespondPrivate("Somehow, you sent an invalid affirmation called " + affirmation)
		}
	}
	err := music.TryConnect(ctx)
	if err != nil {
		if _, ok := err.(*time.ParseError); !ok {
			return err
		}
		return nil
	}
	if !music.SetClickArt(ctx.GuildID, true) {
		return ctx.RespondPrivate("Failed to enable ClickArt session; is something playing right now?")
	}

	ch, err := ctx.Bot.UserChannelCreate(ctx.User.ID)
	if err != nil {
		return fmt.Errorf("failed to create user channel: %w", err)
	}
	chID := ch.ID

	until := activity.minBetween + time.Duration(rand.Int64N(int64(activity.maxBetween-activity.minBetween)))
	activeUsersLock.Lock()
	_, ok := activeUsers[ctx.User.ID]
	if !ok {
		activeUsers[ctx.User.ID] = &activeUser{
			training:    training,
			activity:    activity,
			affirmation: affirmation,
			timer:       time.AfterFunc(until, func() { doClick(ctx.Bot, ctx.User.ID) }),
			channelID:   chID,
			guildID:     ctx.GuildID,
		}
	}
	activeUsersLock.Unlock()
	if training {
		if affirmation != "" {
			return ctx.RespondPrivate("ClickArt session started. I will DM you reminders to do the activity; when you do it, use /praiseme to get a click and your chosen affirmation.")
		}
		return ctx.RespondPrivate("ClickArt session started. I will DM you reminders to do the activity; when you do it, use /praiseme to get a click.")
	}
	if affirmation != "" {
		ctx.RespondPrivate("ClickArt session started. When you hear the click, do the activity and use /praiseme to receive an affirmation.")
	}
	return ctx.RespondPrivate("ClickArt session started. When you hear the click, do the activity and use /praiseme or you won't get a star.")
}

func clickoff(ctx *commands.Context) error {
	activeUsersLock.Lock()
	act, ok := activeUsers[ctx.User.ID]
	if ok {
		delete(activeUsers, ctx.User.ID)
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
		music.SetClickArt(ctx.GuildID, false)
		percent := float32(score) / float32(total)
		var msg string
		if total < 3 {
			msg = "This session was too short for a star. Try again next time!"
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
		return ctx.RespondPrivate(fmt.Sprintf("ClickArt session finished.\nYour score was %d/%d, or %.1f%%.\n%s", score, total, percent, msg))
	}
	return ctx.RespondPrivate("You do not have a ClickArt session.")
}

func praiseme(ctx *commands.Context) error {
	activeUsersLock.Lock()
	act, ok := activeUsers[ctx.User.ID]
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
	activeUsersLock.Unlock()
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
	commands.PrepareCommand("clickart", "temporary description").Guild().Register(clickart, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("activity", "what the hey are you doing?").AsString().Choice(activityChoices).Required().Finalize(),
		commands.NewCommandOption("training", "if true, click as a reward. if false, click as a prompt.").AsBool().Finalize(),
		commands.NewCommandOption("affirmation", "audio affirmations to help you get acquianted").AsString().Choice(affirmationChoices).Finalize(),
	})
	commands.PrepareCommand("clickoff", "Stop your clickart session").Guild().Register(clickoff, nil)
	commands.PrepareCommand("praiseme", "Did you do your task? Get some praise, then!").Register(praiseme, nil)
}

func Cleanup(self *discordgo.Session) {
	activeUsersLock.Lock()
	for k, v := range activeUsers {
		delete(activeUsers, k)
		music.SetClickArt(v.guildID, false)
	}
	activeUsersLock.Unlock()
}
