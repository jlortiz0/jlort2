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
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
)

// ~!song <song alias>
// @GuildOnly
// Plays a song alias
// To get a list of aliases, do ~!song list
// To register or unregister an alias, use ~!addsong and ~!delsong
func song(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!song <song alias>\nFor a list of songs, use ~!song list")
	}
	name := args[0]
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	if name == "list" {
		names := new(strings.Builder)
		names.WriteString("Songs:\n")
		results, err := ctx.Database.Query("SELECT key FROM songAlias WHERE gid=?001;", gid)
		if err != nil {
			return err
		}
		var s string
		for results.Next() {
			results.Scan(&s)
			names.WriteString(s)
			names.WriteByte('\n')
		}
		if s == "" {
			names.Reset()
			names.WriteString("No song aliases have been set. Use ~!addsong to set a song alias.")
		}
		return ctx.Send(names.String())
	}
	result := ctx.Database.QueryRow("SELECT value FROM songAlias WHERE gid=?001 AND key=?002;", gid, name)
	var url string
	if result.Scan(&url) != nil {
		return ctx.Send("No song by that name. For a list, use ~!song list")
	}
	return play(ctx, []string{url})
}

// ~!addsong <alias> <Youtube URL>
// @GuildOnly
// Registers a song alias
// The Youtube URL can be any URL supported by ~!play, but it cannot be a Youtube search.
func addsong(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) < 2 {
		return ctx.Send("Usage: ~!addsong <song alias> <url>")
	}
	name := args[0]
	if name == "list" || name == "all" {
		return ctx.Send("This song name is not allowed.")
	}
	ctx.Bot.ChannelTyping(ctx.ChanID)
	url := strings.Join(args[1:], " ")
	var info YDLInfo
	out, err := exec.Command("yt-dlp", "-f", "bestaudio/best", "-J", url).Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return ctx.Send("Could not get info from this URL. Note that ~!song does not support searches.")
		}
		return fmt.Errorf("Failed to run subprocess: %w", err)
	}
	err = json.Unmarshal(out, &info)
	if err != nil {
		ctx.Send("Could not get info from this URL. Note that ~!song does not support searches.")
		return err
	}
	if info.Extractor == "Generic" {
		return ctx.Send("~!song does not support direct links to files.")
	}
	if info.URL == "" {
		return ctx.Send("Could not get info from this URL.")
	}
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	ctx.Database.Exec("INSERT OR REPLACE INTO songAlias (gid, key, value) VALUES (?001, ?002, ?003);", gid, name, url)
	return ctx.Send(fmt.Sprintf("Set song alias %s", name))
}

// ~!delsong <alias>
// @Alias rmsong
// @Alias removesong
// @GuildOnly
// Unregisters a song alias
// Do ~!delsong all to remove all songs. To remove all songs, you must have the Manage Messages permission.
func delsong(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command only works in servers.")
	}
	if len(args) == 0 {
		return ctx.Send("Usage: ~!delsong <song alias>")
	}
	if args[0] == "all" {
		perms, err := ctx.State.MessagePermissions(ctx.Message)
		if err != nil {
			return fmt.Errorf("Failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageServer == 0 {
			return ctx.Send("You need the Manage Server permission to clear all aliases.")
		}
		gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
		ctx.Database.Exec("DELETE FROM songAlias WHERE gid = ?001;", gid)
		return ctx.Send("All aliases deleted.")
	}
	gid, _ := strconv.ParseUint(ctx.GuildID, 10, 64)
	ctx.Database.Exec("DELETE FROM songAlias WHERE gid = ?001 AND key = ?002;", gid, args[0])
	return ctx.Send("Alias deleted.")
}

func delGuildSongs(_ *discordgo.Session, event *discordgo.GuildDelete) {
	gid, _ := strconv.ParseUint(event.ID, 10, 64)
	commands.GetDatabase().Exec("DELETE FROM djRole WHERE gid = ?001;", gid)
	commands.GetDatabase().Exec("DELETE FROM songAlias WHERE gid = ?001;", gid)
	v := streams[event.ID]
	if v != nil && v.Len() != 0 {
		obj := v.Head().Value
		obj.Stop <- struct{}{}
	}
	delete(streams, event.ID)
}
