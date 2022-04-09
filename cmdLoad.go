package main

import (
	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/brit"
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
	commands.RegisterCommand(vachan, "vachan")
}

func cleanup(self *discordgo.Session) {
	commands.Cleanup(self)
	quotes.Cleanup(self)
	kek.Cleanup(self)
	zip.Cleanup(self)
	music.Cleanup(self)
	brit.Cleanup(self)
	if dirty {
		commands.SavePersistent("vachan", &voiceAnnounce)
	}
}
