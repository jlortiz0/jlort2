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
	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/kek"
	"jlortiz.org/jlort2/modules/log"
	"jlortiz.org/jlort2/modules/music"
	"jlortiz.org/jlort2/modules/quotes"
	"jlortiz.org/jlort2/modules/zip"
)

func initModules(self *discordgo.Session, guildId string) {
	commands.Init(self)
	log.Info("Loaded commands")
	quotes.Init(self)
	log.Info("Loaded quotes")
	kek.Init(self)
	log.Info("Loaded kek")
	zip.Init(self)
	log.Info("Loaded zip")
	music.Init(self)
	log.Info("Loaded music")
	voiceStatement, _ = commands.GetDatabase().Prepare("SELECT cid FROM vachan WHERE gid=?;")
	commands.PrepareCommand("vachan", "Change voice join announcer").Guild().Perms(discordgo.PermissionManageServer).Register(vachan, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("channel", "Voice join announcements will be posted here").AsChannel().Required().Finalize(),
	})
	testMode := len(guildId) > 0 && guildId[0] == 't'
	if testMode {
		guildId = guildId[1:]
	}
	commands.UploadCommands(self, self.State.Application.ID, guildId, testMode)
}

func cleanup(self *discordgo.Session) {
	voiceStatement.Close()
	music.Cleanup(self)
	zip.Cleanup(self)
	kek.Cleanup(self)
	quotes.Cleanup(self)
	commands.Cleanup(self)
}
