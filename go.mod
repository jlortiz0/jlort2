module jlortiz.org/jlort2

go 1.16

require (
	github.com/bwmarrin/discordgo v0.23.2
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/mattn/go-isatty v0.0.12
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	jlortiz.org/jlort2/modules/commands v0.0.0
	jlortiz.org/jlort2/modules/brit v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/kek v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/log v0.0.0
	jlortiz.org/jlort2/modules/music v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/quotes v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/zip v0.0.0-00010101000000-000000000000
)

replace jlortiz.org/jlort2/modules/log => ./modules/log

replace jlortiz.org/jlort2/modules/brit => ./modules/brit

replace jlortiz.org/jlort2/modules/quotes => ./modules/quotes

replace jlortiz.org/jlort2/modules/zip => ./modules/zip

replace jlortiz.org/jlort2/modules/kek => ./modules/kek

replace jlortiz.org/jlort2/modules/music => ./modules/music

replace jlortiz.org/jlort2/modules/commands => ./modules/commands
