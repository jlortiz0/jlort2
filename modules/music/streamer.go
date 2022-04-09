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

package music

import (
	"bufio"
	"encoding/json"
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
	if data.Special {
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
	data.Skippers = make(map[string]bool)
	data.Playing = true
	data.StartedAt = time.Now()
	data.Stop = make(chan bool, 1)
	data.Remake = make(chan bool, 1)
	rd := bufio.NewReaderSize(f, 4096)
	header := make([]byte, 4)
	var count byte
Streamer:
	for {
		_, err = io.ReadFull(rd, header)
		if err != nil || header[0] != 'O' || header[1] != 'g' || header[1] != header[2] || header[3] != 'S' {
			break
		}
		_, err = io.CopyN(io.Discard, rd, 22)
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
				for data.Paused {
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
		default:
		}
		time.Sleep(2 * time.Millisecond)
	}
	data.Subprocess.Process.Kill()
	data.Subprocess.Wait()
	data.Playing = false
	if !data.Special {
		streamLock.Lock()
		lastPlayed[vc.GuildID] = time.Now()
		streamLock.Unlock()
	}
}

// ~!mp3 <link to music file>
// @Alias mp4
// @Alias mp3skip
// @Alias mp4skip
// @GuildOnly
// Adds the linked file to the queue
// This supports numerous formats, including mp3, ogg, m4a, s3m, it, spc, vgm, and more.
// Instead of linking a file, you can upload a file with ~!mp3 as the description. Do not delete the message until the stream has finished.
// If invoked as ~!mp3skip, it will skip the current stream and play the linked file immediately if you have permission to do so.
func mp3(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	connect(ctx, nil)
	vc := ctx.Bot.VoiceConnections[ctx.GuildID]
	if vc == nil {
		return nil
	}
	var source string
	if len(ctx.Message.Attachments) != 0 {
		source = ctx.Message.Attachments[0].URL
	} else if len(args) != 0 {
		source = strings.Join(args, " ")
		_, err := url.ParseRequestURI(source)
		if err != nil {
			return ctx.Send("Not a valid URL. Copy it again or upload the file.")
		}
	} else {
		return ctx.Send("~!mp3 <link to music file>\nAlternatively, upload a music file with ~!mp3 as the description.")
	}
	authorName := commands.DisplayName(ctx.Member)
	np := false
	data := new(StreamObj)
	data.Author = ctx.Author.ID
	data.Channel = ctx.ChanID
	data.Source = source
	data.Vol = 65
	ls := streams[ctx.GuildID]
	if strings.HasSuffix(ctx.InvokedWith, "skip") {
		if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
			return ctx.Send("You do not have permission to skip this song.")
		}
		ls.Lock()
		elem := ls.Head()
		if elem != nil {
			obj := elem.Value
			if obj.Playing {
				// <-vc.OpusSend
				obj.Stop <- true
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
	_, err := ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
	return err
}

// ~!play <youtube url or search>
// @Alias playskip
// @GuildOnly
// Adds a Youtube video to the queue
// If a direct link is not provided, the first search result will be taken instead.
// This command also supports direct links to sites other than Youtube. Check https://ytdl-org.github.io/youtube-dl/supportedsites.html for a list.
func play(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!play <YouTube search or supported URL>\nFor a list of supported websites, check https://ytdl-org.github.io/youtube-dl/supportedsites.html")
	}
	connect(ctx, nil)
	vc := ctx.Bot.VoiceConnections[ctx.GuildID]
	if vc == nil {
		return nil
	}
	ctx.Bot.ChannelTyping(ctx.ChanID)
	source := strings.Join(args, " ")
	if strings.Contains(source, "?list=") && strings.Contains(source, "youtu.be") {
		source = source[:strings.IndexByte(source, '?')]
	}
	var entries YDLPlaylist
	var info YDLInfo
	out, err := exec.Command("yt-dlp", "-f", "bestaudio/best", "-J", "--default-search", "ytsearch", source).Output()
	if err != nil {
		err2, ok := err.(*exec.ExitError)
		if ok {
			return fmt.Errorf("Failed to run subprocess: %w\n%s", err2, string(err2.Stderr))
		}
		return fmt.Errorf("Failed to run subprocess: %w", err)
	}
	err = json.Unmarshal(out, &entries)
	if err != nil {
		ctx.Send("Could not get info from this URL.")
		fmt.Println(err)
		return err
	}
	if len(entries.Entries) == 0 {
		err = json.Unmarshal(out, &info)
		if err != nil {
			ctx.Send("Could not get info from this URL.")
			fmt.Println(err)
			return err
		}
	} else {
		info = entries.Entries[0]
	}
	if info.Extractor == "Generic" {
		return ctx.Send("Use ~!mp3 for direct links to files.")
	}
	if info.URL == "" {
		return ctx.Send("Couldn't find page URL.")
	}
	authorName := commands.DisplayName(ctx.Member)
	np := false
	data := new(StreamObj)
	data.Author = ctx.Author.ID
	data.Channel = ctx.ChanID
	data.Info = &info
	data.Source = info.URL
	data.Vol = 65
	ls := streams[ctx.GuildID]
	if strings.HasSuffix(ctx.InvokedWith, "skip") {
		if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
			return ctx.Send("You do not have permission to modify the current song.")
		}
		ls.Lock()
		elem := ls.Head()
		if elem != nil {
			obj := elem.Value
			if obj.Playing {
				// <-vc.OpusSend
				obj.Stop <- true
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
	_, err = ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
	return err
}

// ~!vol [volume]
// @Alias volume
// @GuildOnly
// Check or set stream volume
// Range is 0-200%
// To set the volume, you must have permission to modify the current song.
func vol(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if streams[ctx.GuildID] == nil {
		return ctx.Send("Nothing is playing.")
	}
	elem := streams[ctx.GuildID].Head()
	if elem == nil {
		return ctx.Send("Nothing is playing.")
	}
	strm := elem.Value
	if len(args) == 0 {
		return ctx.Send(fmt.Sprintf("Volume: %d%%", strm.Vol))
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.Send("You do not have permission to modify the current song.")
	}
	if args[0][len(args[0])-1] == '%' {
		args[0] = args[0][:len(args[0])-1]
	}
	vol, err := strconv.Atoi(args[0])
	if err != nil {
		return ctx.Send("Not a number.")
	}
	if vol < 0 {
		vol = 0
	} else if vol > 200 {
		vol = 200
	}
	strm.Vol = vol
	strm.Remake <- true
	return ctx.Send(fmt.Sprintf("Volume set to %d", vol))
}

// ~!seek <position>
// @GuildOnly
// Seeks to a position in the stream
// Position can be in m:ss format or just a number of seconds.
// To seek, you must have permission to modify the current song. To simply view the current position, use ~!np
func seek(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if streams[ctx.GuildID] == nil {
		return ctx.Send("Nothing is playing.")
	}
	elem := streams[ctx.GuildID].Head()
	if elem == nil {
		return ctx.Send("Nothing is playing.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!seek <position (m:ss or ss)>")
	}
	if !hasMusPerms(ctx.Member, ctx.State, ctx.GuildID, 0) {
		return ctx.Send("You do not have permission to modify the current song.")
	}
	stamp := args[0]
	var desired int
	var err error
	ind := strings.IndexByte(stamp, ':')
	if ind == -1 {
		desired, err = strconv.Atoi(stamp)
		if err != nil {
			return ctx.Send("Not a valid timestamp! (mm:ss or ss)")
		}
	} else {
		min, err := strconv.Atoi(stamp[:ind])
		if err != nil {
			return ctx.Send("Not a valid timestamp! (mm:ss or ss)")
		}
		desired, err = strconv.Atoi(stamp[ind+1:])
		if err != nil {
			return ctx.Send("Not a valid timestamp! (mm:ss or ss)")
		}
		desired += min * 60
	}
	strm := elem.Value
	strm.StartedAt = time.Now().Add(time.Duration(-desired) * time.Second)
	strm.Remake <- true
	return ctx.Send(fmt.Sprintf("Skipped to %d:%02d", desired/60, desired%60))
}

// ~!time
// @Alias popcorn
// Displays the time
func popcorn(ctx commands.Context, _ []string) error {
	now := time.Now()
	err := ctx.Send(now.Format("It is 3:04 PM on January _2, 2006."))
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
	ls.PushFront(&StreamObj{Author: ctx.Author.ID, Channel: ctx.ChanID, Source: builder.String(), Special: true})
	go musicStreamer(vc, ls.Head().Value)
	ls.Unlock()
	return nil
}
