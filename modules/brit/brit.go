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
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

type duelObj struct {
	P1     *discordgo.Member
	P2     *discordgo.Member
	P1Said bool
	P2Said bool
}

func (duel *duelObj) isParticipant(mem string) bool {
	return mem == duel.P1.User.ID || mem == duel.P2.User.ID
}

func (duel *duelObj) said(mem string) *discordgo.Member {
	if mem == duel.P1.User.ID {
		duel.P1Said = true
		return duel.P1
	} else if mem == duel.P2.User.ID {
		duel.P2Said = true
		return duel.P2
	}
	return nil
}

func (duel *duelObj) didSay(mem *discordgo.Member) bool {
	if mem == duel.P1 {
		return duel.P1Said
	} else if mem == duel.P2 {
		return duel.P2Said
	}
	return false
}

func (duel *duelObj) other(mem string) *discordgo.Member {
	if mem == duel.P1.User.ID {
		return duel.P2
	} else if mem == duel.P2.User.ID {
		return duel.P1
	}
	return nil
}

var cooldown map[string]time.Time
var duels map[string]*duelObj
var duelLock *sync.RWMutex = new(sync.RWMutex)
var queryBrit *sql.Stmt
var incrBrit *sql.Stmt

// ~!flip [times]
// Flips a coin
// If times is provided, flips multiple coins.
func flip(ctx commands.Context, args []string) error {
	var err error
	count := 1
	if len(args) > 0 {
		count, err = strconv.Atoi(args[0])
		if err != nil {
			return ctx.Send(args[0] + " is not a number.")
		}
	}
	if count == 1 {
		if rand.Int()&1 == 0 {
			return ctx.Send("Heads")
		}
		return ctx.Send("Tails")
	}
	heads := 0
	for i := 0; i < count; i++ {
		if rand.Int()&1 == 0 {
			heads++
		}
	}
	return ctx.Send(strconv.Itoa(heads) + " heads")
}

// ~!roll [count]
// Rolls a six-sided die
// If count is provided, rolls multiple.
func roll(ctx commands.Context, args []string) error {
	var err error
	count := 1
	if len(args) > 0 {
		count, err = strconv.Atoi(args[0])
		if err != nil {
			return ctx.Send(args[0] + " is not a number.")
		}
	}
	total := count
	for i := 0; i < count; i++ {
		total += rand.Intn(6)
	}
	return ctx.Send("Rolled " + strconv.Itoa(total))
}

var ballresp = [...]string{"Yes", "No", "Maybe so", "Hell yes", "Hell no", "Get back to me when jlort jlort 3 comes out", "Not until you get negative kek", "Of course", "Go to jail, go directly... oh, wrong game.", "When I learn to talk, I'll tell you", "Turn around.", "You? HAHAHAHAHA no", "aaa eee ooo", "500 Internal Server Error", "404 Possibility Not Found", "302 Possibility Found"}

// ~!8ball <thing>
// Ask the magic 8 ball a serious question, and get a stupid answer.
// No, really. This one hates your guts. And my guts.
func eightball(ctx commands.Context, args []string) error {
	if len(args) == 0 {
		return ctx.Send("Come on, ask me something.")
	}
	return ctx.Send(ballresp[rand.Intn(len(ballresp))])
}

// ~!howbrit [user]
// @Hidden
// Checks someone's Britishness
// If none is specified, gives the Britishness of the command runner.
func howbrit(ctx commands.Context, args []string) error {
	target := ctx.Member
	if len(args) != 0 && ctx.GuildID != "" {
		var err error
		other := strings.Join(args, " ")
		target, err = commands.FindMember(ctx.Bot, other, ctx.GuildID)
		if err != nil {
			return err
		}
		if target == nil {
			return ctx.Send("No such member " + other)
		}
	}
	name := commands.DisplayName(target)
	uid, _ := strconv.ParseUint(target.User.ID, 10, 64)
	result := queryBrit.QueryRow(uid)
	var amnt int
	if result.Scan(&amnt) != nil && !target.User.Bot {
		amnt = 50
	}
	return ctx.Send(fmt.Sprintf("%s is %d%% British", name, amnt))
}

// ~!brit <user>
// @Hidden
// @GuildOnly
// Calls someone British
// This increases thier British score by 2.
// If in a duel and no user is specified, your opponent will be called British.
// In that case, your opponent's British score will not increase until after the duel.
func brit(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command can only be used in servers.")
	}
	var target *discordgo.Member
	var other string
	var err error
	duelLock.RLock()
	curDuel := duels[ctx.Author.ID]
	duelLock.RUnlock()
	if len(args) == 0 {
		if curDuel != nil {
			target = curDuel.other(ctx.Author.ID)
			other = commands.DisplayName(target)
		} else {
			return ctx.Send("Usage: ~!brit <user>")
		}
	} else {
		other = strings.Join(args, " ")
		target, err = commands.FindMember(ctx.Bot, other, ctx.GuildID)
		if err != nil {
			return err
		}
		if target == nil {
			return ctx.Send("No such member " + other)
		}
	}
	if target.User.ID == ctx.Author.ID {
		return ctx.Send("Self-deprecation isn't allowed here.")
	}
	if target.User.Bot {
		return ctx.Send("Bots do not have nationalities.")
	}
	if curDuel != nil && curDuel.isParticipant(target.User.ID) {
		myname := commands.DisplayName(curDuel.said(ctx.Author.ID))
		return ctx.Send(fmt.Sprintf("%s taunts %s before the match!", myname, other))
	}
	duelLock.RLock()
	until := time.Until(cooldown[ctx.Author.ID])
	duelLock.RUnlock()
	if until > 0 {
		return ctx.Send(fmt.Sprintf("Wait %.0fs for cooldown.", until.Seconds()))
	}
	duelLock.Lock()
	cooldown[ctx.Author.ID] = time.Now().Add(3 * time.Minute)
	duelLock.Unlock()
	myname := commands.DisplayName(ctx.Member)
	uid, _ := strconv.ParseUint(target.User.ID, 10, 64)
	incrBrit.Exec(uid, 2)
	return ctx.Send(fmt.Sprintf("%s calls %s British!", myname, other))
}

// ~!duel <user>
// @Hidden
// @GuildOnly
// Challenges someone to an ungentlemanly duel
// The winner will be decided after 30s.
// If you're certain of victory, you can call the other person British before the duel is decided. To do so, run ~!brit with no arguments.
// Once the winner is decided, the Brit scores will be adjusted based on whether or not each called the other a Brit.
func duel(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command can only be used in servers.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!duel <@user>")
	}
	if args[0][0] != '<' || args[0][1] != '@' || args[0][len(args[0])-1] != '>' {
		return ctx.Send("You have to @ someone to duel them!")
	}
	target, err := ctx.Bot.GuildMember(ctx.GuildID, args[0][3:len(args[0])-1])
	if err != nil {
		return err
	}
	if target.User.ID == ctx.Author.ID {
		return ctx.Send("Cannot duel yourself!")
	}
	if target.User.Bot {
		return ctx.Send("Cannot duel a bot!")
	}
	duelLock.RLock()
	if duels[ctx.Author.ID] != nil {
		duelLock.RUnlock()
		return ctx.Send("Already in a duel!")
	}
	if duels[target.User.ID] != nil {
		duelLock.RUnlock()
		return ctx.Send(commands.DisplayName(target) + " is already in a duel!")
	}
	until := time.Until(cooldown[ctx.Author.ID])
	if until > 0 {
		duelLock.RUnlock()
		return ctx.Send(fmt.Sprintf("Wait %.0fs for cooldown.", until.Seconds()))
	}
	duelLock.RUnlock()
	duelLock.Lock()
	cooldown[ctx.Author.ID] = time.Now().Add(5 * time.Minute)
	curDuel := &duelObj{target, ctx.Member, false, false}
	duels[ctx.Author.ID] = curDuel
	duels[target.User.ID] = curDuel
	duelLock.Unlock()
	embed := new(discordgo.MessageEmbed)
	embed.Title = "**Duel!**"
	embed.Footer = &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("%s vs %s", commands.DisplayName(ctx.Member), commands.DisplayName(target))}
	embed.Color = 0x992d22
	embed.Description = "Duel will occur in 30s. If you're sure of victory, take this time to call the other person British!"
	msg, err := ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
	if err == nil {
		time.AfterFunc(30*time.Second, func() { duelCleanup(curDuel, msg, ctx) })
	}
	return err
}

func duelCleanup(curDuel *duelObj, msg *discordgo.Message, ctx commands.Context) {
	embed := msg.Embeds[0]
	winRng := rand.Intn(8)
	var winner, loser *discordgo.Member
	if winRng&4 == 0 {
		winner = curDuel.P1
		loser = curDuel.P2
	} else {
		winner = curDuel.P2
		loser = curDuel.P1
	}
	duelLock.Lock()
	delete(duels, winner.User.ID)
	delete(duels, loser.User.ID)
	duelLock.Unlock()
	embed.Description = commands.DisplayName(winner) + " won the duel!"
	winDiff := -4
	losDiff := 8
	if curDuel.didSay(winner) {
		winDiff -= 4
		losDiff += 4
	}
	if curDuel.didSay(loser) {
		losDiff += 8
	}
	tx, _ := ctx.Database.Begin()
	stmt := tx.Stmt(incrBrit)
	cid, _ := strconv.ParseUint(winner.User.ID, 10, 64)
	stmt.Exec(cid, winDiff)
	cid, _ = strconv.ParseUint(loser.User.ID, 10, 64)
	stmt.Exec(cid, losDiff)
	stmt.Close()
	err := tx.Commit()
	if err != nil {
		log.Error(err)
	}
	ctx.Bot.ChannelMessageEditEmbed(ctx.ChanID, msg.ID, embed)
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also initializes the cooldown and duel maps and loads the scores from disk.
func Init(_ *discordgo.Session) {
	cooldown = make(map[string]time.Time)
	duels = make(map[string]*duelObj)
	commands.RegisterCommand(flip, "flip")
	commands.RegisterCommand(roll, "roll")
	commands.RegisterCommand(howbrit, "howbrit")
	commands.RegisterCommand(brit, "brit")
	commands.RegisterCommand(duel, "duel")
	commands.RegisterCommand(eightball, "8ball")
	db := commands.GetDatabase()
	var err error
	queryBrit, err = db.Prepare("SELECT score FROM brit WHERE uid=?001;")
	if err != nil {
		log.Error(err)
	}
	incrBrit, err = db.Prepare(`
	INSERT INTO brit (uid, score) VALUES (?001, 50 + ?002)
		ON CONFLICT (uid) DO
		UPDATE SET score = IIF(?002 > 0,
			IIF(score + ?002 > 100, 100, score + ?002),
			IIF(score + ?002 < 0, 0, score + ?002));
	`)
	if err != nil {
		log.Error(err)
	}
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the scores to disk.
func Cleanup(_ *discordgo.Session) {
	queryBrit.Close()
	incrBrit.Close()
}
