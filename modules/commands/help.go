package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type helpData struct {
	Name    string
	Flags   byte // 1 GuildOnly, 2 Hidden, 4 NSFW
	Syntax  string
	Aliases []string
	Doc     string
}

var helpMap map[string]*helpData

// ~!version
// Get bot info
func version(ctx Context, _ []string) error {
	return ctx.Send("jlort jlort 2.1.4g running on discordgo v" + discordgo.VERSION)
}

// ~!help [command]
// @Hidden
// Well, clearly you know how to use this...
func help(ctx Context, args []string) error {
	if len(args) == 0 {
		builder := new(strings.Builder)
		names := make([]string, 0, len(helpMap))
		for k, v := range helpMap {
			if v.Flags&1 != 0 && ctx.GuildID == "" {
				continue
			} else if v.Flags&6 != 0 {
				continue
			} else if v.Name == k {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		builder.WriteString("```  -- jlort jlort 2 commands --\n\n")
		for _, v := range names {
			data := helpMap[v]
			builder.WriteString(fmt.Sprintf(" ~!%-16s", v))
			doc := data.Doc
			pos := strings.IndexByte(doc, '\n')
			if pos != -1 {
				doc = doc[:pos]
			}
			builder.WriteString(doc)
			builder.WriteByte('\n')
		}
		builder.WriteString("```")
		return ctx.Send(builder.String())
	}
	cmdName := args[0]
	data := helpMap[cmdName]
	if data == nil {
		return ctx.Send("No such command " + cmdName)
	}
	embed := new(discordgo.MessageEmbed)
	embed.Title = data.Syntax
	embed.Description = data.Doc
	if len(data.Aliases) > 0 {
		builder := strings.Join(data.Aliases, ", ")
		embed.Footer = &discordgo.MessageEmbedFooter{Text: "Also known as " + builder}
	}
	_, err := ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
	return err
}

func loadHelpData() error {
	f, err := os.Open("modules" + string(os.PathSeparator) + "commands" + string(os.PathSeparator) + "help.db")
	if err != nil {
		return err
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	for {
		name, err := rd.ReadString(0)
		if err != nil {
			break
		}
		data := new(helpData)
		data.Name = name[:len(name)-1]
		data.Flags, _ = rd.ReadByte()
		syntax, err := rd.ReadString(0)
		if err != nil {
			break
		}
		data.Syntax = syntax[:len(syntax)-1]
		aCount, _ := rd.ReadByte()
		data.Aliases = make([]string, aCount)
		for i := byte(0); i < aCount; i++ {
			a, err := rd.ReadString(0)
			if err != nil {
				break
			}
			data.Aliases[i] = a[:len(a)-1]
		}
		data.Doc, err = rd.ReadString(0)
		data.Doc = data.Doc[:len(data.Doc)-1]
		if err != nil {
			break
		}
		helpMap[data.Name] = data
		for _, a := range data.Aliases {
			helpMap[a] = data
		}
	}
	if err != io.EOF {
		return err
	}
	return nil
}
