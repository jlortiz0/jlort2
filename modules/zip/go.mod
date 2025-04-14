module jlortiz.org/jlort2/modules/zip

go 1.22.6
toolchain go1.24.1

replace jlortiz.org/jlort2/modules/commands => ../commands

replace jlortiz.org/jlort2/modules/log => ../log

require (
	github.com/bwmarrin/discordgo v0.28.1
	jlortiz.org/jlort2/modules/commands v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	jlortiz.org/jlort2/modules/log v0.0.0-00010101000000-000000000000 // indirect
)
