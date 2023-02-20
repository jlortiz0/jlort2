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

package zip

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
)

const tsFormat = "Jan _2 3:04 PM"
const dateFormat = "[Jan _2 2006]\n"
const timeFormat = "[15:04]"

// ~!chatlog [flags]
// @Alias logall
// Puts messages in a file
// Reply to the first message you want to be logged
// Alternative, run as ~!logall to log all messages in the channel
// The optional flags parameter is a number that provides more control over what gets logged.
// To calculate it, add the numbers for the options you want together.
// 1: Don't log empty messages, even if they contain attachments or embeds
// 2: Don't log bot messages
// 4: Don't log attachments
// 8: Don't log embeds
func chatlog(ctx commands.Context, args []string) error {
	if ctx.Message.Type != discordgo.MessageTypeReply && ctx.InvokedWith != "logall" {
		return ctx.Send("Reply to a message to start logging there or use ~!logall")
	}
	var err error
	flags := 0
	if len(args) > 0 {
		if args[0] == "help" {
			return ctx.Send("Flags are numbers representing bits. To use, add up the numbers for the flags you want to enable and use the result as the 3rd argument to ~!chatlog\n1 - Don't include messages without text, even if they have attachments\n2 - Don't include bot messages\n4 - Don't include attachments (but retain the messages)\n8 - Don't include embeds")
		}
		flags, err = strconv.Atoi(args[2])
		if err != nil {
			return ctx.Send(args[2] + " is not a number.")
		}
	}
	// 1 noempty, 2 nobot, 4 noattach, 8 noembed
	output := bytes.NewBufferString("Discord Text Archive created on ")
	output.WriteString(time.Now().Format(tsFormat))
	output.WriteString(" by ")
	output.WriteString(ctx.Author.Username)
	channel, err := ctx.State.Channel(ctx.ChanID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	if ctx.GuildID != "" {
		guild, err := ctx.State.Guild(ctx.GuildID)
		if err != nil {
			return fmt.Errorf("failed to get guild: %w", err)
		}
		output.WriteString("\nServer: ")
		output.WriteString(guild.Name)
		output.WriteString("\nChannel: #")
		output.WriteString(channel.Name)
		if channel.Topic != "" {
			output.WriteString("\nTopic: ")
			output.WriteString(channel.Topic)
		}
	}
	output.WriteByte('\n')
	output.WriteByte('\n')
	ctx.Bot.ChannelTyping(ctx.ChanID)
	lastMsg := "0"
	if ctx.Message.Type == discordgo.MessageTypeReply {
		if ctx.Message.MessageReference.ChannelID != ctx.ChanID {
			return ctx.Send("Cannot log a crossposted message.")
		}
		temp, _ := strconv.ParseUint(ctx.Message.MessageReference.MessageID, 10, 64)
		lastMsg = strconv.FormatUint(temp-1, 10)
	}
	nicks := make(map[string]string)
	var lastDay int
	for {
		toProc, err := ctx.Bot.ChannelMessages(ctx.ChanID, 100, "", lastMsg, "")
		if err != nil {
			return fmt.Errorf("failed to get channel messages: %w", err)
		}
		if len(toProc) == 0 {
			break
		}
		lastMsg = toProc[0].ID
		for i := len(toProc) - 1; i >= 0; i-- {
			v := toProc[i]
			if v.Type == discordgo.MessageTypeThreadStarterMessage {
				v2, err := ctx.State.Message(v.MessageReference.ChannelID, v.MessageReference.MessageID)
				if err != nil {
					v2, _ = ctx.Bot.ChannelMessage(v.MessageReference.ChannelID, v.MessageReference.MessageID)
					if v2 == nil {
						continue
					}
				}
				v = v2
			}
			if v.Type != discordgo.MessageTypeDefault && v.Type != discordgo.MessageTypeReply && v.Type != discordgo.MessageTypeChatInputCommand && v.Type != discordgo.MessageTypeContextMenuCommand {
				continue
			}
			if v.ID == ctx.Message.ID {
				continue
			}
			if v.Author.Bot && flags&2 != 0 {
				continue
			}
			if flags&4 != 0 {
				v.Attachments = nil
			}
			if flags&8 != 0 {
				v.Embeds = nil
			}
			if v.Content == "" {
				if flags&1 != 0 || (len(v.Attachments) == 0 && len(v.Embeds) == 0) {
					continue
				}
			}
			if nicks[v.Author.ID] == "" {
				nicks[v.Author.ID] = v.Author.Username
				if ctx.GuildID != "" {
					mem, err := ctx.State.Member(ctx.GuildID, v.Author.ID)
					if err == nil && mem.Nick != "" {
						nicks[v.Author.ID] = mem.Nick
					}
				}
			}
			t := v.Timestamp.In(time.Local)
			if t.YearDay() != lastDay {
				output.WriteString(t.Format(dateFormat))
				lastDay = t.YearDay()
			}
			output.WriteString(t.Format(timeFormat))
			output.WriteString(" <")
			output.WriteString(nicks[v.Author.ID])
			output.WriteString("> ")
			output.WriteString(v.ContentWithMentionsReplaced())
			if v.Pinned {
				output.WriteString("\n - Pinned")
			}
			for _, attach := range v.Attachments {
				output.WriteString("\n - Attachment: ")
				output.WriteString(attach.URL)
			}
			for _, attach := range v.Embeds {
				if attach.Image != nil {
					output.WriteString("\n - Image: ")
					output.WriteString(attach.Image.URL)
				} else {
					output.WriteString("\n - Embed: ")
					if attach.URL != "" {
						output.WriteString(attach.URL)
					} else {
						output.WriteString(attach.Title)
						output.WriteString(" (")
						output.WriteString(attach.Description)
						output.WriteByte(')')
					}
				}
			}
			output.WriteByte('\n')
		}
	}
	_, err = ctx.Bot.ChannelFileSend(ctx.ChanID, "jlort-jlort-"+channel.Name+".txt", output)
	return err
}

// ~!zip
// @Alias archive
// @Hidden
// Zips all attachments and embeds in the channel.
// This command is hidden because the zip file is invariably so big it can't be uploaded.
func archive(ctx commands.Context, _ []string) error {
	err := ctx.Send("Parsing messages...")
	if err != nil {
		return err
	}
	ctx.Bot.ChannelTyping(ctx.ChanID)
	type FileInfo struct {
		Filename  string
		URL       string
		Timestamp time.Time
	}
	files := make([]FileInfo, 0, 500)
	lastMsg := ""
	for {
		toProc, err := ctx.Bot.ChannelMessages(ctx.ChanID, 100, lastMsg, "", "")
		if err != nil {
			return err
		}
		if len(toProc) == 0 {
			break
		}
		for _, v := range toProc {
			ts := v.Timestamp
			for _, a := range v.Attachments {
				files = append(files, FileInfo{a.ID + "-" + a.Filename, a.URL, ts})
			}
			for _, a := range v.Embeds {
				if a.Image != nil && a.Image.URL != "" {
					s := strings.Split(a.Image.URL, "/")
					files = append(files, FileInfo{v.ID + "-" + s[len(s)-1], a.Image.URL, ts})
				}
			}
			lastMsg = v.ID
		}
	}
	err = ctx.Send(fmt.Sprintf("Found %d messages, zipping...", len(files)))
	if err != nil {
		return err
	}
	fName := fmt.Sprintf("%s/jlort-jlort-%d.zip", os.TempDir(), time.Now().Unix())
	f, err := os.OpenFile(fName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tempfile %s: %w", fName, err)
	}
	defer f.Close()
	buf := bufio.NewWriter(f)
	zWriter := zip.NewWriter(buf)
	for _, fInfo := range files {
		header := new(zip.FileHeader)
		header.Name = fInfo.Filename
		header.Modified = fInfo.Timestamp
		s := strings.Split(fInfo.Filename, ".")
		switch s[len(s)-1] {
		case "png", "jpg", "mp3", "ogg", "zip", "7z", "gz", "m4a", "pdf", "docx", "mp4", "mov":
			//Do not compress, default Store
		default:
			header.Method = 8 // Deflate
		}
		fWriter, err := zWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to append to zip: %w", err)
		}
		resp, err := ctx.Bot.Client.Get(fInfo.URL)
		if err != nil {
			fmt.Println(err)
			continue
		}
		_, err = io.Copy(fWriter, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to append to zip: %w", err)
		}
	}
	err = zWriter.Close()
	if err != nil {
		return fmt.Errorf("failed to close zip: %w", err)
	}
	err = buf.Flush()
	if err != nil {
		return fmt.Errorf("failed to close zip: %w", err)
	}
	return ctx.Send("Zip complete! Ask jlortiz for " + fName)
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
func Init(_ *discordgo.Session) {
	commands.RegisterCommand(chatlog, "chatlog")
	commands.RegisterCommand(chatlog, "logall")
	commands.RegisterCommand(archive, "zip")
	commands.RegisterCommand(archive, "archive")
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
func Cleanup(_ *discordgo.Session) {}
