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
	"jlortiz.org/jlort2/modules/brit"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/gacha"
	"jlortiz.org/jlort2/modules/kek"
	"jlortiz.org/jlort2/modules/log"
	"jlortiz.org/jlort2/modules/music"
	"jlortiz.org/jlort2/modules/quotes"
	"jlortiz.org/jlort2/modules/zip"
)

func initModules(self *discordgo.Session) {
	commands.Init(self)
	log.Info("Loaded commands")
	quotes.Init(self)
	log.Info("Loaded quotes")
	kek.Init(self)
	log.Info("Loaded kek")
	zip.Init(self)
	log.Info("Loaded zip")
	brit.Init(self)
	log.Info("Loaded brit")
	music.Init(self)
	log.Info("Loaded music")
	gacha.Init(self)
	log.Info("Loaded gacha")
	voiceStatement, _ = commands.GetDatabase().Prepare("SELECT cid FROM vachan WHERE gid=?;")
	commands.RegisterCommand(vachan, "vachan")
}

func cleanup(self *discordgo.Session) {
	voiceStatement.Close()
	gacha.Cleanup(self)
	music.Cleanup(self)
	brit.Cleanup(self)
	zip.Cleanup(self)
	kek.Cleanup(self)
	quotes.Cleanup(self)
	commands.Cleanup(self)
}
