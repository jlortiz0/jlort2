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
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/nathan-fiscaletti/consolesize-go"
)

func checkFatal(e error) {
	if e != nil {
		panic(e)
	}
}

func clear() {
	fmt.Print("\033[H\033[2J")
}

func readInt(prefix string, max int) int {
BeginRead:
	fmt.Print(prefix)
	s, err := input.ReadString('\n')
	if err != nil {
		return -1 // Assume end of input
	}
	s = strings.Trim(s, " \r\n\t")
	if s == "exit" {
		return -1
	}
	i, err := strconv.Atoi(s)
	if err != nil || i < 1 {
		fmt.Println("Not a number.")
		goto BeginRead
	}
	if i > max && max > 0 {
		fmt.Printf("Number too large, expected 1-%d\n", max)
		goto BeginRead
	}
	return i
}

var client *discordgo.Session
var input *bufio.Reader
var output *bufio.Writer
var sc chan os.Signal
var height int

func main() {
	_, height = consolesize.GetConsoleSize()
	input = bufio.NewReader(os.Stdin)
	output = bufio.NewWriterSize(os.Stdout, 20480)
	strBytes, err := os.ReadFile("key.txt")
	if err != nil {
		panic(err)
	}
	key, _, _ := strings.Cut(string(strBytes), "\n")
	if key[len(key)-1] == '\r' {
		key = key[:len(key)-1]
	}

	fmt.Println("Starting...")
	client, err = discordgo.New("Bot " + key)
	checkFatal(err)
	client.AddHandlerOnce(ready)
	intent := discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages
	client.Identify.Intents = discordgo.MakeIntent(intent)
	checkFatal(client.Open())
	defer client.Close()

	sc = make(chan os.Signal)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	<-sc
	fmt.Println("Stopping...")
}

func ready(self *discordgo.Session, event *discordgo.Ready) {
	time.Sleep(50 * time.Millisecond)
	var err error
	for i := 0; i < len(event.Guilds); i++ {
		err = self.RequestGuildMembers(event.Guilds[i].ID, "", 250, "", false)
		checkFatal(err)
	}
	usd := discordgo.UpdateStatusData{Status: "invisible"}
	err = self.UpdateStatusComplex(usd)
	checkFatal(err)
	go guildAndModeSel()
}

func guildAndModeSel() {
	for {
		clear()
		for k, v := range client.State.Guilds {
			if v.Unavailable {
				fmt.Fprintf(output, "%d. Unavailable\n", k+1)
			} else {
				fmt.Fprintf(output, "%d. %s\n", k+1, v.Name)
			}
		}
		fmt.Fprintln(output, "Or type exit to exit")
		output.Flush()
		ind := readInt("Sel: ", len(client.State.Guilds))
		if ind == -1 {
			break
		}
		guild := client.State.Guilds[ind-1]
		if !guild.Unavailable {
			if len(guild.Members) == 1 {
				var err error
				guild.Members, err = client.GuildMembers(guild.ID, "", 250)
				if err == nil {
					client.State.GuildAdd(guild)
				}
			}
			for {
				if guild.Unavailable {
					break
				}
				clear()
				fmt.Printf("On guild %s:\n\n1. Channels\n2. Users\n3. Attachment count\n4. Chatlogger\nOr type exit to exit\n\n", guild.Name)
				ind := readInt("Sel: ", 4)
				if ind == -1 {
					break
				}
				switch ind {
				case 1:
					channel := channelSel(guild)
					if channel != nil {
						chatter(channel, guild)
					}
				case 2:
					clear()
					fmt.Fprintf(output, "On guild %s (%s):\n\n", guild.Name, guild.ID)
					for k, v := range guild.Members {
						if v.Nick != "" {
							fmt.Fprintf(output, "%d. %s (%s) ", k+1, v.Nick, v.User.DisplayName())
						} else {
							fmt.Fprintf(output, "%d. %s ", k+1, v.User.DisplayName())
						}
						if v.User.ID == client.State.User.ID {
							fmt.Fprint(output, "(Me)")
						} else if v.User.Bot {
							fmt.Fprint(output, "(Bot)")
						} else if v.User.ID == guild.OwnerID {
							fmt.Fprint(output, "(Owner)")
						}
						output.WriteByte('\n')
					}
					fmt.Fprintln(output, "Or type exit to exit")
					output.Flush()
					ind := readInt("Sel: ", guild.MemberCount)
					if ind == -1 {
						continue
					}
					user := guild.Members[ind-1]
					clear()
					fmt.Fprintf(output, "Username: %s (%s)\n", user.User.GlobalName, user.User.Username)
					if user.Nick != "" {
						fmt.Fprintf(output, "Nick: %s\n", user.Nick)
					}
					fmt.Fprintf(output, "ID: %s\n", user.User.ID)
					fmt.Fprintf(output, "Avatar URL: %s\n", user.User.AvatarURL(""))
					createDate, _ := discordgo.SnowflakeTimestamp(user.User.ID)
					fmt.Fprintf(output, "Created at: %s\n", createDate.In(time.Local).Format("Jan _2 2006, 15:04"))
					fmt.Fprintf(output, "Joined at: %s\n", user.JoinedAt.In(time.Local).Format("Jan _2 2006, 15:04"))
					if len(user.Roles) != 0 {
						fmt.Fprintln(output, "Roles:")
						for _, v := range user.Roles {
							fmt.Fprintf(output, "- %s\n", v)
						}
					}
					presence, err := client.State.Presence(guild.ID, user.User.ID)
					if err == nil {
						fmt.Fprintf(output, "Status: %s\n", presence.Status)
						if len(presence.Activities) > 0 {
							fmt.Fprintf(output, "Playing: %s (%s)\n", presence.Activities[0].Name, presence.Activities[0].URL)
						}
					}
					if user.User.Bot {
						fmt.Fprintln(output, "User is a bot, cannot DM. Press enter to exit.")
					} else {
						fmt.Fprintln(output, "Type anything to start a DM, or nothing to exit.")
					}
					output.Flush()
					d, _ := input.ReadBytes('\n')
					if !user.User.Bot && len(d) != 0 && d[0] != '\n' && d[0] != '\r' {
						channel, err := client.UserChannelCreate(user.User.ID)
						if err != nil {
							fmt.Println(err)
							input.ReadBytes('\n')
							continue
						}
						chatter(channel, nil)
					}
				case 3:
					channel := channelSel(guild)
					if channel == nil {
						continue
					}
					fmt.Println("Scanning...")
					lastMsg := ""
					attach := 0
					attachSize := 0
					embeds := 0
					msgs := 0
					for {
						toProc, err := client.ChannelMessages(channel.ID, 100, lastMsg, "", "")
						if err != nil {
							fmt.Println(err)
							input.ReadBytes('\n')
							break
						}
						if len(toProc) == 0 {
							break
						}
						msgs += len(toProc)
						for _, v := range toProc {
							for _, a := range v.Attachments {
								attach++
								attachSize += a.Size
							}
							embeds += len(v.Embeds)
							lastMsg = v.ID
						}
					}
					fmt.Printf("\nAttach count: %d\nEmbed count: %d\nMessage count: %d\nAttach size: %d\n\nPress any key to continue...", attach, embeds, msgs, attachSize)
					input.ReadBytes('\n')
				case 4:
					channel := channelSel(guild)
					if channel != nil {
						count := readInt("ID of first message to save or 1 for all: ", -1)
						if count <= 0 {
							break
						}
						chatlog(channel, guild, count-1)
					}
				}
			}
		}
	}
	sc <- nil
}

func channelSel(guild *discordgo.Guild) *discordgo.Channel {
	clear()
	fmt.Fprintf(output, "On guild %s (%s):\n\n", guild.Name, guild.ID)
	channels := make([]*discordgo.Channel, 0, len(guild.Channels))
	for _, v := range guild.Channels {
		if v.Type == discordgo.ChannelTypeGuildText {
			channels = append(channels, v)
		}
	}
	for k, v := range channels {
		if v.NSFW {
			fmt.Fprintf(output, "%d. %s (NSFW)\n", k+1, v.Name)
		} else {
			fmt.Fprintf(output, "%d. %s\n", k+1, v.Name)
		}
	}
	fmt.Fprintln(output, "Or type exit to exit")
	output.Flush()
	ind := readInt("Sel: ", len(channels))
	if ind == -1 {
		return nil
	}
	return channels[ind-1]
}

func chatter(channel *discordgo.Channel, guild *discordgo.Guild) {
	var pager []string
	nicks := make(map[string]string)

	for {
		clear()
		fmt.Fprintf(output, "Channel: %s", channel.Name)
		if channel.Topic != "" {
			fmt.Fprintf(output, " (%s)", channel.Topic)
		}
		output.WriteByte('\n')

		var lastMsg string
		if len(pager) != 0 {
			lastMsg = pager[len(pager)-1]
		}
		msgs, err := client.ChannelMessages(channel.ID, height-5, lastMsg, "", "")
		if err != nil {
			fmt.Println(err)
			input.ReadBytes('\n')
			return
		}
		var topMsg string
		var lastDay int
		for i := len(msgs) - 1; i >= 0; i-- {
			v := msgs[i]
			if v.Type == discordgo.MessageTypeThreadStarterMessage {
				v2, err := client.State.Message(v.MessageReference.ChannelID, v.MessageReference.MessageID)
				if err != nil {
					v2, _ = client.ChannelMessage(v.MessageReference.ChannelID, v.MessageReference.MessageID)
					if v2 == nil {
						continue
					}
				}
				v = v2
			}
			if nicks[v.Author.ID] == "" {
				nicks[v.Author.ID] = v.Author.DisplayName()
				if guild != nil {
					mem, err := client.State.Member(guild.ID, v.Author.ID)
					if err == nil && mem.Nick != "" {
						nicks[v.Author.ID] = mem.Nick
					}
				}
			}
			plaintext := true
			t := v.Timestamp.In(time.Local)
			if t.YearDay() != lastDay {
				output.WriteString(t.Format(dateFormat))
				lastDay = t.YearDay()
			}
			output.WriteString(t.Format(timeFormat))
			switch v.Type {
			case discordgo.MessageTypeDefault:
				fallthrough
			case discordgo.MessageTypeChatInputCommand:
				fallthrough
			case discordgo.MessageTypeContextMenuCommand:
				fallthrough
			case discordgo.MessageTypeReply:
				fmt.Fprint(output, " <", nicks[v.Author.ID], "> ")
				output.WriteString(v.Content)
				plaintext = false
			case discordgo.MessageTypeChannelPinnedMessage:
				fmt.Fprintf(output, "%s pinned a message to this channel", nicks[v.Author.ID])
			case discordgo.MessageTypeGuildMemberJoin:
				fmt.Fprintf(output, "%s joined the guild", nicks[v.Author.ID])
			case discordgo.MessageTypeUserPremiumGuildSubscription:
				fallthrough
			case discordgo.MessageTypeUserPremiumGuildSubscriptionTierOne:
				fallthrough
			case discordgo.MessageTypeUserPremiumGuildSubscriptionTierTwo:
				fallthrough
			case discordgo.MessageTypeUserPremiumGuildSubscriptionTierThree:
				fmt.Fprintf(output, "%s boosted the guild", nicks[v.Author.ID])
			case discordgo.MessageTypeThreadCreated:
				if v.MessageReference == nil {
					v.MessageReference = &discordgo.MessageReference{ChannelID: "unknown"}
				}
				fmt.Fprintf(output, "%s created thread %s (%s)", nicks[v.Author.ID], v.Content, v.MessageReference.ChannelID)
			}
			if !plaintext {
				if len(v.Embeds) != 0 {
					if v.Embeds[0].Image != nil {
						fmt.Fprintf(output, " (%s)", v.Embeds[0].Image.URL)
					} else {
						output.WriteString(" (Embed)")
					}
				}
				if len(v.Attachments) != 0 {
					fmt.Fprintf(output, " (%s)", v.Attachments[0].URL)
				}
				if v.Pinned {
					output.WriteString(" (Pinned)")
				}
				if v.Thread != nil && v.Thread.ID != channel.ID {
					fmt.Fprintf(output, " (Thread %s)", v.Thread.ID)
				} else if v.MessageReference != nil && v.MessageReference.MessageID == "" {
					fmt.Fprintf(output, " (Thread %s)", v.MessageReference.ChannelID)
				}
			}
			output.WriteByte('\n')
			if topMsg == "" {
				topMsg = v.ID
			}
		}
		fmt.Fprintf(output, "\nEnter nothing to refresh, /help for additional commands (Current time %s, page %d)\n", time.Now().Format(tsFormat), len(pager)+1)
		output.Flush()
		msg, err := input.ReadString('\n')
		if err != nil {
			return
		}
		msg = strings.Trim(msg, " \r\n\t")
		if len(msg) == 0 {
		} else if len(msg) > 2047 {
			fmt.Println("Message too long!")
			input.ReadBytes('\n')
		} else if msg[0] != '/' {
			msg = strings.ReplaceAll(msg, "\\n", "\n")
			_, err = client.ChannelMessageSend(channel.ID, msg)
			if err != nil {
				fmt.Println(err)
				input.ReadBytes('\n')
			}
		} else {
			cmd := strings.Split(msg, " ")
			cmd[0] = cmd[0][1:]
			switch cmd[0] {
			case "exit":
				return
			case "pageup":
				pager = append(pager, topMsg)
			case "pagedown":
				pager = pager[:len(pager)-1]
			case "delete":
				if len(cmd) == 1 {
					fmt.Println("Usage: /delete <message id>")
				} else {
					msg = strings.Join(cmd[1:], " ")
					err = client.ChannelMessageDelete(channel.ID, msg)
					if err != nil {
						fmt.Println(err)
					}
				}
				input.ReadBytes('\n')
			case "help":
				fmt.Println("/help - This message\n/exit - Quit this mode\n/pageup and /pagedown - Scroll\n/tar [first id] - Log some or all messages\n/chatlog - Tar alias\n/nick - Set nickname\n/typing - Send typing notif\n/file - Upload file\n/delete <id> - Delete a message\n/thread <id> - Enter a thread")
				input.ReadBytes('\n')
			case "tar":
				fallthrough
			case "chatlog":
				count := 0
				if len(cmd) > 1 {
					count, err = strconv.Atoi(cmd[1])
					if err != nil {
						fmt.Println("Not a number!")
						input.ReadBytes('\n')
						continue
					}
				}
				chatlog(channel, guild, count)
			case "zip":
				count := -1
				if len(cmd) > 1 {
					count, err = strconv.Atoi(cmd[1])
					if err != nil {
						fmt.Println("Not a number!")
						input.ReadBytes('\n')
						continue
					}
				}
				archive(channel, guild, count)
			case "nick":
				if len(cmd) == 1 {
					fmt.Println("Usage: /nick <new nickname>\nEnter nil to reset nick")
				} else {
					msg = strings.Join(cmd[1:], " ")
					if len(msg) > 32 {
						fmt.Println("Nickname too long!")
					} else {
						err = client.GuildMemberNickname(guild.ID, "@me", msg)
						if err != nil {
							fmt.Println(err)
						} else {
							fmt.Print("Nickname set successfully.")
						}
					}
				}
				input.ReadBytes('\n')
			case "file":
				if len(cmd) == 1 {
					fmt.Println("Usage: /file <filename>")
				} else {
					msg = strings.Join(cmd[1:], " ")
					f, err := os.Open(msg)
					if err != nil {
						fmt.Println(err)
					} else {
						info, _ := f.Stat()
						if !info.Mode().IsRegular() {
							fmt.Println("Cannot send irregular file!")
						} else {
							fmt.Println("File sending, please wait...")
							_, err = client.ChannelFileSend(channel.ID, path.Base(msg), f)
							f.Close()
							if err != nil {
								fmt.Println(err)
							} else {
								fmt.Println("File sent.")
							}
						}
					}
				}
				input.ReadBytes('\n')
			case "typing":
				err = client.ChannelTyping(channel.ID)
				if err != nil {
					fmt.Println(err)
					input.ReadBytes('\n')
				}
			case "reply":
				if len(cmd) == 1 {
					fmt.Println("Usage: /reply <id> <msg...>")
				} else {
					client.ChannelMessageSendReply(channel.ID, strings.Join(cmd[2:], " "), &discordgo.MessageReference{MessageID: cmd[1], ChannelID: channel.ID, GuildID: channel.GuildID})
				}
			case "thread":
				if len(cmd) == 1 {
					fmt.Println("Usage: /thread <thread id>")
				} else {
					ch2, err := client.Channel(cmd[1])
					if err != nil {
						fmt.Println(err)
						input.ReadBytes('\n')
					} else {
						chatter(ch2, guild)
					}
				}
			default:
				fmt.Print("\a")
			}
		}
	}
}
