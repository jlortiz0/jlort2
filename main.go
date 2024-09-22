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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mattn/go-isatty"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var sc chan os.Signal

func main() {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		log.SetLevel(log.LevelWARN)
	}
	log.Init()
	defer log.Cleanup()
	strBytes, err := os.ReadFile("key.txt")
	if err != nil {
		panic(err)
	}
	// GSM_GUILD or tTEST_GUILD, KEY
	key, guildId, _ := strings.Cut(string(strBytes), "\n")
	guildId, motd, _ := strings.Cut(guildId, "\n")
	motd, _, _ = strings.Cut(motd, "\n")
	if key[len(key)-1] == '\r' {
		key = key[:len(key)-1]
		guildId = guildId[:len(guildId)-1]
		if motd[len(motd)-1] == '\r' {
			motd = motd[:len(motd)-1]
		}
	}
	if len(guildId) > 0 {
		s := guildId
		if s[0] == 't' || s[0] == '-' {
			s = s[1:]
		}
		_, err = strconv.ParseUint(s, 10, 64)
		if err != nil {
			panic("key.txt: line 2: could not parse test/gsm guild id")
		}
	}

	// f, err = os.Create("/run/user/1000/cpu.prof")
	// if err != nil {
	//     panic(err)
	// }
	// defer f.Close()
	// err = pprof.StartCPUProfile(f)
	// if err != nil {
	//     panic(err)
	// }
	// defer pprof.StopCPUProfile()

	client, err := discordgo.New("Bot " + key)
	if err != nil {
		panic(err)
	}

	client.AddHandlerOnce(func(self *discordgo.Session, event *discordgo.Ready) { ready(self, event, guildId, motd) })
	client.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions | discordgo.IntentMessageContent
	client.State.MaxMessageCount = 100
	client.State.TrackVoice = true
	err = client.Open()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	sc = make(chan os.Signal)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	// sc2 := make(chan os.Signal)
	// signal.Notify(sc2, syscall.SIGUSR1)
	// go crashMe(sc2)
	<-sc
	log.Info("Stopping...")
	if len(guildId) > 0 && guildId[0] != '-' {
		cleanup(client)
	}
}

func ready(self *discordgo.Session, event *discordgo.Ready, guildId string, motd string) {
	var err error
	time.Sleep(5 * time.Millisecond)
	for i := 0; i < len(event.Guilds); i++ {
		err = self.RequestGuildMembers(event.Guilds[i].ID, "", 250, "", false)
		if err != nil {
			panic(err)
		}
	}
	self.State.Application, err = self.Application("@me")
	if err != nil {
		panic(err)
	}
	go updatePfp(self)
	initModules(self, guildId)
	f, err := os.Open("avatar.png")
	if err == nil {
		defer f.Close()
		avatar := make([]byte, 0x40000)
		c, err := f.Read(avatar)
		if err == nil {
			_, err = self.UserUpdate("", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(avatar[:c]))
			if err != nil {
				log.Error(fmt.Errorf("could not set avatar: %w", err))
			} else {
				log.Warn("Updated profile picture")
				os.Remove("avatar.png")
			}
		} else {
			log.Error(fmt.Errorf("could not read avatar: %w", err))
		}
	}

	self.AddHandler(interactionCreate)
	self.AddHandler(voiceStateUpdate)
	self.AddHandler(newGuild)
	self.AddHandler(oldGuild)
	if motd != "" {
		motd = strings.Clone(motd)
		setMotd(self, motd)
		self.AddHandler(func(self *discordgo.Session, _ *discordgo.Resumed) {
			setMotd(self, motd)
		})
	}
	log.Info("Ready!")
}

func setMotd(self *discordgo.Session, motd string) {
	self.UpdateGameStatus(0, motd)
}

func interactionCreate(self *discordgo.Session, event *discordgo.InteractionCreate) {
	if event.Type == discordgo.InteractionPing {
		self.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponsePong})
		return
	}
	if event.Type == discordgo.InteractionApplicationCommandAutocomplete {
		data := event.ApplicationCommandData()
		cmd := commands.GetCommandAutocomplete(data.Name)
		out := cmd(commands.MakeContext(self, event.Interaction))
		self.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: out},
		})
	}
	if event.Type != discordgo.InteractionApplicationCommand && event.Type != discordgo.InteractionMessageComponent {
		return
	}
	var cmd commands.Command
	if event.Type == discordgo.InteractionMessageComponent {
		data := event.MessageComponentData()
		cmd = commands.GetCommandComponentHandler(data)
	} else {
		data := event.ApplicationCommandData()
		cmd = commands.GetCommand(data.Name)
	}
	if cmd != nil {
		ctx := commands.MakeContext(self, event.Interaction)
		var err error
		var stack string
		defer func() {
			if err == nil {
				x := recover()
				var ok bool
				err, ok = x.(error)
				if !ok {
					s, _ := x.(string)
					if s != "" {
						err = errors.New(s)
					}
				}
				stack = string(debug.Stack())
			}
			if err != nil {
				handleCommandError(err, ctx, stack)
			}
		}()
		err = cmd(ctx)
	}
}

func handleCommandError(err error, ctx *commands.Context, stack string) {
	if ctx.Type == discordgo.InteractionMessageComponent {
		log.Errors("Error in message component")
		log.Error(err)
		if stack != "" {
			log.Errors(stack)
		}
		return
	}
	log.Errors(fmt.Sprintf("Error in command %s", ctx.ApplicationCommandData().Name))
	log.Error(err)
	if stack != "" {
		log.Errors(stack)
	}
	if ctx.User.ID == ctx.State.Application.Owner.ID {
		if len(err.Error()) < 1990 {
			ctx.RespondPrivate(fmt.Sprintf("Error: %s", err.Error()))
		} else {
			ctx.RespondPrivate("A lengthy error occured.")
		}
	} else {
		err2 := ctx.RespondPrivate("Sorry, something went wrong. An error report was sent to " + ctx.State.Application.Owner.GlobalName)
		if err2 == nil {
			channel, err2 := ctx.Bot.UserChannelCreate(ctx.State.Application.Owner.ID)
			if err2 == nil {
				if len(err.Error()) < 1965 {
					ctx.Bot.ChannelMessageSend(channel.ID, fmt.Sprintf("Error in command %s: %s", ctx.ApplicationCommandData().Name, err.Error()))
				} else {
					ctx.Bot.ChannelMessageSend(channel.ID, "A lengthy error occured.")
				}
			}
		}
	}
}
