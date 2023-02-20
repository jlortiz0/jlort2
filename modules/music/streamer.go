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

package music

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

// musicStreamer is my implementation of... well.. a music streamer.
// It uses a rather long ffmpeg subprocess that pipes to stdout. Stderr is shared with the system.
// On read, the program strips out the headers to get the raw opus data and sends it to the voice connection send channel.
func musicStreamer(vc *discordgo.VoiceConnection, data *StreamObj) {
	if data.Flags&strflag_special != 0 {
		data.Subprocess = exec.Command("ffmpeg", "-i", data.Source, "-map_metadata", "-1", "-acodec", "copy", "-f", "opus", "-loglevel", "warning", "pipe:1")
	} else {
		data.Subprocess = exec.Command("ffmpeg", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5", "-i", data.Source, "-map_metadata", "-1", "-f", "opus", "-ar", "48k", "-ac", "2", "-b:a", "64k", "-compression_level", "8", "-af", fmt.Sprintf("volume=%.2f", float32(data.Vol)/100), "-loglevel", "warning", "pipe:1")
	}
	data.Subprocess.Stderr = os.Stderr
	f, err := data.Subprocess.StdoutPipe()
	if err != nil {
		fmt.Println(err)
		return
	}
	err = data.Subprocess.Start()
	if err != nil {
		fmt.Println(err)
		f.Close()
		return
	}
	data.Skippers = make(map[string]struct{})
	data.Flags |= strflag_playing
	data.StartedAt = time.Now()
	data.Stop = make(chan struct{}, 1)
	data.Remake = make(chan struct{}, 1)
	data.Redirect = make(chan *discordgo.VoiceConnection, 1)
	rd := bufio.NewReaderSize(f, 4096)
	header := make([]byte, 4)
	var count byte
Streamer:
	for {
		_, err = io.ReadFull(rd, header)
		if err != nil || header[0] != 'O' || header[1] != 'g' || header[1] != header[2] || header[3] != 'S' {
			break
		}
		_, err := io.CopyN(io.Discard, rd, 22)
		if err != nil {
			log.Error(err)
			break
		}
		count, err = rd.ReadByte()
		if err != nil {
			break
		}
		segtable := make([]byte, count)
		_, err = io.ReadFull(rd, segtable)
		if err != nil {
			break
		}
		size := 0
		for _, v := range segtable {
			size += int(v)
			if v != 255 {
				b := make([]byte, size)
				_, err = io.ReadFull(rd, b)
				if err != nil {
					break Streamer
				}
				for data.Flags&strflag_paused != 0 {
					time.Sleep(time.Millisecond * 250)
				}
				select {
				case <-data.Stop:
					break Streamer
				case vc.OpusSend <- b:
				}
				size = 0
			}
		}
		select {
		case <-data.Remake:
			data.Subprocess.Process.Kill()
			data.Subprocess.Process.Wait()
			elapsed := time.Since(data.StartedAt).Round(time.Millisecond)
			data.Subprocess = exec.Command("ffmpeg", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5", "-ss", strconv.FormatFloat(elapsed.Seconds(), 'f', 3, 64), "-i", data.Source, "-map_metadata", "-1", "-f", "opus", "-ar", "48k", "-ac", "2", "-b:a", "64k", "-compression_level", "0", "-af", fmt.Sprintf("volume=%.2f", float32(data.Vol)/100), "-loglevel", "warning", "pipe:1")
			f, err = data.Subprocess.StdoutPipe()
			if err != nil {
				fmt.Println(err)
				return
			}
			err = data.Subprocess.Start()
			if err != nil {
				fmt.Println(err)
				return
			}
			rd.Reset(f)
		case <-data.Stop:
			break Streamer
		case v := <-data.Redirect:
			if v != nil {
				vc = v
			}
		default:
		}
		time.Sleep(2 * time.Millisecond)
	}
	data.Subprocess.Process.Kill()
	data.Subprocess.Wait()
	data.Flags &= ^uint16(strflag_playing)
	if data.Flags&strflag_special == 0 {
		streamLock.Lock()
		lastPlayed[vc.GuildID] = time.Now()
		streamLock.Unlock()
	}
	if data.Flags&strflag_dconend != 0 {
		vc.Disconnect()
	}
}

// ~!mp3 <link to audio file>
// @Alias mp4
// @Alias mp3skip
// @Alias mp4skip
// @GuildOnly
// Adds the linked file to the queue
// This supports numerous formats, including mp3, ogg, m4a, s3m, it, spc, vgm, and more.
// Instead of linking a file, you can upload a file with ~!mp3 as the description. Do not delete the message until the stream has finished.
// If invoked as ~!mp3skip, it will skip the current stream and play the linked file immediately if you have permission to do so.
func mp3(ctx commands.Context) error {
	connect(ctx)
	vc := ctx.Bot.VoiceConnections[ctx.GuildID]
	if vc == nil {
		return ctx.RespondPrivate("Network hiccup, please try again.")
	}
	var source string
	cmData := ctx.ApplicationCommandData()
	if cmData.Name == "mp3file" {
		source = cmData.Resolved.Attachments[cmData.Options[0].Value.(string)].URL
	} else {
		source = cmData.Options[0].StringValue()
	}
	_, err := url.ParseRequestURI(source)
	if err != nil {
		return ctx.RespondPrivate("Not a valid URL.")
	}
	authorName := commands.DisplayName(ctx.Member)
	np := false
	data := new(StreamObj)
	data.Author = ctx.User.ID
	data.Channel = ctx.ChannelID
	data.Source = source
	data.Vol = 65
	ls := streams[ctx.GuildID]
	if strings.HasSuffix(ctx.ApplicationCommandData().Name, "skip") {
		if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
			return ctx.RespondPrivate("You do not have permission to skip this stream.")
		}
		ls.Lock()
		elem := ls.Head()
		if elem != nil {
			obj := elem.Value
			if obj.Flags&strflag_noskip != 0 {
				ls.Unlock()
				return ctx.RespondPrivate("This stream cannot be skipped.")
			}
			if obj.Flags&strflag_playing != 0 {
				// <-vc.OpusSend
				obj.Stop <- struct{}{}
			}
			ls.Remove(elem)
		}
		ls.PushFront(data)
		go musicStreamer(vc, data)
		np = true
	} else if ls.Len() == 0 {
		ls.Lock()
		ls.PushFront(data)
		go musicStreamer(vc, data)
		np = true
	} else {
		ls.Lock()
		ls.PushBack(data)
	}
	ls.Unlock()
	embed := buildMusEmbed(data, np, authorName)
	return ctx.RespondEmbed(embed, false)
}

// ~!play <youtube url or search>
// @Alias playskip
// @GuildOnly
// Adds a Youtube video to the queue
// If a direct link is not provided, the first search result will be taken instead.
// This command also supports direct links to sites other than Youtube. Check https://ytdl-org.github.io/youtube-dl/supportedsites.html for a list.
func play(ctx commands.Context) error {
	connect(ctx)
	vc := ctx.Bot.VoiceConnections[ctx.GuildID]
	if vc == nil {
		return ctx.RespondPrivate("Network hiccup, please try again.")
	}
	ctx.RespondDelayed(true)
	source := ctx.ApplicationCommandData().Options[0].StringValue()
	if strings.Contains(source, "?list=") && strings.Contains(source, "youtu.be") {
		source = source[:strings.IndexByte(source, '?')]
	}
	var entries YDLPlaylist
	var info YDLInfo
	out, err := exec.Command("yt-dlp", "-f", "bestaudio/best", "-J", "--default-search", "ytsearch", "--no-playlist", source).Output()
	if err != nil {
		err2, ok := err.(*exec.ExitError)
		if ok {
			return ctx.RespondEdit(fmt.Sprintf("Failed to run subprocess: %s\n%s", err2.Error(), string(err2.Stderr)))
		}
		return fmt.Errorf("failed to run subprocess: %w", err)
	}
	err = json.Unmarshal(out, &entries)
	if err != nil {
		ctx.RespondEdit("Could not get info from this URL.")
		return err
	}
	if len(entries.Entries) == 0 {
		err = json.Unmarshal(out, &info)
		if err != nil {
			ctx.RespondEdit("Could not get info from this URL.")
			return err
		}
	} else {
		info = entries.Entries[0]
	}
	if info.Extractor == "Generic" {
		return ctx.RespondEdit("Use /mp3 for direct links to files.")
	}
	if info.URL == "" {
		return ctx.RespondEdit("Could not get info from this URL.")
	}
	authorName := commands.DisplayName(ctx.Member)
	np := false
	data := new(StreamObj)
	data.Author = ctx.User.ID
	data.Channel = ctx.ChannelID
	data.Info = &info
	data.Source = info.URL
	data.Vol = 65
	ls := streams[ctx.GuildID]
	if ls == nil {
		return ctx.RespondEdit("Discord network error while processing request. Please try again.")
	}
	// TODO: Is there a better way to do this?
	if strings.HasSuffix(ctx.ApplicationCommandData().Name, "skip") {
		if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
			return ctx.RespondEdit("You do not have permission to modify the current stream.")
		}
		ls.Lock()
		elem := ls.Head()
		if elem != nil {
			obj := elem.Value
			if obj.Flags&strflag_noskip != 0 {
				ls.Unlock()
				return ctx.RespondEdit("This stream cannot be skipped.")
			}
			if obj.Flags&strflag_playing != 0 {
				// <-vc.OpusSend
				obj.Stop <- struct{}{}
			}
			ls.Remove(elem)
		}
		ls.PushFront(data)
		go musicStreamer(vc, data)
		np = true
	} else if ls.Len() == 0 {
		ls.Lock()
		ls.PushFront(data)
		go musicStreamer(vc, data)
		np = true
	} else {
		ls.Lock()
		ls.PushBack(data)
	}
	ls.Unlock()
	embed := buildMusEmbed(data, np, authorName)
	// If the RespondDelayed above is changed back to true, uncomment this
	// _, err = ctx.Bot.FollowupMessageCreate(ctx.Interaction, false, &discordgo.WebhookParams{Embeds: []*discordgo.MessageEmbed{embed}})
	// ctx.Bot.InteractionResponseDelete(ctx.Interaction)
	// return err
	return ctx.RespondEditEmbed(embed)
}

// ~!vol [volume]
// @Alias volume
// @GuildOnly
// Check or set stream volume
// Range is 0-200%
// To set the volume, you must have permission to modify the current stream.
func vol(ctx commands.Context) error {
	if streams[ctx.GuildID] == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	elem := streams[ctx.GuildID].Head()
	if elem == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	strm := elem.Value
	args := ctx.ApplicationCommandData().Options
	if len(args) == 0 {
		return ctx.RespondPrivate(fmt.Sprintf("Volume: %d%%", strm.Vol))
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.RespondPrivate("You do not have permission to modify the current song.")
	}
	vol := int(args[0].IntValue())
	if vol < 0 {
		vol = 0
	} else if vol > 200 {
		vol = 200
	}
	strm.Vol = vol
	strm.Remake <- struct{}{}
	return ctx.Respond(fmt.Sprintf("Volume set to %d", vol))
}

// ~!seek <position m:ss or ss>
// @GuildOnly
// Seeks to a position in the stream
// Position can be in m:ss format or just a number of seconds.
// To seek, you must have permission to modify the current stream. To simply view the current position, use ~!np
func seek(ctx commands.Context) error {
	if streams[ctx.GuildID] == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	elem := streams[ctx.GuildID].Head()
	if elem == nil {
		return ctx.RespondPrivate("Nothing is playing.")
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.RespondPrivate("You do not have permission to modify the current stream.")
	}
	stamp := ctx.ApplicationCommandData().Options[0].StringValue()
	var desired int
	var err error
	ind := strings.IndexByte(stamp, ':')
	if ind == -1 {
		desired, err = strconv.Atoi(stamp)
		if err != nil {
			return ctx.RespondPrivate("Not a valid timestamp! (mm:ss or ss)")
		}
	} else {
		min, err := strconv.Atoi(stamp[:ind])
		if err != nil {
			return ctx.RespondPrivate("Not a valid timestamp! (mm:ss or ss)")
		}
		desired, err = strconv.Atoi(stamp[ind+1:])
		if err != nil {
			return ctx.RespondPrivate("Not a valid timestamp! (mm:ss or ss)")
		}
		desired += min * 60
	}
	strm := elem.Value
	strm.StartedAt = time.Now().Add(time.Duration(-desired) * time.Second)
	strm.Remake <- struct{}{}
	return ctx.Respond(fmt.Sprintf("Skipped to %d:%02d", desired/60, desired%60))
}

// ~!time
// @Alias popcorn
// Displays the time
func popcorn(ctx commands.Context) error {
	now := time.Now()
	err := ctx.Respond(now.Format("It is 3:04 PM on January _2, 2006."))
	if err != nil || ctx.GuildID == "" {
		return err
	}

	ls := streams[ctx.GuildID]
	if ls == nil || ls.Len() > 0 {
		return nil
	}
	vc := ctx.Bot.VoiceConnections[ctx.GuildID]
	if vc == nil {
		return nil
	}
	sampleLs := make([]string, 2, 11)
	sampleLs[0] = "itis"
	sampleLs[1] = strconv.Itoa(now.Hour() % 12)
	if sampleLs[1] == "0" {
		sampleLs[1] = "12"
	}
	if now.Minute() == 0 {
	} else if now.Minute() > 20 && now.Minute()%10 > 0 {
		sampleLs = append(sampleLs, strconv.Itoa(now.Minute()/10*10), strconv.Itoa(now.Minute()%10))
	} else {
		if now.Minute() < 10 {
			sampleLs = append(sampleLs, "0")
		}
		sampleLs = append(sampleLs, strconv.Itoa(now.Minute()))
	}
	sampleLs = append(sampleLs, now.Format("PM"), "on", now.Format("Jan"))
	if now.Day() > 20 && now.Day()%10 > 0 {
		sampleLs = append(sampleLs, strconv.Itoa(now.Day()/10*10), strconv.Itoa(now.Day()%10))
	} else {
		sampleLs = append(sampleLs, strconv.Itoa(now.Day()))
	}
	sampleLs = append(sampleLs, strconv.Itoa(now.Year()))
	switch int(now.Month())*128 + now.Day() {
	case 129:
		// New year
	case 1433:
		// Birthday
	case 513:
		// April fools
	}
	builder := new(strings.Builder)
	builder.WriteString("concat:")
	for k, v := range sampleLs {
		builder.WriteString("time/")
		builder.WriteString(v)
		builder.WriteString(".ogg")
		if k < len(sampleLs)-1 {
			builder.WriteByte('|')
		}
	}
	ls.Lock()
	ls.PushFront(&StreamObj{Author: ctx.User.ID, Channel: ctx.ChannelID, Source: builder.String(), Flags: strflag_special | strflag_noskip})
	go musicStreamer(vc, ls.Head().Value)
	ls.Unlock()
	return nil
}

// ~!outro <name>
// @GuildOnly
// Leave the call with style
// Only works if nothing else is playing
// For a list of outros, do ~!outro list
func outro(ctx commands.Context) error {
	name := ctx.ApplicationCommandData().Options[0].StringValue()
	if name == "list" {
		f, err := os.Open("outro")
		if err != nil {
			return err
		}
		names, err := f.Readdirnames(0)
		if err != nil {
			return err
		}
		builder := new(strings.Builder)
		builder.WriteString("Outros:")
		for _, x := range names {
			builder.WriteByte('\n')
			ind := strings.LastIndexByte(x, '.')
			if ind == -1 {
				ind = len(x)
			}
			builder.WriteString(x[:ind])
		}
		return ctx.RespondPrivate(builder.String())
	}
	ls := streams[ctx.GuildID]
	vc := ctx.Bot.VoiceConnections[ctx.GuildID]
	if vc == nil || ls == nil {
		return ctx.RespondPrivate("Not connected to voice.")
	}
	if ls.Len() > 0 {
		return ctx.RespondPrivate("Can't play an outro while something else is playing.")
	}
	_, err := os.Stat("outro" + string(os.PathSeparator) + name + ".ogg")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ctx.RespondPrivate("That outro does not exist.")
		}
		return err
	}
	ls.Lock()
	ls.PushFront(&StreamObj{Author: ctx.User.ID, Channel: ctx.ChannelID, Source: "outro" + string(os.PathSeparator) + name + ".ogg", Flags: strflag_dconend | strflag_noskip | strflag_special})
	go musicStreamer(vc, ls.Head().Value)
	ls.Unlock()
	return ctx.RespondEmpty()
}

func outroAutocomplete(ctx commands.Context) []*discordgo.ApplicationCommandOptionChoice {
	pre := ctx.ApplicationCommandData().Options[0].StringValue()
	fList, err := os.ReadDir("outro")
	if err != nil {
		return nil
	}
	output := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(fList))
	for _, x := range fList {
		n := x.Name()
		if !strings.HasSuffix(n, ".ogg") {
			continue
		}
		n = n[:len(n)-4]
		if strings.HasPrefix(n, pre) {
			output = append(output, &discordgo.ApplicationCommandOptionChoice{Name: n, Value: n})
		}
	}
	return output
}
