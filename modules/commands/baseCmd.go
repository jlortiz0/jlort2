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

package commands

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
	"jlortiz.org/jlort2/modules/log"
)

// ~!purge [user]
// Delete a user's messages
// If not specified, or if we are in a DM, purges messages by me.
// If used in a server, will also purge messages with the prefix ~!
// For this command to work in servers, I need the Manage Messages permission.
// To specify a user other than me, you need the Manage Messages permission.
// Due to Discord limitations, this only scans the 100 most recent messages.
func purge(ctx *Context) error {
	ctx.RespondDelayed(true)
	if ctx.GuildID == "" {
		msgs, err := ctx.Bot.ChannelMessages(ctx.ChannelID, 100, "", "", "")
		if err != nil {
			return fmt.Errorf("failed to get message list: %w", err)
		}
		todel := make([]string, 0, len(msgs)-1)
		for _, msg := range msgs {
			if msg.Author.ID == ctx.Me.ID {
				todel = append(todel, msg.ID)
			}
		}
		for _, msg := range todel {
			err = ctx.Bot.ChannelMessageDelete(ctx.ChannelID, msg)
			if err != nil {
				return fmt.Errorf("failed to delete message: %w", err)
			}
		}
		return ctx.RespondPrivate(fmt.Sprintf("Purged %d messages", len(todel)))
	}
	if ctx.AppPermissions&discordgo.PermissionManageMessages == 0 {
		return ctx.RespondPrivate("I need the Manage Messages permission to use this command.")
	}
	target := ctx.Me.ID
	d := ctx.ApplicationCommandData()
	if d.TargetID != "" {
		target = d.TargetID
	} else if len(d.Options) != 0 {
		args := d.Options[0].UserValue(nil)
		// Uncomment this if Perms(discordgo.ManageMessages) is removed for this command
		// if args.ID != target {
		// 	perms, err := ctx.State.UserChannelPermissions(ctx.User.ID, ctx.ChannelID)
		// 	if err != nil {
		// 		return fmt.Errorf("failed to get permissions: %w", err)
		// 	}
		// 	if perms&discordgo.PermissionManageMessages == 0 {
		// 		return ctx.RespondPrivate("You need the Manage Messages permission to purge other people's messages.")
		// 	}
		// }
		target = args.ID
	}
	msgs, err := ctx.Bot.ChannelMessages(ctx.ChannelID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("failed to get message list: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -13)
	todel := make([]string, 0, len(msgs)-1)
	for _, msg := range msgs {
		if msg.Timestamp.Before(cutoff) {
			break
		}
		if msg.Author.ID == target {
			todel = append(todel, msg.ID)
		}
	}
	err = ctx.Bot.ChannelMessagesBulkDelete(ctx.ChannelID, todel)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return ctx.RespondPrivate(fmt.Sprintf("Purged %d messages from <#%s>", len(todel), ctx.ChannelID))
}

// ~!ppurge [prefix]
// @GuildOnly
// @ManageMessages
// Delete messages by prefix
// If not specified, the prefix is assumed to be ~!
// You need Manage Messages to use this command.
// Due to library limitations, this only scans the 100 most recent messages, and then only on messages from the last 2 weeks.
func ppurge(ctx *Context) error {
	ctx.RespondDelayed(true)
	prefix := ctx.ApplicationCommandData().Options[0].StringValue()
	msgs, err := ctx.Bot.ChannelMessages(ctx.ChannelID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("failed to get message list: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -13)
	todel := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Timestamp.Before(cutoff) {
			break
		}
		if strings.HasPrefix(msg.Content, prefix) {
			todel = append(todel, msg.ID)
		}
	}
	err = ctx.Bot.ChannelMessagesBulkDelete(ctx.ChannelID, todel)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return ctx.RespondPrivate(fmt.Sprintf("Purged %d messages from <#%s>", len(todel), ctx.ChannelID))
}

// ~!ping
// Get latency
func ping(ctx *Context) error {
	return ctx.RespondPrivate(fmt.Sprintf("Latency: %d ms", ctx.Bot.HeartbeatLatency().Milliseconds()))
}

var updating bool

// ~!gsm <arg1>
// @Hidden
// Run a game server. Do ~!gsm help for help.
// You must be part of a private server to use this command.
func gsm(ctx *Context) error {
	if updating {
		return ctx.RespondPrivate("The servers are currently updating.")
	}
	arg := ctx.ApplicationCommandData().Options[0].StringValue()
	if arg == "update" || arg == "poweroff" {
		if ctx.State.Application.Owner.ID != ctx.User.ID {
			return ctx.RespondPrivate("You do not have access to that command, and never will.")
		}
	}
	bashLoc, err := exec.LookPath("bash")
	if err != nil {
		// This means bash dissapeared at some point between init and now, which is quite panic-worthy
		panic(err)
	}
	if arg == "update" {
		updating = true
		ctx.RespondDelayed(true)
		cmd := exec.Command(bashLoc, "gsm.sh", arg)
		cmd.Start()
		cmd.Wait()
		os.Chtimes("lastUpdate", time.Now(), time.Now())
		updating = false
		return ctx.RespondEdit("Update complete!")
	}
	for _, x := range arg {
		if x < 'A' || x > 'z' || (x > 'Z' && x < 'a') {
			return ctx.RespondPrivate("Illegal character.")
		}
	}
	var cmd *exec.Cmd
	if len(ctx.ApplicationCommandData().Options) == 1 {
		exec.Command(bashLoc, "gsm.sh", arg)
	} else {
		exec.Command(bashLoc, "gsm.sh", arg, "silent")
	}
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run gsm: %w", err)
	}
	if len(out) == 0 {
		log.Warn("Empty output from /gsm " + arg)
		return ctx.RespondEmpty()
	}
	return ctx.Respond(string(out))
}

func gsmAutocomplete(ctx *Context) []*discordgo.ApplicationCommandOptionChoice {
	pre := ctx.ApplicationCommandData().Options[0].StringValue()
	fList, err := os.ReadDir("/home/McServer/")
	if err != nil {
		return nil
	}
	output := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(fList)+3)
	for _, x := range fList {
		if x.IsDir() && strings.HasPrefix(x.Name(), pre) {
			output = append(output, &discordgo.ApplicationCommandOptionChoice{Name: x.Name(), Value: x.Name()})
		}
	}
	if strings.HasPrefix("stop", pre) {
		output = append(output, &discordgo.ApplicationCommandOptionChoice{Name: "stop", Value: "stop"})
	} else if strings.HasPrefix("ping", pre) {
		output = append(output, &discordgo.ApplicationCommandOptionChoice{Name: "ping", Value: "ping"})
	} else if strings.HasPrefix("ip", pre) {
		output = append(output, &discordgo.ApplicationCommandOptionChoice{Name: "ip", Value: "ip"})
	}
	return output
}

var buildDate string
var verNum string

// ~!version
// Get bot info
func version(ctx *Context) error {
	return ctx.RespondPrivate(ctx.State.Application.Name + " " + verNum + " running on discordgo v" + discordgo.VERSION + " " + runtime.Version() + "\nBuilt: " + buildDate)
}

// ~!flip [times]
// Flips a coin
// If times is provided, flips multiple coins.
func flip(ctx *Context) error {
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
func roll(ctx *Context) error {
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

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also initializes the command map. This means that calling commands.Init will unregister any existing commands.
func Init(self *discordgo.Session) {
	cmdMap = make(map[string]cmdMapEntry, 64)
	var err error
	db, err = sql.Open("sqlite3", "persistent.db")
	if err != nil {
		log.Error(err)
		return
	}
	db.Exec("pragma journal_mode = WAL; pragma synchronous = normal; pragma mmap_size = 4194304;")
	PrepareCommand("purge", "Delete messages by user").Perms(discordgo.PermissionManageMessages).Register(purge, []*discordgo.ApplicationCommandOption{
		NewCommandOption("user", "User to purge, default me").AsUser().Finalize(),
	})
	PrepareCommand("Purge Messages", "").AsUser().Guild().Perms(discordgo.PermissionManageMessages).Register(purge, nil)
	PrepareCommand("ppurge", "Delete messages by prefix").Guild().Perms(discordgo.PermissionManageMessages).Register(ppurge, []*discordgo.ApplicationCommandOption{
		NewCommandOption("prefix", "Messages that start with this will be deleted").AsString().Required().Finalize(),
	})
	PrepareCommand("ping", "Get bot latency").Register(ping, nil)
	PrepareCommand("version", "Get version info").Register(version, nil)
	PrepareCommand("flip", "Flip one or more coins").Register(flip, []*discordgo.ApplicationCommandOption{
		NewCommandOption("coins", "How many coins to flip").AsInt().SetMinMax(1, 255).Finalize(),
	})
	PrepareCommand("roll", "Roll one or more D6").Register(roll, []*discordgo.ApplicationCommandOption{
		NewCommandOption("dice", "How many dice to roll").AsInt().SetMinMax(1, 255).Finalize(),
	})
	if runtime.GOOS != "windows" {
		st, _ := os.Stat("gsm.sh")
		if st != nil && st.Mode()&0100 != 0 {
			PrepareCommand("gsm", "Game Server Manager").Guild().Auto(gsmAutocomplete).Gsm().Register(gsm, []*discordgo.ApplicationCommandOption{
				NewCommandOption("arg", "Run /gsm help for a list of arguments").AsString().Required().Auto().Finalize(),
				NewCommandOption("silent", "If true, server startup will not be announced, default false").AsBool().Finalize(),
			})
		}
	}
	info, err := os.Stat("lastUpdate")
	if err == nil {
		since := time.Since(info.ModTime())
		if since > 12*time.Hour {
			var cmd *exec.Cmd
			if runtime.GOOS != "windows" {
				bashLoc, _ := exec.LookPath("bash")
				cmd = exec.Command(bashLoc, "gsm.sh", "update")
			} else {
				pipLoc, _ := exec.LookPath("pip")
				cmd = exec.Command(pipLoc, "install", "--user", "-q", "-U", "yt-dlp")
			}
			cmd.Start()
			updating = true
			go func() {
				cmd.Wait()
				os.Chtimes("lastUpdate", time.Now(), time.Now())
				updating = false
			}()
		}
	} else {
		f, _ := os.OpenFile("lastUpdate", os.O_WRONLY|os.O_CREATE, 0)
		if f != nil {
			f.Close()
		}
	}
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
func Cleanup(_ *discordgo.Session) {
	db.Exec("PRAGMA optimize;")
	err := db.Close()
	if err != nil {
		log.Error(err)
	}
}
