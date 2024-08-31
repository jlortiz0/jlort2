module jlortiz.org/jlort2

go 1.22

require (
	github.com/bwmarrin/discordgo v0.28.1
	github.com/mattn/go-isatty v0.0.20
	github.com/mattn/go-sqlite3 v1.14.22
	jlortiz.org/jlort2/modules/commands v0.0.0
	jlortiz.org/jlort2/modules/clickart v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/kek v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/log v0.0.0
	jlortiz.org/jlort2/modules/music v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/quotes v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/reminder v0.0.0-00010101000000-000000000000
	jlortiz.org/jlort2/modules/zip v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
)

replace jlortiz.org/jlort2/modules/log => ./modules/log

replace jlortiz.org/jlort2/modules/quotes => ./modules/quotes

replace jlortiz.org/jlort2/modules/zip => ./modules/zip

replace jlortiz.org/jlort2/modules/kek => ./modules/kek

replace jlortiz.org/jlort2/modules/music => ./modules/music

replace jlortiz.org/jlort2/modules/reminder => ./modules/reminder

replace jlortiz.org/jlort2/modules/clickart => ./modules/clickart

replace jlortiz.org/jlort2/modules/commands => ./modules/commands
