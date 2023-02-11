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
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// ~!echo <message>
// Says stuff back
func echo(ctx Context) error {
	return ctx.RespondPrivate(ctx.ApplicationCommandData().Options[0].StringValue())
}

// ~!purge [user]
// Delete a user's messages
// If not specified, or if we are in a DM, purges messages by me.
// If used in a server, will also purge messages with the prefix ~!
// For this command to work in servers, I need the Manage Messages permission.
// To specify a user other than me, you need the Manage Messages permission.
// Due to Discord limitations, this only scans the 100 most recent messages.
func purge(ctx Context) error {
	if ctx.GuildID == "" {
		// ctx.DelayedRespond()
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
	perms, err := ctx.State.UserChannelPermissions(ctx.Me.ID, ctx.ChannelID)
	if err != nil {
		return fmt.Errorf("failed to get permissions: %w", err)
	}
	if perms&discordgo.PermissionManageMessages == 0 {
		return ctx.RespondPrivate("I need the Manage Messages permission to use this command.")
	}
	target := ctx.Me.ID
	if len(ctx.ApplicationCommandData().Options) != 0 {
		args := ctx.ApplicationCommandData().Options[0].UserValue(nil)
		target = args.ID
	}
	msgs, err := ctx.Bot.ChannelMessages(ctx.ChannelID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("failed to get message list: %w", err)
	}
	// ctx.DelayedRespond()
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
// Delete messages by prefix
// If not specified, the prefix is assumed to be ~!
// You need Manage Messages to use this command.
// Due to library limitations, this only scans the 100 most recent messages, and then only on messages from the last 2 weeks.
func ppurge(ctx Context) error {
	prefix := ctx.ApplicationCommandData().Options[0].StringValue()
	// ctx.DelayedRespond()
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
func ping(ctx Context) error {
	return ctx.RespondPrivate(fmt.Sprintf("Latency: %d ms", ctx.Bot.HeartbeatLatency().Milliseconds()))
}

var updating bool

// ~!gsm <arg1>
// @Hidden
// Run a game server. Do ~!gsm help for help.
// You must be part of a private server to use this command.
func gsm(ctx Context) error {
	if _, err := ctx.State.Member(GSM_GUILD, ctx.User.ID); err != nil {
		return ctx.RespondPrivate("You do not have access to these servers.")
	}
	if updating {
		return ctx.Respond("The servers are currently updating.")
	}
	arg := ctx.ApplicationCommandData().Options[0].StringValue()
	if arg == "update" || arg == "poweroff" {
		if OWNER_ID != ctx.User.ID {
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
		ctx.DelayedRespond()
		cmd := exec.Command(bashLoc, "gsm.sh", arg)
		cmd.Start()
		cmd.Wait()
		os.Chtimes("lastUpdate", time.Now(), time.Now())
		updating = false
		ctx.EditResponse("Update complete!")
		return nil
	}
	for _, x := range arg {
		if x < 'A' || x > 'z' || (x > 'Z' && x < 'a') {
			return ctx.RespondPrivate("Illegal character.")
		}
	}
	out, err := exec.Command(bashLoc, "gsm.sh", arg).Output()
	if err != nil {
		return fmt.Errorf("failed to run gsm: %w", err)
	}
	return ctx.Respond(string(out))
}

// ~!tpa <user>
// @Alias tpahere
// @Hidden
// @GuildOnly
// Send a teleport request to someone.
// Doesn't really do anything, it's just for fun.
func tpa(ctx Context) error {
	target := ctx.ApplicationCommandData().Options[0].UserValue(ctx.Bot)
	if target.Bot {
		return ctx.RespondPrivate(fmt.Sprintf("**%s** has teleportation disabled.", target.Username))
	}
	if target.ID == ctx.User.ID {
		return ctx.RespondPrivate("**Error:** Cannot teleport to yourself.")
	}
	channel, err := ctx.Bot.UserChannelCreate(target.ID)
	if err != nil {
		return err
	}
	err = ctx.RespondPrivate(fmt.Sprintf("Request sent to **%s**.", target.Username))
	if err != nil {
		return err
	}
	_, err = ctx.Bot.ChannelMessageSend(channel.ID, fmt.Sprintf("**%s** has requested that you teleport to <#%s>.\nTo teleport, type **~!tpaccept**.\nTo deny this request, type **~!tpdeny**.", DisplayName(ctx.Member), ctx.ChannelID))
	return err
}

var buildDate string
var verNum string

// ~!version
// Get bot info
func version(ctx Context) error {
	return ctx.RespondPrivate("jlort jlort " + verNum + " running on discordgo v" + discordgo.VERSION + " " + runtime.Version() + "\nBuilt: " + buildDate)
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also initializes the command map. This means that calling commands.Init will unregister any existing commands.
func Init(self *discordgo.Session) {
	cmdMap = make(map[string]Command, 72)
	PrepareCommand("echo", "Say stuff").Register(echo, []*discordgo.ApplicationCommandOption{
		NewCommandOption("stuff", "say something cool").AsString().Required().Finalize(),
	})
	PrepareCommand("purge", "Delete messages by user").Perms(discordgo.PermissionManageMessages).Register(purge, []*discordgo.ApplicationCommandOption{
		NewCommandOption("user", "User to purge, default me").AsUser().Finalize(),
	})
	PrepareCommand("ppurge", "Delete messages by prefix").Guild().Perms(discordgo.PermissionManageMessages).Register(ppurge, []*discordgo.ApplicationCommandOption{
		NewCommandOption("prefix", "Messages that start with this will be deleted").AsString().Required().Finalize(),
	})
	PrepareCommand("ping", "Get bot latency").Register(ping, nil)
	PrepareCommand("tpa", "Teleport to a user").Guild().Register(tpa, []*discordgo.ApplicationCommandOption{
		NewCommandOption("user", "User to teleport to").AsUser().Required().Finalize(),
	})
	PrepareCommand("version", "Get version info").Register(version, nil)
	if runtime.GOOS != "windows" && GSM_GUILD != "" {
		optionString := new(discordgo.ApplicationCommandOption)
		optionString.Type = discordgo.ApplicationCommandOptionString
		optionString.Name = "arg"
		optionString.Description = "Run /gsm help for a list of arguments"
		// TODO: Make a way to delete old guild commands to avoid traces in case I change GSM_GUILD
		tmp := PrepareCommand("gsm", "Game Server Manager").Guild().Perms(discordgo.PermissionAll)
		RegisterGsmGuildCommand(self, tmp, gsm, []*discordgo.ApplicationCommandOption{
			NewCommandOption("arg", "Run /gsm help for a list of arguments").AsString().Finalize(),
		})
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
	}
	saverVersion++
	go saverLoop()
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
func Cleanup(_ *discordgo.Session) {
	savers = nil
}
