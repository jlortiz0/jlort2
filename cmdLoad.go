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

func initModules(self *discordgo.Session) {
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
	commands.RegisterCommand(vachan, "vachan")
	commands.RegisterSaver(saveVoice)
}

func cleanup(self *discordgo.Session) {
	commands.Cleanup(self)
	quotes.Cleanup(self)
	kek.Cleanup(self)
	zip.Cleanup(self)
	music.Cleanup(self)
	saveVoice()
}
