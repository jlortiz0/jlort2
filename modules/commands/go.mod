module jlortiz.org/jlort2/modules/commands

go 1.22

replace jlortiz.org/jlort2/modules/log => ../log

require (
	github.com/bwmarrin/discordgo v0.28.1
	github.com/mattn/go-sqlite3 v1.14.22
	jlortiz.org/jlort2/modules/log v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
)
