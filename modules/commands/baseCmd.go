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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/log"
)

const gsmServerID = ""

// ~!echo <message>
// Says stuff back
func echo(ctx Context, args []string) error {
	out := strings.Join(args, " ")
	if out == "" {
		out = "You have to type something, you know."
	}
	err := ctx.Send(out)
	if err != nil {
		return err
	}
	if ctx.GuildID != "" {
		perms, err := ctx.State.UserChannelPermissions(ctx.Me.ID, ctx.Message.ChannelID)
		if err != nil {
			return fmt.Errorf("Failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageMessages != 0 {
			return ctx.Bot.ChannelMessageDelete(ctx.ChanID, ctx.Message.ID)
		}
	}
	return nil
}

// ~!purge [user]
// Delete a user's messages
// If not specified, or if we are in a DM, purges messages by me.
// If used in a server, will also purge messages with the prefix ~!
// For this command to work in servers, I need the Manage Messages permission.
// To specify a user other than me, you need the Manage Messages permission.
// Due to library limitations, this only scans the 100 most recent messages.
func purge(ctx Context, args []string) error {
	if ctx.GuildID == "" {
		ctx.Bot.ChannelTyping(ctx.ChanID)
		msgs, err := ctx.Bot.ChannelMessages(ctx.ChanID, 100, "", "", "")
		if err != nil {
			return fmt.Errorf("Failed to get message list: %w", err)
		}
		todel := make([]string, 0, len(msgs)-1)
		for _, msg := range msgs {
			if msg.Author.ID == ctx.Me.ID {
				todel = append(todel, msg.ID)
			}
		}
		for _, msg := range todel {
			err = ctx.Bot.ChannelMessageDelete(ctx.ChanID, msg)
			if err != nil {
				return fmt.Errorf("Failed to delete message: %w", err)
			}
		}
		msg, err := ctx.Bot.ChannelMessageSend(ctx.ChanID, "Purged "+strconv.Itoa(len(todel))+" messages")
		time.AfterFunc(3*time.Second, func() { ctx.Bot.ChannelMessageDelete(ctx.ChanID, msg.ID) })
		return err
	}
	perms, err := ctx.State.UserChannelPermissions(ctx.Me.ID, ctx.Message.ChannelID)
	if err != nil {
		return fmt.Errorf("Failed to get permissions: %w", err)
	}
	if perms&discordgo.PermissionManageMessages == 0 {
		return ctx.Send("I need the Manage Messages permission to use this command.")
	}
	target := ctx.Me.ID
	if len(args) > 0 {
		perms, err = ctx.State.MessagePermissions(ctx.Message)
		if err != nil {
			return fmt.Errorf("Failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageMessages == 0 {
			return ctx.Send("You need the Manage Messages permission to clear other users' messages.")
		}
		other := strings.Join(args, " ")
		targetMem, err := FindMember(ctx.Bot, other, ctx.GuildID)
		if err != nil {
			return err
		}
		if targetMem == nil {
			return ctx.Send("No such member " + other)
		}
		target = targetMem.User.ID
	}
	msgs, err := ctx.Bot.ChannelMessages(ctx.ChanID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("Failed to get message list: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -13)
	todel := make([]string, 0, len(msgs)-1)
	for _, msg := range msgs {
		if msg.Timestamp.Before(cutoff) {
			break
		}
		if msg.Author.ID == target || strings.HasPrefix(msg.Content, "~!") {
			todel = append(todel, msg.ID)
		}
	}
	err = ctx.Bot.ChannelMessagesBulkDelete(ctx.ChanID, todel)
	if err != nil {
		return fmt.Errorf("Failed to delete messages: %w", err)
	}
	channel, err := ctx.State.Channel(ctx.ChanID)
	if err != nil {
		return fmt.Errorf("Failed to get channel info: %w", err)
	}
	msg, err := ctx.Bot.ChannelMessageSend(ctx.ChanID, "Purged "+strconv.Itoa(len(todel))+" messages from "+channel.Name)
	time.AfterFunc(3*time.Second, func() { ctx.Bot.ChannelMessageDelete(ctx.ChanID, msg.ID) })
	return err
}

// ~!ppurge [prefix]
// @GuildOnly
// Delete messages by prefix
// If not specified, the prefix is assumed to be ~!
// You need Manage Messages to use this command.
// Due to library limitations, this only scans the 100 most recent messages, and then only on messages from the last 2 weeks.
func ppurge(ctx Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	perms, err := ctx.State.MessagePermissions(ctx.Message)
	if err != nil {
		return fmt.Errorf("Failed to get permissions: %w", err)
	}
	if perms&discordgo.PermissionManageMessages == 0 {
		return ctx.Send("You need the Manage Messages permission to clear other users' messages.")
	}
	prefix := "~!"
	if len(args) == 0 {
		prefix = strings.Join(args, " ")
	}
	msgs, err := ctx.Bot.ChannelMessages(ctx.ChanID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("Failed to get message list: %w", err)
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
	err = ctx.Bot.ChannelMessagesBulkDelete(ctx.ChanID, todel)
	if err != nil {
		return fmt.Errorf("Failed to delete messages: %w", err)
	}
	channel, err := ctx.State.Channel(ctx.ChanID)
	if err != nil {
		return fmt.Errorf("Failed to get channel info: %w", err)
	}
	msg, err := ctx.Bot.ChannelMessageSend(ctx.ChanID, "Purged "+strconv.Itoa(len(todel))+" messages from "+channel.Name)
	time.AfterFunc(3*time.Second, func() { ctx.Bot.ChannelMessageDelete(ctx.ChanID, msg.ID) })
	return err
}

// ~!ping
// Get latency
func ping(ctx Context, _ []string) error {
	return ctx.Send(fmt.Sprintf("Latency: %d ms", ctx.Bot.HeartbeatLatency().Milliseconds()))
}

// ~!nh <6 digits>
// @Alias nhentai
// @NSFW
// Info about !?
func nh(ctx Context, args []string) error {
	channel, err := ctx.State.Channel(ctx.ChanID)
	if err != nil {
		return err
	}
	if channel.Type != discordgo.ChannelTypeDM && !channel.NSFW {
		return ctx.Send("This command is restricted to NSFW channels and DMs.")
	}
	if len(args) == 0 {
		return ctx.Send("~!nh <6 digits>")
	}
	if _, err := strconv.Atoi(args[0]); err != nil {
		return ctx.Send(args[0] + " is not a number.")
	}
	resp, err := ctx.Bot.Client.Get("https://nhentai.net/api/gallery/" + args[0])
	if err != nil {
		return fmt.Errorf("Failed to fetch doujin: %w", err)
	}
	defer resp.Body.Close()
	type nhdata struct {
		Title struct {
			English string
		}
		MediaID  string `json:"media_id"`
		NumPages int    `json:"num_pages"`
		Images   struct {
			Cover struct {
				T string
			}
		}
		Tags []struct {
			Name string
			Type string
			URL  string
		}
	}
	data := nhdata{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return fmt.Errorf("Failed to read doujin: %w", err)
	}
	output := new(discordgo.MessageEmbed)
	output.URL = "https://nhentai.net/g/" + args[0]
	output.Type = discordgo.EmbedTypeImage
	output.Title = data.Title.English
	var coverext string
	switch data.Images.Cover.T {
	case "j":
		coverext = "jpg"
	case "p":
		coverext = "png"
	case "g":
		coverext = "gif"
	}
	output.Image = &discordgo.MessageEmbedImage{URL: fmt.Sprintf("https://t.nhentai.net/galleries/%s/cover.%s", data.MediaID, coverext)}
	tagStr := new(strings.Builder)
	var author *discordgo.MessageEmbedAuthor
	for _, tag := range data.Tags {
		if tag.Type == "artist" {
			author = &discordgo.MessageEmbedAuthor{URL: "https://nhentai.net/" + tag.URL, Name: tag.Name}
		}
		if tagStr.Len() != 0 {
			tagStr.WriteString(", ")
		}
		tagStr.WriteString(tag.Name)
	}
	if author != nil {
		output.Author = author
	}
	output.Fields = []*discordgo.MessageEmbedField{
		{Name: "Pages", Value: strconv.Itoa(data.NumPages), Inline: true},
		{Name: "Tags", Value: tagStr.String(), Inline: true},
	}
	_, err = ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, output)
	return err
}

var updating bool

// ~!gsm <arg1> [arg2]
// @Hidden
// Run a game server. Do ~!gsm help for help.
// You must be part of a private server to use this command.
func gsm(ctx Context, args []string) error {
	if _, err := ctx.State.Member(gsmServerID, ctx.Author.ID); err != nil {
		return ctx.Send("You do not have access to these servers.")
	}
	if updating {
		return ctx.Send("The servers are currently updating.")
	}
	if len(args) > 0 && (args[0] == "update" || args[0] == "poweroff") {
		app, err := ctx.Bot.Application("@me")
		if err != nil {
			return fmt.Errorf("Failed to get app info: %w", err)
		}
		if app.Owner.ID != ctx.Author.ID {
			return ctx.Send("You do not have access to that command, and never will.")
		}
	}
	bashLoc, err := exec.LookPath("bash")
	if err != nil {
		// This means bash dissapeared at some point between init and now, which is quite panic-worthy
		panic(err)
	}
	if len(args) > 0 && args[0] == "update" {
		cmd := exec.Command(bashLoc, "gsm.sh", args[0])
		cmd.Start()
		updating = true
		ctx.Send("Updating now, please wait...")
		cmd.Wait()
		os.Chtimes("lastUpdate", time.Now(), time.Now())
		updating = false
		return nil
	}
	var out []byte
	switch len(args) {
	case 0:
		out, err = exec.Command(bashLoc, "gsm.sh").Output()
	case 1:
		for _, x := range args[0] {
			if x < 'A' || x > 'z' || (x > 'Z' && x < 'a') {
				return ctx.Send("Illegal character.")
			}
		}
		out, err = exec.Command(bashLoc, "gsm.sh", args[0]).Output()
	default:
		for _, x := range args[0] {
			if x < 'A' || x > 'z' || (x > 'Z' && x < 'a') {
				return ctx.Send("Illegal character.")
			}
		}
		for _, x := range args[1] {
			if x < 'A' || x > 'z' || (x > 'Z' && x < 'a') {
				return ctx.Send("Illegal character.")
			}
		}
		out, err = exec.Command(bashLoc, "gsm.sh", args[0], args[1]).Output()
	}
	if err != nil {
		return fmt.Errorf("Failed to run gsm: %w", err)
	}
	return ctx.Send(string(out))
}

// ~!invite
// Invite me!
// Bots can't accept regular invites, they must use this link.
// Fun fact: this is the shortest command at only 1 line.
func invite(ctx Context, _ []string) error {
	return ctx.Send("https://discord.com/api/oauth2/authorize?client_id=787850217302130688&permissions=8192&scope=bot")
}

// ~!tpa <user>
// @Alias ~!tpahere
// @Hidden
// @GuildOnly
// Send a teleport request to someone.
// Doesn't really do anything, it's just for fun.
func tpa(ctx Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) == 0 {
		return ctx.Send(fmt.Sprintf("~!%s <user>", ctx.InvokedWith))
	}
	other := strings.Join(args, " ")
	targetMem, err := FindMember(ctx.Bot, other, ctx.GuildID)
	if err != nil {
		return err
	}
	if targetMem == nil {
		return ctx.Send("**Error:** No player by that name.")
	}
	if targetMem.User.Bot {
		return ctx.Send(fmt.Sprintf("**%s** has teleportation disabled.", DisplayName(targetMem)))
	}
	if targetMem.User.ID == ctx.Author.ID {
		return ctx.Send("**Error:** Cannot teleport to yourself.")
	}
	channel, err := ctx.Bot.UserChannelCreate(targetMem.User.ID)
	if err != nil {
		return err
	}
	err = ctx.Send(fmt.Sprintf("Request sent to **%s**.", DisplayName(targetMem)))
	if err != nil {
		return err
	}
	_, err = ctx.Bot.ChannelMessageSend(channel.ID, fmt.Sprintf("**%s** has requested that you teleport to <#%s>.\nTo teleport, type **~!tpaccept**.\nTo deny this request, type **~!tpdeny**.", DisplayName(ctx.Member), ctx.ChanID))
	return err
}

func avatar(ctx Context, _ []string) error {
	app, err := ctx.Bot.Application("@me")
	if err != nil {
		return fmt.Errorf("Failed to get app info: %w", err)
	}
	if app.Owner.ID != ctx.Author.ID {
		return ctx.Send("You do not have access to that command, and never will.")
	}
	if len(ctx.Message.Attachments) == 0 {
		return ctx.Send("Attach an image in png or jpg format.")
	}
	URL := ctx.Message.Attachments[0].URL
	ind := strings.LastIndexByte(URL, '.')
	ext := URL[ind+1:]
	ind = strings.IndexByte(ext, '?')
	if ind != -1 {
		ext = ext[:ind]
	}
	if ext != "png" && ext != "jpg" && ext != "jpeg" {
		return ctx.Send("Invalid format.")
	}
	resp, err := http.Get(URL)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 > 3 {
		return errors.New(resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	prefix := "data:image/jpeg;base64,"
	if ext == "png" {
		prefix = "data:image/png;base64,"
	}
	_, err = ctx.Bot.UserUpdate("", prefix+base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return err
	}
	return ctx.Send("Updated avatar.")
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also initializes the command map. This means that calling commands.Init will unregister any existing commands.
func Init(_ *discordgo.Session) {
	cmdMap = make(map[string]Command, 72)
	helpMap = make(map[string]*helpData, 72)
	err := loadHelpData()
	if err != nil {
		log.Error(err)
		return
	}
	RegisterCommand(help, "help")
	RegisterCommand(echo, "echo")
	RegisterCommand(purge, "purge")
	RegisterCommand(ppurge, "ppurge")
	RegisterCommand(ping, "ping")
	RegisterCommand(ping, "latency")
	RegisterCommand(nh, "nh")
	RegisterCommand(nh, "nhentai")
	RegisterCommand(invite, "invite")
	RegisterCommand(version, "version")
	RegisterCommand(tpa, "tpa")
	RegisterCommand(tpa, "tpahere")
	RegisterCommand(avatar, "_avatar")
	if runtime.GOOS != "windows" {
		RegisterCommand(gsm, "gsm")
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
