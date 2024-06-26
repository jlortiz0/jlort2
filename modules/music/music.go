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
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

// YDLInfo holds some of the fields returned by youtube-dl.
// Only those needed for my streamer to work are represented.
type YDLInfo struct {
	Title     string
	Thumbnail string
	Extractor string `json:"extractor_key"`
	Webpage   string `json:"webpage_url"`
	URL       string
	Duration  float32
}

// YDLPlaylist is a list of YDLInfo, used when youtube-dl returns a playlist instead of a single video.
// Usually, only the first item of the playlist is counted. The only reason this isn't a slice
// is for compatibility with json.Unmarshal
type YDLPlaylist struct {
	Entries []YDLInfo
}

type streamFlag uint8

const (
	strflag_special streamFlag = 1 << iota // Should this stream be treated as special? If true, lastPlayed will not be set, and the source will not be filtered.
	strflag_loop                           // If true, stream will loop after end
	strflag_playing                        // If true, there is currently a thread running this stream
	strflag_paused                         // If true, the thread running this stream should sleep
	strflag_noskip                         // If true, this stream should not be skippable once playing
	strflag_dconend                        // If true, the bot should disconnect once this stream ends
)

// StreamObj stores the data needed for an active stream. A partial version of this is used for a queued stream.
type StreamObj struct {
	StartedAt  time.Time                       // The time the streamer started
	Info       *YDLInfo                        // The YDLInfo associated with this stream. If nil, this is a direct file stream
	Remake     chan struct{}                   // When this channel is written to, the ffmpeg process will be recreated with new parameters
	Skippers   map[string]struct{}             // A set of the IDs of users who have voted to skip this stream
	Subprocess *exec.Cmd                       // The ffmpeg subprocess that the streamer streams from
	Stop       chan struct{}                   // When this channel is written to, the streamer will stop
	Redirect   chan *discordgo.VoiceConnection // Uses this new VoiceConnection instead of the original one passsed in
	Author     string                          // The ID of the user who queued the stream
	Channel    string                          // The channel in which the stream was queued, used for next up announcements
	Source     string                          // The URL to stream from
	InterID    string                          // Interaction ID
	Vol        int                             // The volume, 0-200. This will be copied from the previous stream if possible
	PauseTs    time.Duration                   // How long the stream was playing when we paused it
	Flags      streamFlag                      // See above constants
}

var streams map[string]*lockQueue
var lastPlayed map[string]time.Time
var streamLock *sync.RWMutex = new(sync.RWMutex)
var queryDj *sql.Stmt

const dcTimeout time.Duration = time.Minute * -10
const eggTimeout time.Duration = time.Minute * -8
const popRefreshRate = 3 * time.Second

// ~!connect
// @GuildOnly
// Connects to voice
// If you are in a voice channel and I am in a different voice channel, you will be asked to move.
// This function is automatically called if you queue something and I am not connected.
func connect(ctx *commands.Context) error {
	authorVoice, err := ctx.State.VoiceState(ctx.GuildID, ctx.User.ID)
	if err != nil || authorVoice.ChannelID == "" {
		return ctx.RespondPrivate("You must be in a voice channel to use this command.")
	}
	vc, ok := ctx.Bot.VoiceConnections[ctx.GuildID]
	if ok {
		streamLock.Lock()
		if lastPlayed[ctx.GuildID].IsZero() {
			if streams[ctx.GuildID] != nil && streams[ctx.GuildID].Len() > 0 {
				streams[ctx.GuildID].Head().Value.Stop <- struct{}{}
			}
			lastPlayed[ctx.GuildID] = time.Now()
			streams[ctx.GuildID] = new(lockQueue)
		}
		streamLock.Unlock()
		if vc.ChannelID != authorVoice.ChannelID {
			channel, err := ctx.State.Channel(vc.ChannelID)
			if err != nil {
				return fmt.Errorf("failed to get channel info: %w", err)
			}
			return ctx.RespondPrivate("Please move to voice channel " + channel.Name)
		}
	} else {
		perms, err := ctx.State.UserChannelPermissions(ctx.Me.ID, authorVoice.ChannelID)
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionVoiceConnect == 0 {
			return ctx.RespondPrivate("I need the Connect permission to use this command.")
		}
		streamLock.Lock()
		if streams[ctx.GuildID] != nil && streams[ctx.GuildID].Len() > 0 {
			streams[ctx.GuildID].Head().Value.Stop <- struct{}{}
		}
		lastPlayed[ctx.GuildID] = time.Now()
		streams[ctx.GuildID] = new(lockQueue)
		streamLock.Unlock()
		_, err = ctx.Bot.ChannelVoiceJoin(ctx.GuildID, authorVoice.ChannelID, false, true)
		if err != nil {
			return fmt.Errorf("failed to connect to voice: %w", err)
		}
	}
	if ctx.ApplicationCommandData().Name == "connect" {
		return ctx.RespondEmpty()
	}
	return nil
}

// ~!dc
// @Alias disconnect
// @GuildOnly
// Disconnects from voice
// If streams are currently playing, paused, or queued, I will not disconnect unless you have permissions to clear the queue.
// I will automatically disconnect after 5 minutes of inactivity or if there is nobody else in the call.
// Note that I may consider bots as "other people" if they are not server deafened. For best results, you should server deafen other bots.
func dc(ctx *commands.Context) error {
	vc, ok := ctx.Bot.VoiceConnections[ctx.GuildID]
	if ok {
		streamLock.RLock()
		ls, ok := streams[ctx.GuildID]
		streamLock.RUnlock()
		if ok && ls.Len() > 1 {
			dr := ctx.Data.(discordgo.ApplicationCommandInteractionData)
			dr.Options = []*discordgo.ApplicationCommandInteractionDataOption{{Type: discordgo.ApplicationCommandOptionInteger, Value: -5738}}
			err := remove(ctx)
			if ls.Len() > 1 {
				return err
			}
		}
		if ok && ls.Head() != nil && ls.Head().Value.Flags&strflag_noskip != 0 {
			if ls.Head().Value.Flags&strflag_dconend != 0 {
				return ctx.RespondPrivate("Won't you at least wait for the outro?")
			}
			return ctx.RespondPrivate("This stream cannot be ended.")
		}
		err := vc.Disconnect()
		if err != nil {
			return fmt.Errorf("failed to disconnect from voice: %w", err)
		}
	}
	return ctx.RespondEmpty()
}

// ~!dj <@role>
// @GuildOnly
// @ManageServer
// See or change the DJ role
// People with the DJ role can remove or skip any stream, regardless of who queued it.
// Only people with the Manage Server permission can change the DJ role.
// To disable the DJ role, set to @everyone
func dj(ctx *commands.Context) error {
	role := ctx.ApplicationCommandData().Options[0].RoleValue(ctx.Bot, ctx.GuildID)
	if role.Managed {
		return ctx.RespondPrivate("Role must be user-created.")
	}
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	if role.Name == "@everyone" {
		ctx.Database.Exec("DELETE FROM djRole WHERE gid=?001;", gid)
		return ctx.RespondPrivate("DJ role disabled.")
	}
	ctx.Database.Exec("INSERT OR REPLACE INTO djRole (gid, rid) VALUES (?001, ?002);", gid, role.ID)
	return ctx.RespondPrivate("DJ role set to " + role.Name)
}

// onDc is called when a user, including the bot, disconnects from a voice channel.
// This function determines if the bot should also disconnect depending on how many users are left in the channel.
// If it was the bot that triggered this event, the bot cleans up the stream queue and additonal structures.
func onDc(self *discordgo.Session, event *discordgo.VoiceStateUpdate) {
	if self.State.User.ID == event.UserID && event.ChannelID == "" {
		lastPlayed[event.GuildID] = time.Time{}
		time.AfterFunc(5*time.Second, func() {
			streamLock.RLock()
			if streams[event.GuildID] != nil && !lastPlayed[event.GuildID].IsZero() {
				streams[event.GuildID].Lock()
				elem := streams[event.GuildID].Head()
				if elem != nil {
					elem.Value.Stop <- struct{}{}
				}
				streams[event.GuildID].Clear()
				streams[event.GuildID].Unlock()
				streamLock.RUnlock()
				streamLock.Lock()
				delete(streams, event.GuildID)
				delete(lastPlayed, event.GuildID)
				streamLock.Unlock()
			} else {
				streamLock.RUnlock()
			}
		})
		return
	}
	vc := self.VoiceConnections[event.GuildID]
	if vc != nil {
		vguild, err := self.State.Guild(event.GuildID)
		if err != nil {
			return
		}
		count := 0
		for _, v := range vguild.VoiceStates {
			if v.ChannelID == vc.ChannelID && !v.Deaf && !v.SelfDeaf && v.UserID != self.State.User.ID {
				count++
			}
		}
		if count == 0 {
			vc.Disconnect()
		}
	}
}

// popLock is a simple session.lock like byte to prevent multiple musicPoppers running at the same time
// This can occur if RESUMED is not handled properly
// I would use a Once, but I think that an invalid session could cause issues
var popLock byte

// musicPopper checks all guilds registered in the streams map every 3 seconds.
// If the voice client in that guild has been inactive too long, it is disconnected. onDc takes care of the cleanup for that.
// If there is a stream in the now playing slot and it has finished, it is popped and the next one is started if it exists.
// Now you might ask... why not use the functions of the time.Time package to implement the first part and have the second part called by the streamer on exit?
// I wrote this early in jlort2's development and I'm too lazy to change it. The original intention was to handle an abnormal streamer exit, but that's been mostly fixed now.
func musicPopper(self *discordgo.Session, myLock byte) {
	for popLock == myLock {
		cutoff := time.Now().Add(dcTimeout)
		eggCutoff := time.Now().Add(eggTimeout)
		byeCutoff := time.Now().Add(dcTimeout + popRefreshRate)
		streamLock.RLock()
		for k, v := range streams {
			if v.Len() == 0 {
				lp := lastPlayed[k]
				if lp.Before(cutoff) {
					vc := self.VoiceConnections[k]
					if vc != nil {
						vc.Disconnect()
					}
				} else if lp.Before(eggCutoff) && eggCutoff.Sub(lp) < popRefreshRate {
					if rand.Intn(8) != 0 {
						continue
					}
					f, err := os.Open("spook/")
					if err != nil {
						continue
					}
					entries, err := f.ReadDir(0)
					f.Close()
					if err != nil {
						continue
					}
					source := "spook/" + entries[rand.Intn(len(entries))].Name()
					vc := self.VoiceConnections[k]
					if vc == nil {
						continue
					}
					log.Info(fmt.Sprintf("Playing %s for %s", source, k))
					obj := &StreamObj{Flags: strflag_special | strflag_noskip, Source: source, Author: "@me"}
					v.Lock()
					v.PushFront(obj)
					v.Unlock()
					go musicStreamer(vc, obj)
				} else if lp.Before(byeCutoff) {
					vc := self.VoiceConnections[k]
					if vc == nil {
						continue
					}
					obj := &StreamObj{Flags: strflag_special | strflag_noskip | strflag_dconend, Source: "modules/music/bye.ogg", Author: "@me"}
					v.Lock()
					v.PushFront(obj)
					v.Unlock()
					go musicStreamer(vc, obj)
				}
				continue
			}
			v.RLock()
			obj := v.Head().Value
			v.RUnlock()
			if obj.Flags&strflag_playing == 0 {
				if obj.Flags&strflag_loop == 0 {
					vol := obj.Vol
					v.Lock()
					v.Remove(v.Head())
					v.Unlock()
					if v.Len() == 0 {
						continue
					}
					obj = v.Head().Value
					obj.Vol = vol
				}
				vc := self.VoiceConnections[k]
				if vc == nil {
					v.Lock()
					v.Clear()
					v.Unlock()
					continue
				}
				mem, err := self.State.Member(k, obj.Author)
				if err != nil {
					go musicStreamer(vc, obj)
					continue
				}
				authorName := commands.DisplayName(mem)
				embed := buildMusEmbed(obj, true, authorName)
				if obj.Flags&strflag_loop != 0 {
					embed.Title = "Looping"
				}
				go musicStreamer(vc, obj)
				component := discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{Emoji: discordgo.ComponentEmoji{Name: "\u23ED"}, Style: discordgo.SecondaryButton, CustomID: "skip\a"}}}
				self.ChannelMessageSendComplex(obj.Channel, &discordgo.MessageSend{
					Embeds:     []*discordgo.MessageEmbed{embed},
					Components: []discordgo.MessageComponent{component},
				})
			}
		}
		streamLock.RUnlock()
		time.Sleep(popRefreshRate)
	}
}

// buildMusEmbed builds an embed for announcing queued or next up streams.
// If np is true, a Now Playing embed is generated. Otherwise, a Queued embed is generated.
func buildMusEmbed(data *StreamObj, np bool, authorName string) *discordgo.MessageEmbed {
	output := new(discordgo.MessageEmbed)
	if np {
		output.Description = "Now Playing"
	} else {
		output.Description = "Queued"
	}
	output.Color = 0x992d22
	output.Fields = []*discordgo.MessageEmbedField{{Name: "Submitter:", Value: authorName}}
	if data.Info != nil {
		info := data.Info
		output.Title = info.Title
		output.URL = info.Webpage
		if info.Thumbnail != "" {
			output.Image = &discordgo.MessageEmbedImage{URL: info.Thumbnail}
		}
		if info.Duration > 0 {
			output.Fields = append(output.Fields, &discordgo.MessageEmbedField{Name: "Length:",
				Value: fmt.Sprintf("%01d:%02d", int(info.Duration)/60, int(info.Duration)%60)})
		}
	} else {
		output.Title = "Uploaded File"
		output.URL = data.Source
	}
	if runtime.GOOS != "windows" {
		f, err := os.Open("/run/systemd/shutdown/scheduled")
		if err == nil {
			var nsec int64
			fmt.Fscanf(f, "USEC=%d\n", &nsec)
			nsec *= 1000
			until := time.Until(time.Unix(0, nsec))
			if until > 0 && until < time.Minute*45 {
				footer := new(discordgo.MessageEmbedFooter)
				footer.Text = fmt.Sprintf("Notice: jlort jlort will shut down in %d minutes", until/time.Minute)
				output.Footer = footer
			}
			f.Close()
		}
	}
	return output
}

// hasMusPerms determines if a user has permissions over a stream object.
// First, it checks if the stream is empty. Then, it checks if the user submitted the stream.
// Lastly, it checks if the user has the ManageServer permission or the DJ role.
func hasMusPerms(user *discordgo.Member, state *discordgo.State, guild string, index int) bool {
	ls := streams[guild]
	if ls == nil || ls.Len() == 0 {
		return true
	}
	log.Debug("hasMusPerms: list is not nil")
	if user == nil {
		log.Errors("hasMusPerms: member is nil")
		return false
	}
	ls.RLock()
	var elem *queueObj
	if ls.Len()-1 == index {
		elem = ls.Tail()
	} else {
		elem = ls.Head()
		for i := 0; i < index; i++ {
			elem = elem.Next()
			if elem == nil {
				return true
			}
		}
	}
	current := elem.Value
	ls.RUnlock()
	log.Debug("hasMusPerms: iterated list")
	if user.User.ID == current.Author {
		return true
	}
	log.Debug("hasMusPerms: checked ID")
	perms, err := state.UserChannelPermissions(user.User.ID, current.Channel)
	if err != nil && perms&discordgo.PermissionManageServer != 0 {
		return true
	}
	log.Debug("hasMusPerms: checked perms")
	gid, _ := strconv.ParseUint(guild, 10, 64)
	result := queryDj.QueryRow(gid)
	var DJ string
	result.Scan(&DJ)
	if DJ != "" {
		for _, v := range user.Roles {
			if v == DJ {
				return true
			}
		}
	}
	log.Debug("hasMusPerms: checked roles")
	return false
}

func handleReconnect(self *discordgo.Session, _ *discordgo.Resumed) {
	time.Sleep(time.Millisecond * 50)
	cutoff := time.Now().Add(dcTimeout)
	streamLock.Lock()
	for k, v := range self.VoiceConnections {
		if lastPlayed[k].Before(cutoff) {
			v.Disconnect()
		} else if streams[k] == nil {
			streams[k] = new(lockQueue)
			lastPlayed[k] = time.Now()
			log.Warn("Reconnected to " + k)
		} else if streams[k].Len() > 0 {
			streams[k].Head().Value.Redirect <- v
			lastPlayed[k] = time.Now()
			log.Warn("Redirected on " + k)
		}
	}
	streamLock.Unlock()
}

func delGuildSongs(_ *discordgo.Session, event *discordgo.GuildDelete) {
	if !event.Unavailable {
		gid, _ := strconv.ParseUint(event.ID, 10, 64)
		commands.GetDatabase().Exec("DELETE FROM djRole WHERE gid = ?001;", gid)
	}
	streamLock.Lock()
	v := streams[event.ID]
	if v != nil && v.Len() != 0 {
		obj := v.Head().Value
		obj.Stop <- struct{}{}
		v.Clear()
	}
	delete(streams, event.ID)
	delete(lastPlayed, event.ID)
	streamLock.Unlock()
}

func dfpwm(ctx *commands.Context) error {
	var source, title string
	cmData := ctx.ApplicationCommandData()
	ctx.RespondDelayed(false)
	if cmData.Name == "dfpwmfile" {
		source = cmData.Resolved.Attachments[cmData.Options[0].Value.(string)].URL
		title = path.Base(source)
		ind := strings.LastIndexByte(title, '.')
		if ind != -1 {
			title = title[:ind]
		}
	} else {
		source = cmData.Options[0].StringValue()
		if strings.Contains(source, "?list=") && strings.Contains(source, "youtu.be") {
			source, _, _ = strings.Cut(source, "?list=")
		}
		if strings.Contains(source, "&list=") && strings.Contains(source, "youtube.com") {
			source, _, _ = strings.Cut(source, "&list=")
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
			title = path.Base(source)
			ind := strings.LastIndexByte(title, '.')
			if ind != -1 {
				title = title[:ind]
			}
		} else {
			source = info.URL
			title = info.Title
		}
		if info.URL == "" {
			return ctx.RespondEdit("Could not get info from this URL.")
		}
	}
	out, err := exec.Command("ffmpeg", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5", "-i", source, "-map_metadata", "-1", "-f", "dfpwm", "-ar", "48k", "-ac", "1", "-loglevel", "warning", "pipe:1").Output()
	if err != nil {
		return fmt.Errorf("failed to run subprocess: %w", err)
	}
	resp := new(discordgo.WebhookEdit)
	resp.Files = []*discordgo.File{{Name: title + ".dfpwm", Reader: bytes.NewReader(out)}}
	_, err = ctx.Bot.InteractionResponseEdit(ctx.Interaction, resp)
	return err
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// In addition to commands, this function creates two GuildID -> data maps, loads the song aliases from disk, and starts the stream popper thread.
func Init(self *discordgo.Session) {
	streams = make(map[string]*lockQueue)
	lastPlayed = make(map[string]time.Time)
	self.AddHandler(onDc)
	self.AddHandler(delGuildSongs)
	self.AddHandler(handleReconnect)
	commands.PrepareCommand("connect", "Connect to voice").Guild().Register(connect, nil)
	commands.PrepareCommand("dc", "Disconnect from voice").Guild().Register(dc, nil)
	commands.PrepareCommand("dj", "Set DJ role").Guild().Perms(discordgo.PermissionManageServer).Register(dj, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("role", "DJ role to set, @everyone to disable").AsRole().Required().Finalize(),
	})
	optionLink := commands.NewCommandOption("url", "Link to audio file").AsString().Required().Finalize()
	commands.PrepareCommand("mp3", "Play file from a link").Guild().Component(playComponent).Register(mp3, []*discordgo.ApplicationCommandOption{optionLink})
	commands.PrepareCommand("mp3skip", "Skip to file from a link").Guild().Component(playComponent).Register(mp3, []*discordgo.ApplicationCommandOption{optionLink})
	commands.PrepareCommand("mp3file", "Play an uploaded file").Guild().Component(playComponent).Register(mp3, []*discordgo.ApplicationCommandOption{
		{Name: "file", Description: "File to play", Type: discordgo.ApplicationCommandOptionAttachment, Required: true},
	})
	commands.PrepareCommand("loop", "Set stream loop").Guild().Register(loop, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("enabled", "Loop this song?").AsBool().Required().Finalize(),
	})
	commands.PrepareCommand("skip", "Skip current song").Guild().Component(skip).Register(skip, nil)
	optionVideo := commands.NewCommandOption("url", "Link to YouTube video, or anything supported by yt-dlp").AsString().Required().Finalize()
	commands.PrepareCommand("play", "Play YouTube video").Guild().Component(playComponent).Register(play, []*discordgo.ApplicationCommandOption{optionVideo})
	commands.PrepareCommand("playskip", "Skip to YouTube video").Guild().Component(playComponent).Register(play, []*discordgo.ApplicationCommandOption{optionVideo})
	commands.PrepareCommand("pause", "Pause or unpause current stream").Guild().Component(pause).Register(pause, nil)
	commands.PrepareCommand("remove", "Remove stream from queue").Guild().Register(remove, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("index", "Index to remove, -1 for all").AsInt().SetMinMax(-1, music_queue_max*2).Required().Finalize(),
	})
	commands.PrepareCommand("np", "Details of current stream").Guild().Register(np, nil)
	commands.PrepareCommand("queue", "See what's in the queue").Guild().Register(queue, nil)
	commands.PrepareCommand("vol", "Check or change volume").Guild().Register(vol, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("volume", "Volume in percent").AsInt().SetMinMax(0, 200).Finalize(),
	})
	commands.PrepareCommand("seek", "Seek to a position in the stream").Guild().Register(seek, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("pos", "Position in mm:ss").AsString().Required().Finalize(),
	})
	commands.PrepareCommand("dfpwm", "Convert to DFPWM").Register(dfpwm, []*discordgo.ApplicationCommandOption{optionVideo})
	commands.PrepareCommand("dfpwmfile", "Convert to DFPWM").Register(dfpwm, []*discordgo.ApplicationCommandOption{
		{Name: "file", Description: "File to convert", Type: discordgo.ApplicationCommandOptionAttachment, Required: true},
	})
	fList, err := os.ReadDir("outro")
	if err != nil {
		log.Error(err)
	} else {
		outroComp := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(fList))
		for _, x := range fList {
			n := x.Name()
			if !strings.HasSuffix(n, ".ogg") {
				continue
			}
			n = n[:len(n)-4]
			outroComp = append(outroComp, &discordgo.ApplicationCommandOptionChoice{Name: n, Value: n})
		}
		commands.PrepareCommand("outro", "Play an outro").Guild().Register(outro, []*discordgo.ApplicationCommandOption{
			commands.NewCommandOption("name", "Name of outro to play").AsString().Required().Choice(outroComp).Finalize(),
		})
	}
	popLock++
	go musicPopper(self, popLock)
	queryDj, err = commands.GetDatabase().Prepare("SELECT rid FROM djRole WHERE gid=?001;")
	if err != nil {
		log.Error(err)
		return
	}
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the song alias list, unregisters the disconnect handler, clears all queues, and disconnects all voice clients.
func Cleanup(self *discordgo.Session) {
	popLock++
	streamLock.Lock()
	for _, v := range streams {
		if v.Len() != 0 {
			obj := v.Head().Value
			obj.Stop <- struct{}{}
		}
	}
	streamLock.Unlock()
	for _, v := range self.VoiceConnections {
		v.Disconnect()
	}
	queryDj.Close()
}
