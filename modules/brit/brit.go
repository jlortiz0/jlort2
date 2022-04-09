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

package brit

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

type duelObj struct {
	P1     *discordgo.User
	P2     *discordgo.User
	P1Said bool
	P2Said bool
}

func (duel *duelObj) isParticipant(mem string) bool {
	return duel != nil && (mem == duel.P1.ID || mem == duel.P2.ID)
}

func (duel *duelObj) said(mem string) *discordgo.User {
	if mem == duel.P1.ID {
		duel.P1Said = true
		return duel.P2
	} else if mem == duel.P2.ID {
		duel.P2Said = true
		return duel.P1
	}
	return nil
}

func (duel *duelObj) didSay(mem string) bool {
	if mem == duel.P1.ID {
		return duel.P1Said
	} else if mem == duel.P2.ID {
		return duel.P2Said
	}
	return false
}

func (duel *duelObj) other(mem string) *discordgo.User {
	if mem == duel.P1.ID {
		return duel.P2
	} else if mem == duel.P2.ID {
		return duel.P1
	}
	return nil
}

var britdata map[string]int
var cooldown map[string]time.Time
var duels map[string]*duelObj
var dirty bool
var britLock *sync.Mutex = new(sync.Mutex)
var duelLock *sync.RWMutex = new(sync.RWMutex)

// ~!flip [times]
// Flips a coin
// If times is provided, flips multiple coins.
func flip(ctx commands.Context) error {
	count := 1
	args := ctx.ApplicationCommandData().Options
	if len(args) > 0 {
		count = int(args[0].IntValue())
	}
	if count == 1 {
		if rand.Int()&1 == 0 {
			return ctx.Respond("Heads")
		}
		return ctx.Respond("Tails")
	}
	heads := 0
	for i := 0; i < count; i++ {
		if rand.Int()&1 == 0 {
			heads++
		}
	}
	return ctx.Respond(strconv.Itoa(heads) + " heads")
}

// ~!roll [count]
// Rolls a six-sided die
// If count is provided, rolls multiple.
func roll(ctx commands.Context) error {
	count := 1
	args := ctx.ApplicationCommandData().Options
	if len(args) > 0 {
		count = int(args[0].IntValue())
	}
	total := count
	for i := 0; i < count; i++ {
		total += rand.Intn(6)
	}
	return ctx.Respond("Rolled " + strconv.Itoa(total))
}

var ballresp = [...]string{"Yes", "No", "Maybe so", "Hell yes", "Hell no", "Get back to me when jlort jlort 3 comes out", "Not until you get negative kek", "Of course", "Go to jail, go directly... oh, wrong game.", "When I learn to talk, I'll tell you", "Turn around.", "You? HAHAHAHAHA no", "aaa eee ooo", "500 Internal Server Error", "404 Possibility Not Found", "302 Possibility Found"}

// ~!8ball <thing>
// Ask the magic 8 ball a serious question, and get a stupid answer.
// No, really. This one hates your guts. And my guts.
func eightball(ctx commands.Context) error {
	return ctx.Respond(ballresp[rand.Intn(len(ballresp))])
}

// ~!howbrit [user]
// @Hidden
// Checks someone's Britishness
// If none is specified, gives the Britishness of the command runner.
func howbrit(ctx commands.Context) error {
	target := ctx.User
	args := ctx.ApplicationCommandData().Options
	if len(args) != 0 && ctx.GuildID != "" {
		target = args[0].UserValue(ctx.Bot)
	}
	// britLock.RLock()
	amnt, ok := britdata[target.ID]
	// britLock.RUnlock()
	if !ok && !target.Bot {
		amnt = 50
	}
	return ctx.Respond(fmt.Sprintf("%s is %d%% British", target.Username, amnt))
}

// ~!brit <user>
// @Hidden
// @GuildOnly
// Calls someone British
// This increases thier British score by 2.
// If in a duel and no user is specified, your opponent will be called British.
// In that case, your opponent's British score will not increase until after the duel.
func brit(ctx commands.Context) error {
	if ctx.GuildID == "" {
		return ctx.RespondPrivate("This command can only be used in servers.")
	}
	duelLock.RLock()
	curDuel := duels[ctx.User.ID]
	duelLock.RUnlock()
	target := ctx.ApplicationCommandData().Options[0].UserValue(ctx.Bot)
	if target.ID == ctx.User.ID {
		return ctx.RespondPrivate("Self-deprecation isn't allowed here.")
	}
	if curDuel.isParticipant(target.ID) {
		return ctx.Respond(fmt.Sprintf("%s taunts %s before the match!", commands.DisplayName(ctx.Member), curDuel.said(ctx.User.ID).Username))
	}
	duelLock.RLock()
	until := time.Until(cooldown[ctx.User.ID])
	duelLock.RUnlock()
	if until > 0 {
		return ctx.RespondPrivate(fmt.Sprintf("Wait %.0fs for cooldown.", until.Seconds()))
	}
	duelLock.Lock()
	cooldown[ctx.User.ID] = time.Now().Add(3 * time.Minute)
	duelLock.Unlock()
	myname := commands.DisplayName(ctx.Member)
	britLock.Lock()
	amnt, ok := britdata[target.ID]
	if !ok && !target.Bot {
		amnt = 50
	}
	amnt += 2
	if amnt > 100 {
		amnt = 100
	}
	britdata[target.ID] = amnt
	dirty = true
	britLock.Unlock()
	return ctx.Respond(fmt.Sprintf("%s calls %s British!", myname, target.Username))
}

// ~!duel <user>
// @Hidden
// @GuildOnly
// Challenges someone to an ungentlemanly duel
// The winner will be decided after 30s.
// If you're certain of victory, you can call the other person British before the duel is decided. To do so, run ~!brit with no arguments.
// Once the winner is decided, the Brit scores will be adjusted based on whether or not each called the other a Brit.
func duel(ctx commands.Context) error {
	if ctx.GuildID == "" {
		return ctx.RespondPrivate("This command can only be used in servers.")
	}
	target := ctx.ApplicationCommandData().Options[0].UserValue(ctx.Bot)
	if target.ID == ctx.User.ID {
		return ctx.RespondPrivate("Cannot duel yourself.")
	}
	duelLock.RLock()
	if duels[ctx.User.ID] != nil {
		duelLock.RUnlock()
		return ctx.RespondPrivate("Already in a duel!")
	}
	if duels[target.ID] != nil {
		duelLock.RUnlock()
		return ctx.RespondPrivate(target.Username + " is already in a duel!")
	}
	until := time.Until(cooldown[ctx.User.ID])
	if until > 0 {
		duelLock.RUnlock()
		return ctx.RespondPrivate(fmt.Sprintf("Wait %.0fs for cooldown.", until.Seconds()))
	}
	duelLock.RUnlock()
	duelLock.Lock()
	cooldown[ctx.User.ID] = time.Now().Add(5 * time.Minute)
	curDuel := &duelObj{target, ctx.User, false, false}
	duels[ctx.User.ID] = curDuel
	duels[target.ID] = curDuel
	duelLock.Unlock()
	embed := new(discordgo.MessageEmbed)
	embed.Title = "**Duel!**"
	embed.Footer = &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("%s vs %s", commands.DisplayName(ctx.Member), target.Username)}
	embed.Color = 0x992d22
	embed.Description = "Duel will occur in 30s. If you're sure of victory, take this time to call the other person British!"
	err := ctx.RespondEmbed(embed)
	if err == nil {
		time.AfterFunc(30*time.Second, func() { duelCleanup(curDuel, embed, ctx) })
	}
	return err
}

func duelCleanup(curDuel *duelObj, embed *discordgo.MessageEmbed, ctx commands.Context) {
	winRng := rand.Intn(8)
	var win bool
	if curDuel.P2.Bot {
		win = (winRng == 0)
	} else {
		win = (winRng&4 == 0)
	}
	var winner *discordgo.User
	if win {
		winner = curDuel.P1
	} else {
		winner = curDuel.P2
	}
	loser := curDuel.other(winner.ID)
	duelLock.Lock()
	delete(duels, winner.ID)
	delete(duels, loser.ID)
	duelLock.Unlock()
	embed.Description = winner.Username + " won the duel!"
	britLock.Lock()
	winBrit, ok := britdata[winner.ID]
	if !ok && !winner.Bot {
		winBrit = 50
	}
	losBrit, ok := britdata[loser.ID]
	if !ok && !loser.Bot {
		losBrit = 50
	}
	winBrit -= 4
	losBrit += 8
	if curDuel.didSay(winner.ID) {
		winBrit -= 4
		losBrit += 4
	}
	if curDuel.didSay(loser.ID) {
		losBrit += 8
	}
	if losBrit > 100 {
		losBrit = 100
	}
	if winBrit < 0 {
		winBrit = 0
	}
	britdata[winner.ID] = winBrit
	britdata[loser.ID] = losBrit
	dirty = true
	britLock.Unlock()
	ctx.RespondEditEmbed(embed)
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also initializes the cooldown and duel maps and loads the scores from disk.
func Init(_ *discordgo.Session) {
	err := commands.LoadPersistent("brit", &britdata)
	if err != nil {
		log.Error(err)
		return
	}
	cooldown = make(map[string]time.Time)
	duels = make(map[string]*duelObj)
	optionCoins := new(discordgo.ApplicationCommandOption)
	optionCoins.Type = discordgo.ApplicationCommandOptionInteger
	optionCoins.Name = "coins"
	optionCoins.Description = "How many coins to flip"
	commands.RegisterCommand(flip, "flip", "Flip a coin", []*discordgo.ApplicationCommandOption{optionCoins})
	optionDice := new(discordgo.ApplicationCommandOption)
	optionDice.Type = discordgo.ApplicationCommandOptionInteger
	optionDice.Name = "dice"
	optionDice.Description = "How many dice to roll"
	commands.RegisterCommand(roll, "roll", "Roll a die", []*discordgo.ApplicationCommandOption{optionDice})
	optionUser1 := new(discordgo.ApplicationCommandOption)
	optionUser1.Type = discordgo.ApplicationCommandOptionUser
	optionUser1.Name = "user"
	optionUser1.Description = "Person to test the Britishness of, default you"
	commands.RegisterCommand(howbrit, "howbrit", "^", []*discordgo.ApplicationCommandOption{optionUser1})
	optionUser2 := new(discordgo.ApplicationCommandOption)
	optionUser2.Type = discordgo.ApplicationCommandOptionUser
	optionUser2.Name = "user"
	optionUser2.Description = "Person to call British"
	optionUser2.Required = true
	commands.RegisterCommand(brit, "brit", "Call someone out", []*discordgo.ApplicationCommandOption{optionUser2})
	optionUser3 := new(discordgo.ApplicationCommandOption)
	optionUser3.Type = discordgo.ApplicationCommandOptionUser
	optionUser3.Name = "user"
	optionUser3.Description = "Person to duel"
	optionUser3.Required = true
	commands.RegisterCommand(duel, "duel", "Your nationality is on the line", []*discordgo.ApplicationCommandOption{optionUser3})
    optionSomething := new(discordgo.ApplicationCommandOption)
    optionSomething.Type = discordgo.ApplicationCommandOptionString
    optionSomething.Name = "question"
    optionSomething.Required = true
    commands.RegisterCommand(eightball, "8ball", "Get a stupid answer", []*discordgo.ApplicationCommandOption{optionSomething})
	commands.RegisterSaver(saveBrit)
}

func saveBrit() error {
	if !dirty {
		return nil
	}
	britLock.Lock()
	err := commands.SavePersistent("brit", &britdata)
	if err == nil {
		dirty = false
	}
	britLock.Unlock()
	return err
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the scores to disk.
func Cleanup(_ *discordgo.Session) {
	err := saveBrit()
	if err != nil {
		log.Error(err)
	}
}
