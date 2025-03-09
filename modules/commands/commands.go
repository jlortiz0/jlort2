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
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var db *sql.DB

// Context is a helper struct for defining a command invokation context.
// All this can be gotten from the three fields in MakeContext, but this makes it shorter to do so.
type Context struct {
	*discordgo.Interaction

	Bot        *discordgo.Session
	Me         *discordgo.User
	State      *discordgo.State
	Database   *sql.DB
	origName   string
	followup   string
	components []discordgo.MessageComponent
	hasDelayed bool
}

func (ctx *Context) resp(msg string, embed *discordgo.MessageEmbed, private bool) error {
	if ctx.followup == "0" {
		ctx.RespondDelayed(private)
		data := new(discordgo.WebhookParams)
		data.Content = msg
		data.Components = ctx.components
		if private {
			data.Flags = discordgo.MessageFlagsEphemeral
		}
		if embed != nil {
			data.Embeds = []*discordgo.MessageEmbed{embed}
		}
		s, err := ctx.Bot.FollowupMessageCreate(ctx.Interaction, true, data)
		if err != nil {
			err = fmt.Errorf("failed to send response: %w", err)
		} else {
			ctx.followup = s.ID
		}
		return err
	} else if ctx.followup != "" {
		resp := new(discordgo.WebhookEdit)
		resp.Content = &msg
		resp.Components = &ctx.components
		if embed != nil {
			resp.Embeds = &[]*discordgo.MessageEmbed{embed}
		}
		_, err := ctx.Bot.FollowupMessageEdit(ctx.Interaction, ctx.followup, resp)
		if err != nil {
			err = fmt.Errorf("failed to send response: %w", err)
		}
		return err
	}
	resp := new(discordgo.InteractionResponse)
	if ctx.origName == "" {
		resp.Type = discordgo.InteractionResponseChannelMessageWithSource
	} else {
		resp.Type = discordgo.InteractionResponseUpdateMessage
	}
	resp.Data = new(discordgo.InteractionResponseData)
	resp.Data.Content = msg
	resp.Data.Components = ctx.components
	if private {
		resp.Data.Flags = discordgo.MessageFlagsEphemeral
	}
	if embed != nil {
		resp.Data.Embeds = []*discordgo.MessageEmbed{embed}
	}
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	}
	return err
}

// Send a message to the channel where the command was invoked.
func (ctx *Context) Respond(msg string) error {
	if ctx.hasDelayed {
		return ctx.RespondEdit(msg)
	}
	return ctx.resp(msg, nil, false)
}

func (ctx *Context) RespondPrivate(msg string) error {
	if ctx.hasDelayed {
		return ctx.RespondEdit(msg)
	}
	return ctx.resp(msg, nil, true)
}

func (ctx *Context) RespondEmbed(embed *discordgo.MessageEmbed, private bool) error {
	if ctx.hasDelayed {
		return ctx.RespondEditEmbed(embed)
	}
	return ctx.resp("", embed, private)
}

func (ctx *Context) RespondDelayed(private bool) error {
	if ctx.hasDelayed {
		return nil
	}
	resp := new(discordgo.InteractionResponse)
	resp.Type = discordgo.InteractionResponseDeferredChannelMessageWithSource
	if ctx.origName != "" {
		resp.Type = discordgo.InteractionResponseDeferredMessageUpdate
	}
	resp.Data = new(discordgo.InteractionResponseData)
	if private {
		resp.Data.Flags = discordgo.MessageFlagsEphemeral
	}
	err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to send response: %w", err)
	} else {
		ctx.hasDelayed = true
	}
	return err
}

func (ctx *Context) RespondEdit(msg string) error {
	if ctx.origName != "" {
		return ctx.Respond(msg)
	}
	resp := new(discordgo.WebhookEdit)
	resp.Content = &msg
	resp.Components = &ctx.components
	_, err := ctx.Bot.InteractionResponseEdit(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to edit response: %w", err)
	}
	return err
}

func (ctx *Context) RespondEditEmbed(embed *discordgo.MessageEmbed) error {
	if ctx.origName != "" {
		return ctx.RespondEmbed(embed, false)
	}
	resp := new(discordgo.WebhookEdit)
	resp.Embeds = &[]*discordgo.MessageEmbed{embed}
	resp.Components = &ctx.components
	_, err := ctx.Bot.InteractionResponseEdit(ctx.Interaction, resp)
	if err != nil {
		err = fmt.Errorf("failed to edit response: %w", err)
	}
	return err
}

func (ctx *Context) RespondEmpty() error {
	if ctx.origName != "" {
		return ctx.Bot.InteractionRespond(ctx.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	}
	if !ctx.hasDelayed {
		resp := new(discordgo.InteractionResponse)
		resp.Type = discordgo.InteractionResponseDeferredChannelMessageWithSource
		resp.Data = new(discordgo.InteractionResponseData)
		resp.Data.Flags = discordgo.MessageFlagsEphemeral
		err := ctx.Bot.InteractionRespond(ctx.Interaction, resp)
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
	}
	return ctx.Bot.InteractionResponseDelete(ctx.Interaction)
}

func (ctx *Context) SetComponents(args ...discordgo.MessageComponent) {
	if len(args) == 0 {
		return
	}
	name := ctx.origName + "\a"
	if name == "\a" {
		name = ctx.ApplicationCommandData().Name + "\a"
	}
	for i, x := range args {
		if x.Type() == discordgo.ButtonComponent {
			x2 := x.(discordgo.Button)
			x2.CustomID = name + x2.CustomID
			args[i] = x2
		} else {
			return
		}
	}
	com := new(discordgo.ActionsRow)
	com.Components = args
	ctx.components = []discordgo.MessageComponent{com}
}

func (ctx *Context) FollowupPrepare() {
	ctx.followup = "0"
}

func (ctx *Context) FollowupDestroy() {
	ctx.followup = ""
}

// MakeContext returns a Context populated with data from the message event.
func MakeContext(self *discordgo.Session, event *discordgo.Interaction) *Context {
	ctx := new(Context)
	ctx.Interaction = event
	if ctx.Member != nil {
		ctx.User = event.Member.User
	}
	ctx.Bot = self
	ctx.State = self.State
	ctx.Me = self.State.User
	if event.Type == discordgo.InteractionMessageComponent {
		data := event.MessageComponentData()
		ctx.origName, data.CustomID, _ = strings.Cut(data.CustomID, "\a")
		event.Data = data
	}
	ctx.Database = db
	return ctx
}

// Command defines the function interface for a valid command.
type Command func(*Context) error
type Autocompleter func(*Context) []*discordgo.ApplicationCommandOptionChoice
type cmdMapEntry struct {
	c Command
	a Autocompleter
	h Command // Component handler
}

var batchCmdList []commandStruct
var cmdMap map[string]cmdMapEntry

type commandStruct struct {
	*discordgo.ApplicationCommand
	autocomplete Autocompleter
	handler      Command
	gsm          bool
}

func PrepareCommand(name, description string) commandStruct {
	cmd := new(discordgo.ApplicationCommand)
	cmd.Description = description
	cmd.Name = name
	cmd.Type = discordgo.ChatApplicationCommand
	return commandStruct{ApplicationCommand: cmd}
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

func (c commandStruct) Auto(c2 Autocompleter) commandStruct {
	c.autocomplete = c2
	return c
}

func (c commandStruct) Component(c2 Command) commandStruct {
	c.handler = c2
	return c
}

func (c commandStruct) Gsm() commandStruct {
	c.gsm = true
	return c
}

func (c commandStruct) Register(cmd Command, options []*discordgo.ApplicationCommandOption) {
	c.Options = options
	batchCmdList = append(batchCmdList, c)
	cmdMap[c.Name] = cmdMapEntry{cmd, c.autocomplete, c.handler}
}

func UploadCommands(self *discordgo.Session, appId string, guildId string, testMode bool) {
	var err error
	if testMode {
		ls := make([]*discordgo.ApplicationCommand, len(batchCmdList))
		for i, x := range batchCmdList {
			ls[i] = x.ApplicationCommand
		}
		_, err = self.ApplicationCommandBulkOverwrite(appId, guildId, ls)
	} else {
		ls := make([]*discordgo.ApplicationCommand, 0, len(batchCmdList))
		ls2 := make([]*discordgo.ApplicationCommand, 0, 4)
		for _, x := range batchCmdList {
			if x.gsm {
				ls2 = append(ls2, x.ApplicationCommand)
			} else {
				ls = append(ls, x.ApplicationCommand)
			}
		}
		_, err = self.ApplicationCommandBulkOverwrite(appId, "", ls)
		if err == nil && guildId != "" {
			_, err = self.ApplicationCommandBulkOverwrite(appId, guildId, ls2)
		}
	}
	if err != nil {
		if testMode {
			err2 := err.(*discordgo.RESTError)
			var errBody struct {
				Errors map[string]any
				Code   int
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

func ClearGuildCommands(self *discordgo.Session, appId string, guildID string) {
	_, err := self.ApplicationCommandBulkOverwrite(appId, guildID, nil)
	if err != nil {
		panic(err)
	}
}

// GetCommand returns the command associated with the given name
func GetCommand(name string) Command {
	return cmdMap[name].c
}

// GetCommand returns the command associated with the given name
func GetCommandAutocomplete(name string) Autocompleter {
	return cmdMap[name].a
}

func GetCommandComponentHandler(data discordgo.MessageComponentInteractionData) Command {
	name, _, _ := strings.Cut(data.CustomID, "\a")
	return cmdMap[name].h
}

func GetDatabase() *sql.DB {
	return db
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

func (c *commandOption) AsChannel(chType []discordgo.ChannelType) *commandOption {
	c.Type = discordgo.ApplicationCommandOptionChannel
	c.ChannelTypes = chType
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

func (c *commandOption) Auto() *commandOption {
	c.Autocomplete = true
	c.Choices = nil
	return c
}

func (c *commandOption) Choice(ls []*discordgo.ApplicationCommandOptionChoice) *commandOption {
	c.Choices = ls
	c.Autocomplete = false
	return c
}

func (c *commandOption) Finalize() *discordgo.ApplicationCommandOption {
	if c.Type == 0 {
		panic("command option type not set")
	}
	return &(*c).ApplicationCommandOption
}
