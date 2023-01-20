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

package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const tsFormat = "Jan _2 3:04 PM"
const dateFormat = "[Jan _2 2006]\n"
const timeFormat = "[15:04]"

// ~!chatlog [flags]
// @Alias logall
// Puts messages in a file
// Reply to the first message you want to be logged
// Alternative, run as ~!logall to log all messages in the channel
func chatlog(channel *discordgo.Channel, guild *discordgo.Guild, count int) {
	fName := fmt.Sprintf("jlort-jlort-%d.txt", time.Now().Unix())
	f, err := os.OpenFile(fName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fmt.Printf("Failed to open output file %s: %s", fName, err.Error())
		input.ReadBytes('\n')
		return
	}
	defer f.Close()
	output := bufio.NewWriter(f)
	output.WriteString(time.Now().Format(tsFormat))
	output.WriteString(" by jlortiz's Discord Bot Console")
	if guild != nil {
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
	lastMsg := strconv.FormatUint(uint64(count), 10)
	nicks := make(map[string]string)
	var lastDay int
	for {
		toProc, err := client.ChannelMessages(channel.ID, 100, "", lastMsg, "")
		if err != nil {
			fmt.Println(err)
			input.ReadBytes('\n')
			return
		}
		if len(toProc) == 0 {
			break
		}
		lastMsg = toProc[0].ID
		for i := len(toProc) - 1; i >= 0; i-- {
			v := toProc[i]
			if v.Type != discordgo.MessageTypeDefault && v.Type != discordgo.MessageTypeReply {
				continue
			}
			if v.Content == "" && len(v.Attachments) == 0 && len(v.Embeds) == 0 {
				continue
			}
			if nicks[v.Author.ID] == "" {
				nicks[v.Author.ID] = v.Author.Username
				if guild != nil {
					mem, err := client.State.Member(guild.ID, v.Author.ID)
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
	checkFatal(output.Flush())
	checkFatal(f.Close())
	fmt.Println("Log complete! Look for " + fName)
	input.ReadBytes('\n')
}

// ~!zip
// @Alias archive
// @Hidden
// Zips all attachments and embeds in the channel.
// This command is hidden because the zip file is invariably so big it can't be uploaded.
func archive(channel *discordgo.Channel, _ *discordgo.Guild, count int) {
	type FileInfo struct {
		Filename  string
		URL       string
		Timestamp time.Time
	}
	files := make([]FileInfo, 0, 500)
	lastMsg := ""
	for {
		toProc, err := client.ChannelMessages(channel.ID, 100, lastMsg, "", "")
		if err != nil {
			fmt.Println(err)
			input.ReadBytes('\n')
			return
		}
		if count >= 0 && len(toProc) > count {
			toProc = toProc[:count-1]
		}
		if len(toProc) == 0 {
			break
		}
		count -= len(toProc)
		lastMsg = toProc[len(toProc)-1].ID
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
		}
	}
	fmt.Printf("Found %d messages, zipping...\n", len(files))
	fName := fmt.Sprintf("%s/jlort-jlort-%d.zip", os.TempDir(), time.Now().Unix())
	f, err := os.OpenFile(fName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fmt.Printf("Failed to open tempfile %s: %s", fName, err.Error())
		input.ReadBytes('\n')
		return
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
			fmt.Printf("Failed to append to zip: %s", err.Error())
			input.ReadBytes('\n')
			return
		}
		resp, err := client.Client.Get(fInfo.URL)
		if err != nil {
			fmt.Println(err)
			continue
		}
		_, err = io.Copy(fWriter, resp.Body)
		if err != nil {
			fmt.Printf("Failed to append to zip: %s", err.Error())
			input.ReadBytes('\n')
			return
		}
	}
	checkFatal(zWriter.Close())
	checkFatal(buf.Flush())
	checkFatal(f.Close())
	fmt.Println("Zip complete! Look for " + fName)
	input.ReadBytes('\n')
}
