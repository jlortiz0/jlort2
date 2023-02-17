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
	"fmt"
	"math/rand"
	"os"
	"os/exec"
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
	Duration  float32
	Title     string
	Thumbnail string
	Extractor string `json:"extractor_key"`
	Webpage   string `json:"webpage_url"`
	URL       string
}

// YDLPlaylist is a list of YDLInfo, used when youtube-dl returns a playlist instead of a single video.
// Usually, only the first item of the playlist is counted. The only reason this isn't a slice
// is for compatibility with json.Unmarshal
type YDLPlaylist struct {
	Entries []YDLInfo
}

const (
	strflag_special = 1 << iota // Should this stream be treated as special? If true, lastPlayed will not be set, and the source will not be filtered.
	strflag_loop                // If true, stream will loop after end
	strflag_playing             // If true, there is currently a thread running this stream
	strflag_paused              // If true, the thread running this stream should sleep
	strflag_noskip              // If true, this stream should not be skippable once playing
	strflag_dconend             // If true, the bot should disconnect once this stream ends
)

// StreamObj stores the data needed for an active stream. A partial version of this is used for a queued stream.
type StreamObj struct {
	Author  string   // The ID of the user who queued the stream
	Channel string   // The channel in which the stream was queued, used for next up announcements
	Source  string   // The URL to stream from
	Vol     int      // The volume, 0-200. This will be copied from the previous stream if possible
	Flags   uint16   // See above constants
	Info    *YDLInfo // The YDLInfo associated with this stream. If nil, this is a direct file stream
	// Fields below this line may not be populated or valid until the streamer starts
	Remake     chan struct{}                   // When this channel is written to, the ffmpeg process will be recreated with new parameters
	Skippers   map[string]struct{}             // A set of the IDs of users who have voted to skip this stream
	StartedAt  time.Time                       // The time the streamer started
	Subprocess *exec.Cmd                       // The ffmpeg subprocess that the streamer streams from
	Stop       chan struct{}                   // When this channel is written to, the streamer will stop
	Redirect   chan *discordgo.VoiceConnection // Uses this new VoiceConnection instead of the original one passsed in
}

var streams map[string]*lockQueue
var lastPlayed map[string]time.Time
var djRoles map[string]string
var streamLock *sync.RWMutex = new(sync.RWMutex)
var djLock *sync.RWMutex = new(sync.RWMutex)
var dirty bool

const dcTimeout time.Duration = time.Minute * -10
const eggTimeout time.Duration = time.Minute * -8
const popRefreshRate = 3 * time.Second

// ~!connect
// @GuildOnly
// Connects to voice
// If you are in a voice channel and jlort jlort is in a different voice channel, you will be asked to move.
// This function is automatically called if you queue something and jlort is not connected.
func connect(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	authorVoice, err := ctx.State.VoiceState(ctx.GuildID, ctx.Author.ID)
	if err != nil || authorVoice.ChannelID == "" {
		return ctx.Send("You must be in a voice channel to use this command.")
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
			return ctx.Send("Please move to voice channel " + channel.Name)
		}
	} else {
		perms, err := ctx.State.UserChannelPermissions(ctx.Me.ID, authorVoice.ChannelID)
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionVoiceConnect == 0 {
			return ctx.Send("I need the Connect permission to use this command.")
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
	return nil
}

// ~!dc
// @Alias disconnect
// @GuildOnly
// Disconnects from voice
// If streams are currently playing, paused, or queued, jlort jlort will not disconnect unless you have permissions to clear the queue.
// jlort jlort will automatically disconnect after 5 minutes of inactivity or if there is nobody to listen to it.
// Note that jlort jlort may consider bots valid listeners if they are not server deafened. For best results, you should server deafen other bots.
func dc(ctx commands.Context, _ []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	vc, ok := ctx.Bot.VoiceConnections[ctx.GuildID]
	if ok {
		streamLock.RLock()
		ls, ok := streams[ctx.GuildID]
		streamLock.RUnlock()
		if ok && ls.Len() > 1 {
			err := remove(ctx, []string{"all-q"})
			if ls.Len() > 1 {
				return err
			}
		}
		if ok && ls.Head() != nil && ls.Head().Value.Flags&strflag_noskip != 0 {
			if ls.Head().Value.Flags&strflag_dconend != 0 {
				return ctx.Send("Won't you at least wait for the outro?")
			}
			return ctx.Send("This stream cannot be ended.")
		}
		err := vc.Disconnect()
		if err != nil {
			return fmt.Errorf("failed to disconnect from voice: %w", err)
		}
	}
	return nil
}

// ~!dj [@role]
// @GuildOnly
// @ManageServer
// See or change the DJ role
// People with the DJ role can remove or skip any stream, regardless of who queued it.
// Only people with the Manage Server permission can change the DJ role.
// You must mention the DJ role to set it because I am lazy.
// To disable the DJ role, set to "none" without quotes or @
func dj(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) == 0 {
		djLock.RLock()
		if djRoles[ctx.GuildID] == "" {
			djLock.RUnlock()
			return ctx.Send("No DJ role set.")
		}
		role, err := ctx.State.Role(ctx.GuildID, djRoles[ctx.GuildID])
		djLock.RUnlock()
		if err != nil {
			djLock.Lock()
			delete(djRoles, ctx.GuildID)
			djLock.Unlock()
			return ctx.Send("No DJ role set.")
		}
		return ctx.Send("DJ role is " + role.Name)
	}
	perms, err := ctx.State.UserChannelPermissions(ctx.Author.ID, ctx.ChanID)
	if err != nil {
		return err
	}
	if perms&discordgo.PermissionManageServer == 0 {
		return ctx.Send("You need Manage Server to change the DJ role.")
	}
	if strings.ToLower(args[0]) == "none" {
		djLock.Lock()
		delete(djRoles, ctx.GuildID)
		dirty = true
		djLock.Unlock()
		return ctx.Send("DJ role disabled.")
	}
	if !strings.HasPrefix(args[0], "<@&") || args[0][len(args[0])-1] != '>' {
		return ctx.Send("Not a valid role mention.")
	}
	roleID := args[0][3 : len(args[0])-1]
	djLock.Lock()
	djRoles[ctx.GuildID] = roleID
	dirty = true
	role, err := ctx.State.Role(ctx.GuildID, djRoles[ctx.GuildID])
	djLock.Unlock()
	if err != nil {
		return err
	}
	return ctx.Send("DJ role set to " + role.Name)
}

// onDc is called when a user, including the bot, disconnects from a voice channel.
// This function determines if the bot should also disconnect depending on how many users are left in the channel.
// If it was the bot that triggered this event, the bot cleans up the stream queue and additonal structures.
func onDc(self *discordgo.Session, event *discordgo.VoiceStateUpdate) {
	vc := self.VoiceConnections[event.GuildID]
	if vc != nil {
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
					streams[event.GuildID].Unlock()
					streamLock.RUnlock()
					streamLock.Lock()
					delete(streams, event.GuildID)
					streamLock.Unlock()
				} else {
					streamLock.RUnlock()
				}
			})
			return
		}
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
					if rand.Intn(4) != 0 {
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
					continue
				}
				mem, err := self.State.Member(k, obj.Author)
				if err != nil {
					continue
				}
				authorName := commands.DisplayName(mem)
				embed := buildMusEmbed(obj, true, authorName)
				if obj.Flags&strflag_loop != 0 {
					embed.Title = "Looping"
				}
				go musicStreamer(vc, obj)
				self.ChannelMessageSendEmbed(obj.Channel, embed)
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
	djLock.RLock()
	DJ := djRoles[guild]
	djLock.RUnlock()
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
	for k, v := range self.VoiceConnections {
		if time.Until(lastPlayed[k]) < dcTimeout {
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
}

func delGuildSongs(_ *discordgo.Session, event *discordgo.GuildDelete) {
	djLock.Lock()
	delete(djRoles, event.ID)
	djLock.Unlock()
	dirty = true
	v := streams[event.ID]
	if v != nil && v.Len() != 0 {
		obj := v.Head().Value
		obj.Stop <- struct{}{}
	}
	delete(streams, event.ID)
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// In addition to commands, this function creates two GuildID -> data maps, loads the song aliases from disk, and starts the stream popper thread.
func Init(self *discordgo.Session) {
	streams = make(map[string]*lockQueue)
	lastPlayed = make(map[string]time.Time)
	err := commands.LoadPersistent("dj", &djRoles)
	if err != nil {
		log.Error(err)
		return
	}
	self.AddHandler(onDc)
	self.AddHandler(delGuildSongs)
	self.AddHandler(handleReconnect)
	commands.RegisterCommand(connect, "connect")
	commands.RegisterCommand(dc, "dc")
	commands.RegisterCommand(dj, "dj")
	commands.RegisterCommand(mp3, "mp3")
	commands.RegisterCommand(mp3, "mp3skip")
	commands.RegisterCommand(mp3, "mp4")
	commands.RegisterCommand(mp3, "mp4skip")
	commands.RegisterCommand(loop, "loop")
	commands.RegisterCommand(skip, "skip")
	commands.RegisterCommand(play, "play")
	commands.RegisterCommand(play, "playskip")
	commands.RegisterCommand(pause, "pause")
	commands.RegisterCommand(pause, "unpause")
	commands.RegisterCommand(remove, "remove")
	commands.RegisterCommand(remove, "rm")
	commands.RegisterCommand(np, "np")
	commands.RegisterCommand(np, "playing")
	commands.RegisterCommand(queue, "queue")
	commands.RegisterCommand(vol, "vol")
	commands.RegisterCommand(seek, "seek")
	commands.RegisterCommand(popcorn, "popcorn")
	commands.RegisterCommand(popcorn, "time")
	commands.RegisterCommand(locket, "_locket")
	commands.RegisterCommand(outro, "outro")
	commands.RegisterSaver(saveData)
	popLock++
	go musicPopper(self, popLock)
}

func saveData() error {
	if !dirty {
		return nil
	}
	djLock.RLock()
	err := commands.SavePersistent("dj", &djRoles)
	if err == nil {
		dirty = false
	}
	djLock.RUnlock()
	return err
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the song alias list, unregisters the disconnect handler, clears all queues, and disconnects all voice clients.
func Cleanup(self *discordgo.Session) {
	err := saveData()
	if err != nil {
		log.Error(err)
	}
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
}
