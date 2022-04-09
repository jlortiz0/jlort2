module jlortiz.org/jlort2/modules/music

go 1.16

replace jlortiz.org/jlort2/modules/commands => ../commands

replace jlortiz.org/jlort2/modules/log => ../log

require (
	github.com/bwmarrin/discordgo v0.23.2
	jlortiz.org/jlort2/modules/commands v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/log v0.0.0-00010101000000-000000000000
)
