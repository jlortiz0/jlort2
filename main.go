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
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime/debug"
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
		log.SetLevel(log.LevelWarn)
	}
	rand.Seed(time.Now().Unix())
	log.Init()
	defer log.Cleanup()
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

var notForThisOne map[string]struct{} = make(map[string]struct{}, 10)

func ready(self *discordgo.Session, event *discordgo.Ready) {
	time.Sleep(5 * time.Millisecond)
	// notForThisOne = make(map[string]struct{}, len(event.Guilds))
	for i := 0; i < len(event.Guilds); i++ {
		err := self.RequestGuildMembers(event.Guilds[i].ID, "", 250, false)
		notForThisOne[event.Guilds[i].ID] = struct{}{}
		if err != nil {
			panic(err)
		}
	}
	updatePfp(self)
	initModules(self)

	self.AddHandler(interactionCreate)
	self.AddHandler(voiceStateUpdate)
	self.AddHandler(newGuild)
	log.Info("Ready!")
}

func interactionCreate(self *discordgo.Session, event *discordgo.InteractionCreate) {
	if event.Type != discordgo.InteractionApplicationCommand {
		if event.Type == discordgo.InteractionPing {
			self.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponsePong})
		}
		return
	}
	data := event.ApplicationCommandData()
	cmd := commands.GetCommand(data.Name)
	if cmd != nil {
		ctx := commands.MakeContext(self, event.Interaction)
		var err error
		var stack string
		defer func() {
			if err == nil {
				err, _ = recover().(error)
				stack = string(debug.Stack())
			}
			if err != nil {
				handleCommandError(err, ctx, stack)
			}
		}()
		err = cmd(ctx)
	} else if event.GuildID == "" && event.Author.ID != ownerid {
		channel, err2 := self.UserChannelCreate(ownerid)
		if err2 == nil {
			if len(event.Content) < 1965 {
				self.ChannelMessageSend(channel.ID, "Non-command message from "+event.Author.Username+"\n"+event.Content)
			} else {
				self.ChannelMessageSend(channel.ID, "Non-command message from "+event.Author.Username)
			}
		}
	}
}

func handleCommandError(err error, ctx commands.Context, stack string) {
	log.Errors(fmt.Sprintf("Error in command %s", ctx.ApplicationCommandData().Name))
	log.Error(err)
	if stack != "" {
		log.Errors(stack)
	}
	if ctx.User.ID == commands.OWNER_ID {
		if len(err.Error()) < 1990 {
			ctx.Respond(fmt.Sprintf("Error: %s", err.Error()))
		} else {
			ctx.Respond("A lengthy error occured.")
		}
	} else {
		err2 := ctx.Respond("Sorry, something went wrong. An error report was sent to jlortiz.")
		if err2 == nil {
			channel, err2 := ctx.Bot.UserChannelCreate(ownerid)
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
