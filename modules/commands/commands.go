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
	"strconv"

	"github.com/bwmarrin/discordgo"
)

const APP_ID = ""
const OWNER_ID = ""
const TEST_GUILD_ID = ""
const TEST_MODE = true

// Context is a helper struct for defining a command invokation context.
// All this can be gotten from the three fields in MakeContext, but this makes it shorter to do so.
type Context struct {
	*discordgo.Interaction

	Bot   *discordgo.Session
	Me    *discordgo.User
	State *discordgo.State
}

// Send a message to the channel where the command was invoked.
func (ctx Context) Respond(msg string) error {
	resp := new(discordgo.InteractionResponse)
	resp.Type = discordgo.InteractionResponseChannelMessageWithSource
	resp.Data = new(discordgo.InteractionResponseData)
	resp.Data.Content = msg
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	}
	return err
}

func (ctx Context) RespondPrivate(msg string) error {
	resp := new(discordgo.InteractionResponse)
	resp.Type = discordgo.InteractionResponseChannelMessageWithSource
	resp.Data = new(discordgo.InteractionResponseData)
	resp.Data.Content = msg
	resp.Data.Flags = 1 << 6
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	}
	return err
}

func (ctx Context) RespondEmbed(embed *discordgo.MessageEmbed) error {
	resp := new(discordgo.InteractionResponse)
	resp.Type = discordgo.InteractionResponseChannelMessageWithSource
	resp.Data = new(discordgo.InteractionResponseData)
	resp.Data.Embeds = []*discordgo.MessageEmbed{embed}
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	}
	return err
}

func (ctx Context) DelayedRespond() error {
	resp := new(discordgo.InteractionResponse)
	resp.Type = discordgo.InteractionResponseDeferredChannelMessageWithSource
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	}
	return err
}

func (ctx Context) EditResponse(msg string) error {
	resp := new(discordgo.WebhookEdit)
	resp.Content = &msg
	_, err := ctx.Bot.InteractionResponseEdit(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to edit response: %w", err)
	}
	return err
}

func (ctx Context) RespondEditEmbed(embed *discordgo.MessageEmbed) error {
	resp := new(discordgo.WebhookEdit)
	resp.Embeds = &[]*discordgo.MessageEmbed{embed}
	_, err := ctx.Bot.InteractionResponseEdit(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to edit response: %w", err)
	}
	return err
}

// MakeContext returns a Context populated with data from the message event.
func MakeContext(self *discordgo.Session, event *discordgo.Interaction) Context {
	ctx := Context{Interaction: event}
	if ctx.Member != nil {
		ctx.User = event.Member.User
	}
	ctx.Bot = self
	ctx.State = self.State
	ctx.Me = self.State.User
	return ctx
}

// Command defines the function interface for a valid command.
type Command func(Context) error

var batchCmdList []*discordgo.ApplicationCommand
var cmdMap map[string]Command

// RegisterCommand registers a command with the commands module.
// The name need not be the same as the function, but it must be unique.
// A command can have multiple names, and can see which is used to call it.
func RegisterCommand(cmd Command, name string, description string, options []*discordgo.ApplicationCommandOption) {
	cmdStruct := new(discordgo.ApplicationCommand)
	cmdStruct.Description = description
	cmdStruct.Name = name
	cmdStruct.ApplicationID = APP_ID
	cmdStruct.Options = options
	cmdStruct.Type = discordgo.ChatApplicationCommand
	batchCmdList = append(batchCmdList, cmdStruct)
	cmdMap[name] = cmd
}

func UploadCommands(self *discordgo.Session) {
	var err error
	if TEST_MODE {
		_, err = self.ApplicationCommandBulkOverwrite(APP_ID, TEST_GUILD_ID, batchCmdList)
	} else {
		_, err = self.ApplicationCommandBulkOverwrite(APP_ID, "", batchCmdList)
	}
	if err != nil {
		if TEST_MODE {
			err2 := err.(*discordgo.RESTError)
			var errBody struct {
				Code   int
				Errors map[string]interface{}
			}
			json.Unmarshal(err2.ResponseBody, &errBody)
			if errBody.Code == discordgo.ErrCodeInvalidFormBody {
				for k := range errBody.Errors {
					i, _ := strconv.Atoi(k)
					fmt.Printf("%d: %s\n", i, batchCmdList[i].Name)
					for _, v := range batchCmdList[i].Options {
						fmt.Printf(" - %s (%d)\n", v.Name, v.Type)
					}
				}
			}
		}
		panic(err)
	}
	batchCmdList = nil
}

// GetCommand returns the command associated with the given name
func GetCommand(name string) Command {
	return cmdMap[name]
}

// FindMember returns the first member with the given name or nickname from a guild
// If the name begins with @, the @ is stripped before searching.
// If no member is found, but there was no error getting the members, nil, nil is returned.
// If there was an error getting the members, nil, error is returned.
func FindMembers(self *discordgo.Session, name string, guildID string) (*discordgo.Member, error) {
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

type commandOption struct {
	discordgo.ApplicationCommandOption
}

func NewCommandOption(name, description string) *commandOption {
	ret := new(commandOption)
	ret.Name = name
	ret.Description = description
	return ret
}

func (c *commandOption) AsInt() *commandOption {
	c.Type = discordgo.ApplicationCommandOptionInteger
	return c
}

func (c *commandOption) AsString() *commandOption {
	c.Type = discordgo.ApplicationCommandOptionString
	return c
}

func (c *commandOption) SetMinMax(min, max int) *commandOption {
	min2 := float64(min)
	c.MinValue = &min2
	max2 := float64(max)
	c.MaxValue = max2
	return c
}

func (c *commandOption) Required() *commandOption {
	c.ApplicationCommandOption.Required = true
	return c
}

func (c *commandOption) Finalize() *discordgo.ApplicationCommandOption {
	return &(*c).ApplicationCommandOption
}
