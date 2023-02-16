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
const GSM_GUILD = ""
const TEST_MODE = true

// Context is a helper struct for defining a command invokation context.
// All this can be gotten from the three fields in MakeContext, but this makes it shorter to do so.
type Context struct {
	*discordgo.Interaction

	Bot        *discordgo.Session
	Me         *discordgo.User
	State      *discordgo.State
	hasDelayed int
}

// Send a message to the channel where the command was invoked.
func (ctx Context) Respond(msg string) error {
	if ctx.hasDelayed == 1 {
		return ctx.RespondEdit(msg)
	} else if ctx.hasDelayed != 0 {
		ctx.Bot.InteractionResponseDelete(ctx.Interaction)
	}
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
	if ctx.hasDelayed == 1 {
		ctx.Bot.InteractionResponseDelete(ctx.Interaction)
	} else if ctx.hasDelayed != 0 {
		return ctx.RespondEdit(msg)
	}
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

func (ctx Context) RespondEmbed(embed *discordgo.MessageEmbed, private bool) error {
	if ctx.hasDelayed != 0 {
		temp := ctx.hasDelayed ^ 1
		if private {
			temp ^= (1 << 6)
		}
		if temp == 0 {
			return ctx.RespondEditEmbed(embed)
		}
		ctx.Bot.InteractionResponseDelete(ctx.Interaction)
	}
	resp := new(discordgo.InteractionResponse)
	resp.Type = discordgo.InteractionResponseChannelMessageWithSource
	resp.Data = new(discordgo.InteractionResponseData)
	resp.Data.Embeds = []*discordgo.MessageEmbed{embed}
	if private {
		resp.Data.Flags = 1 << 6
	}
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	}
	return err
}

func (ctx *Context) RespondDelayed(private bool) error {
	if ctx.hasDelayed != 0 {
		temp := ctx.hasDelayed ^ 1
		if private {
			temp ^= (1 << 6)
		}
		if temp == 0 {
			return nil
		}
		ctx.Bot.InteractionResponseDelete(ctx.Interaction)
	}
	resp := new(discordgo.InteractionResponse)
	resp.Type = discordgo.InteractionResponseDeferredChannelMessageWithSource
	resp.Data = new(discordgo.InteractionResponseData)
	if private {
		resp.Data.Flags = 1 << 6
	}
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	} else {
		ctx.hasDelayed = int(resp.Data.Flags | 1)
	}
	return err
}

func (ctx Context) RespondEdit(msg string) error {
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

func (ctx Context) RespondEmpty() error {
	if ctx.hasDelayed == 0 {
		resp := new(discordgo.InteractionResponse)
		resp.Type = discordgo.InteractionResponseDeferredChannelMessageWithSource
		resp.Data = new(discordgo.InteractionResponseData)
		resp.Data.Flags = 1 << 6
		err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
	}
	return ctx.Bot.InteractionResponseDelete(ctx.Interaction)
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

type commandStruct struct {
	*discordgo.ApplicationCommand
}

func PrepareCommand(name, description string) commandStruct {
	cmd := new(discordgo.ApplicationCommand)
	cmd.Description = description
	cmd.Name = name
	cmd.ApplicationID = APP_ID
	cmd.Type = discordgo.ChatApplicationCommand
	return commandStruct{cmd}
}

func (c commandStruct) AsMsg() commandStruct {
	c.Type = discordgo.MessageApplicationCommand
	c.Description = ""
	return c
}

func (c commandStruct) AsUser() commandStruct {
	c.Type = discordgo.UserApplicationCommand
	c.Description = ""
	return c
}

func (c commandStruct) NSFW() commandStruct {
	b := true
	c.ApplicationCommand.NSFW = &b
	return c
}

func (c commandStruct) Guild() commandStruct {
	b := false
	c.DMPermission = &b
	return c
}

func (c commandStruct) Perms(p int64) commandStruct {
	c.DefaultMemberPermissions = &p
	return c
}

func (c commandStruct) Register(cmd Command, options []*discordgo.ApplicationCommandOption) {
	c.Options = options
	batchCmdList = append(batchCmdList, c.ApplicationCommand)
	cmdMap[c.Name] = cmd
}

func RegisterGsmGuildCommand(self *discordgo.Session, c commandStruct, cmd Command, options []*discordgo.ApplicationCommandOption) {
	if TEST_MODE {
		c.Register(cmd, options)
		return
	}
	c.Options = options
	_, err := self.ApplicationCommandCreate(APP_ID, GSM_GUILD, c.ApplicationCommand)
	if err != nil {
		panic(err)
	}
	cmdMap[c.Name] = cmd
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

func clearGuildCommands(self *discordgo.Session, guildID string) {
	_, err := self.ApplicationCommandBulkOverwrite(APP_ID, guildID, nil)
	if err != nil {
		panic(err)
	}
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

func (c *commandOption) AsUser() *commandOption {
	c.Type = discordgo.ApplicationCommandOptionUser
	return c
}

func (c *commandOption) AsBool() *commandOption {
	c.Type = discordgo.ApplicationCommandOptionBoolean
	return c
}

func (c *commandOption) AsChannel() *commandOption {
	c.Type = discordgo.ApplicationCommandOptionChannel
	return c
}

func (c *commandOption) AsRole() *commandOption {
	c.Type = discordgo.ApplicationCommandOptionRole
	return c
}

func (c *commandOption) AsSubcommand(o []*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
	c.Type = discordgo.ApplicationCommandOptionSubCommand
	c.Options = o
	return &c.ApplicationCommandOption
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
	if c.Type == 0 {
		panic("command option type not set")
	}
	return &(*c).ApplicationCommandOption
}
