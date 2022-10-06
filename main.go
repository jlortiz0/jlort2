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

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/mattn/go-isatty"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var sc chan os.Signal
var ownerid string

func main() {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		log.SetLevel(log.LevelWarn)
	}
start:
	strBytes, err := os.ReadFile("key.txt")
	if err != nil {
		panic(err)
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

	client, err := discordgo.New("Bot " + string(strBytes))
	if err != nil {
		panic(err)
	}
	err = commands.LoadPersistent("vachan", &voiceAnnounce)
	if err != nil {
		panic(err)
	}

	client.AddHandlerOnce(ready)
	client.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions | discordgo.IntentsDirectMessages
	client.State.MaxMessageCount = 100
	client.State.TrackVoice = true
	err = client.Open()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	sc = make(chan os.Signal)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	// sc2 := make(chan os.Signal)
	// signal.Notify(sc2, syscall.SIGUSR1)
	// go crashMe(sc2)
	if (<-sc) == syscall.SIGHUP && log.GetLevel() == log.LevelWarn {
		cleanup(client)
		client.Close()
		goto start
	}
	log.Info("Stopping...")
	cleanup(client)
}

func crashMe(ch chan os.Signal) {
	<-ch
	debug.SetTraceback("all")
	var test *int
	*test = 0
}

var notForThisOne map[string]bool = make(map[string]bool, 10)

func ready(self *discordgo.Session, event *discordgo.Ready) {
	time.Sleep(5 * time.Millisecond)
	var err error
	// notForThisOne = make(map[string]bool, len(event.Guilds))
	for i := 0; i < len(event.Guilds); i++ {
		err := self.RequestGuildMembers(event.Guilds[i].ID, "", 250, false)
		notForThisOne[event.Guilds[i].ID] = true
		if err != nil {
			panic(err)
		}
	}
	initModules(self)
	avatar, err := os.ReadFile("avatar.png")
	if err == nil {
		_, err = self.UserUpdate("", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(avatar))
		if err != nil {
			log.Error(fmt.Errorf("could not set avatar: %w", err))
		} else {
			log.Warn("Updated profile picture")
			os.Remove("avatar.png")
		}
	} else if (!errors.Is(err, os.ErrNotExist)) {
		log.Error(fmt.Errorf("could not read avatar: %w", err))
	}

	app, err := self.Application("@me")
	if err != nil {
		log.Error(fmt.Errorf("could not retrieve owner id: %w", err))
	} else {
		ownerid = app.Owner.ID
	}
	self.AddHandler(messageCreate)
	self.AddHandler(voiceStateUpdate)
	self.AddHandler(newGuild)
	self.UpdateGameStatus(0, "~!help")
	log.Info("Ready!")
}

func messageCreate(self *discordgo.Session, event *discordgo.MessageCreate) {
	if event.Author.Bot {
		return
	}

	if strings.HasPrefix(event.Content, "~!") || strings.HasPrefix(event.Content, "!!") {
		args := make([]string, 0, 8)
		for _, v := range strings.Split(event.Content, " ") {
			if len(v) > 0 {
				args = append(args, v)
			}
		}
		cmd := commands.GetCommand(args[0][2:])
		if cmd != nil {
			log.Fine(event.Content)
			ctx := commands.MakeContext(self, event, args[0][2:])
			var err error
			var stack string
			defer func() {
				if err == nil {
					err, _ = recover().(error)
					stack = string(debug.Stack())
				}
				if err != nil {
					handleCommandError(err, ctx, args, stack)
				}
			}()
			err = cmd(ctx, args[1:])
		}
	}
}

func handleCommandError(err error, ctx commands.Context, args []string, stack string) {
	log.Errors(fmt.Sprintf("Error in command %s", ctx.InvokedWith))
	log.Error(err)
	if stack != "" {
		log.Errors(stack)
	}
	if ctx.Author.ID == ownerid {
		if len(err.Error()) < 1990 {
			ctx.Send(fmt.Sprintf("Error: %s", err.Error()))
		} else {
			ctx.Send("A lengthy error occured.")
		}
	} else {
		err2 := ctx.Send("Sorry, something went wrong. An error report was sent to jlortiz.")
		if err2 == nil {
			channel, err2 := ctx.Bot.UserChannelCreate(ownerid)
			if err2 == nil {
				if len(err.Error()) < 1965 {
					ctx.Bot.ChannelMessageSend(channel.ID, fmt.Sprintf("Error in command %s: %s", ctx.InvokedWith, err.Error()))
				} else {
					ctx.Bot.ChannelMessageSend(channel.ID, "A lengthy error occured.")
				}
			}
		}
	}
}
