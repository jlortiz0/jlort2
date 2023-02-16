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
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
)

var aliases map[string]map[string]string
var dirty bool
var aliasLock *sync.RWMutex = new(sync.RWMutex)

// ~!song <song alias>
// @GuildOnly
// Plays a song alias
// To get a list of aliases, do ~!song list
// To register or unregister an alias, use ~!addsong and ~!delsong
func song(ctx commands.Context) error {
	name := ctx.ApplicationCommandData().Options[0].StringValue()
	aliasLock.RLock()
	mappings := aliases[ctx.GuildID]
	if len(mappings) == 0 {
		aliasLock.RUnlock()
		return ctx.RespondPrivate("No song aliases have been set. Use /addsong to set a song alias.")
	}
	if name == "list" {
		names := make([]string, len(mappings)+1)
		names[0] = "Songs:"
		i := 1
		for k := range mappings {
			names[i] = k
			i++
		}
		aliasLock.RUnlock()
		return ctx.RespondPrivate(strings.Join(names, "\n"))
	}
	url := mappings[name]
	aliasLock.RUnlock()
	if url == "" {
		return ctx.RespondPrivate("No song by that name. For a list, use /song list")
	}
	ctx.ApplicationCommandData().Options[0].Value = url
	return play(ctx)
}

// ~!addsong <alias> <Youtube URL>
// @GuildOnly
// Registers a song alias
// The Youtube URL can be any URL supported by ~!play, but it cannot be a Youtube search.
func addsong(ctx commands.Context) error {
	args := ctx.ApplicationCommandData().Options
	name := args[0].StringValue()
	if name == "list" || name == "all" {
		return ctx.RespondPrivate("This song name is not allowed.")
	}
	ctx.RespondDelayed(true)
	url := args[1].StringValue()
	var info YDLInfo
	out, err := exec.Command("yt-dlp", "-f", "bestaudio/best", "-J", url).Output()
	if err != nil {
		if err2, ok := err.(*exec.ExitError); ok {
			return ctx.RespondEdit(fmt.Sprintf("Failed to run subprocess: %s\n%s", err2.Error(), string(err2.Stderr)))
		}
		return fmt.Errorf("failed to run subprocess: %w", err)
	}
	err = json.Unmarshal(out, &info)
	if err != nil {
		ctx.RespondEdit("Could not get info from this URL. Note that /song does not support searches.")
		return err
	}
	if info.Extractor == "Generic" {
		return ctx.RespondEdit("/song does not support direct links to files.")
	}
	if info.URL == "" {
		return ctx.RespondEdit("Could not get info from this URL.")
	}
	aliasLock.Lock()
	mappings := aliases[ctx.GuildID]
	if mappings == nil {
		aliases[ctx.GuildID] = make(map[string]string)
		mappings = aliases[ctx.GuildID]
	}
	mappings[name] = url
	aliasLock.Unlock()
	dirty = true
	return ctx.RespondEdit(fmt.Sprintf("Set song alias %s", name))
}

// ~!delsong <alias>
// @Alias rmsong
// @Alias removesong
// @GuildOnly
// Unregisters a song alias
// Do ~!delsong all to remove all songs. To remove all songs, you must have the Manage Messages permission.
func delsong(ctx commands.Context) error {
	name := ctx.ApplicationCommandData().Options[0].StringValue()
	aliasLock.Lock()
	defer aliasLock.Unlock()
	mappings := aliases[ctx.GuildID]
	if name == "all" {
		perms, err := ctx.State.UserChannelPermissions(ctx.User.ID, ctx.ChannelID)
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		if perms&discordgo.PermissionManageServer == 0 {
			return ctx.RespondPrivate("You need the Manage Server permission to clear all aliases.")
		}
		delete(aliases, ctx.GuildID)
		return ctx.RespondPrivate("All aliases deleted.")
	}
	delete(mappings, name)
	dirty = true
	return ctx.RespondPrivate("Alias deleted.")
}

func delGuildSongs(_ *discordgo.Session, event *discordgo.GuildDelete) {
	aliasLock.Lock()
	delete(aliases, event.ID)
	aliasLock.Unlock()
	djLock.Lock()
	delete(djRoles, event.ID)
	djLock.Unlock()
	dirty = true
	v := streams[event.ID]
	if v != nil && v.Len() != 0 {
		obj := v.Head().Value
		obj.Stop <- struct{}{}
	}
	delete(streams, event.ID)
}
