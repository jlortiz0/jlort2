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

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
)

// Context is a helper struct for defining a command invokation context.
// All this can be gotten from the three fields in MakeContext, but this makes it shorter to do so.
type Context struct {
	Message     *discordgo.Message
	Bot         *discordgo.Session
	InvokedWith string
	Author      *discordgo.User
	Member      *discordgo.Member
	Me          *discordgo.User
	ChanID      string
	GuildID     string
	State       *discordgo.State
}

// Send a message to the channel where the command was invoked.
func (ctx Context) Send(msg string) error {
	_, err := ctx.Bot.ChannelMessageSend(ctx.ChanID, msg)
	if err != nil {
		err = fmt.Errorf("Could not send message: %w", err)
	}
	return err
}

// MakeContext returns a Context populated with data from the message event.
func MakeContext(self *discordgo.Session, event *discordgo.MessageCreate, invocation string) Context {
	ctx := Context{Bot: self, InvokedWith: invocation}
	ctx.Author = event.Author
	if event.Member != nil {
		ctx.Member = event.Member
		ctx.Member.User = event.Author
	}
	ctx.Message = event.Message
	ctx.ChanID = event.ChannelID
	ctx.GuildID = event.GuildID
	ctx.State = self.State
	ctx.Me = self.State.User
	return ctx
}

// Command defines the function interface for a valid command.
type Command func(Context, []string) error

var cmdMap map[string]Command

// RegisterCommand registers a command with the commands module.
// The name need not be the same as the function, but it must be unique.
// A command can have multiple names, and can see which is used to call it.
func RegisterCommand(cmd Command, name string) {
	_, in := cmdMap[name]
	if in {
		panic(fmt.Sprintf("Command name %s already registered", name))
	}
	cmdMap[name] = cmd
}

// GetCommand returns the command associated with the given name
func GetCommand(name string) Command {
	return cmdMap[name]
}

// UnregisterCommand dissociates the command with the given name
// The function will not error even if there is no command with that name
func UnregisterCommand(name string) {
	delete(cmdMap, name)
}

// FindMember returns the first member with the given name or nickname from a guild
// If the name begins with @, the @ is stripped before searching.
// If no member is found, but there was no error getting the members, nil, nil is returned.
// If there was an error getting the members, nil, error is returned.
func FindMember(self *discordgo.Session, name string, guildID string) (*discordgo.Member, error) {
	guild, err := self.State.Guild(guildID)
	if err != nil {
		return nil, err
	}
	if name[0] == '<' && name[1] == '@' && name[len(name)-1] == '>' {
		if name[2] == '!' {
			return self.GuildMember(guildID, name[3:len(name)-1])
		}
		return self.GuildMember(guildID, name[2:len(name)-1])
	}
	for i := 0; i < len(guild.Members); i++ {
		if guild.Members[i].Nick == name || guild.Members[i].User.Username == name {
			return guild.Members[i], nil
		}
	}
	return nil, nil
}

// DisplayName returns the nickname of a member, or the username if there is none.
func DisplayName(mem *discordgo.Member) string {
	if mem.Nick == "" {
		return mem.User.Username
	}
	return mem.Nick
}

// LoadPersistent loads data from a persistent file to the given pointer
func LoadPersistent(name string, data interface{}) error {
	b, err := os.ReadFile("persistent/" + name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, data)
}

// SavePersistent saves data to a persistent file from the given pointer
func SavePersistent(name string, data interface{}) error {
	if data == nil {
		panic("Refusing to save null pointer")
	}
	output, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = os.WriteFile("persistent/"+name+".new", output, 0600)
	if err != nil {
		return err
	}
	return os.Rename("persistent/"+name+".new", "persistent/"+name)
}
