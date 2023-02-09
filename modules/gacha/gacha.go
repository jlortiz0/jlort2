package gacha

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

type TradeData struct {
	From      string
	To        string
	Giving    int
	GiveCount uint16
	Getting   int
	GetCount  uint16
	Expire    time.Time
}

var gachaItems [][4]string
var gachaShortNames map[string]int
var trades map[int]*TradeData
var tradeLock *sync.RWMutex = new(sync.RWMutex)
var queryGetData *sql.Stmt
var queryGetItem *sql.Stmt

// ~!pull [use token]
// Pull on the general banner.
// If you have already pulled today, pulling again will cost 3 reroll tokens.
// You must confirm the spending of reroll tokens with ~!pull yes
func pull(ctx commands.Context, args []string) error {
	uid, _ := strconv.ParseUint(ctx.Author.ID, 10, 64)
	result := queryGetData.QueryRow(uid)
	var tokens uint
	var nextPull time.Time
	if result.Scan(&tokens, &nextPull) != nil {
		if len(args) != 0 {
			return ctx.Send("You don't have enough tokens.")
		}
	} else if len(args) == 0 && time.Now().Before(nextPull) {
		diff := time.Until(nextPull)
		if diff > time.Hour {
			return ctx.Send(fmt.Sprintf("Wait %d hours or use tokens to pull again.", diff/time.Hour))
		} else if diff > time.Minute {
			return ctx.Send(fmt.Sprintf("Wait %d minutes or use tokens to pull again.", diff/time.Minute))
		} else {
			return ctx.Send("Wait a minute or two to pull again.")
		}
	} else if len(args) != 0 && tokens < 3 {
		return ctx.Send("You don't have enough tokens.")
	}
	choice := rand.Intn(len(gachaItems))
	embed := makeItemEmbed(choice)
	if time.Now().Before(nextPull) {
		tokens -= 3
	} else {
		nextPull = time.Now().Add(24 * time.Hour)
	}
	ctx.Database.Exec(`
	INSERT OR REPLACE INTO gachaPlayer (uid, tokens, nextPull) VALUES (?001, ?002, ?003);
	INSERT INTO gachaItems (uid, itemId, count) VALUES (?001, ?004, 1)
	ON CONFLICT DO SET count = count + 1;
	`, uid, tokens, nextPull, choice)
	_, err := ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
	return err
}

// ~!relics list [page] [user] or ~!relics sell <short name> [count] or ~!relics info <short name>
// Get info about the relics you currently have.
// You can see another user's relics with ~!relics list 1 <user>
// By default, ~!relic sell will sell one relic. Selling gives you reroll tokens.
// The "short names" of relics are displayed in parenthesis on the list, and will never contain spaces.
func relics(ctx commands.Context, args []string) error {
	if len(args) == 0 {
		return ctx.Send("Usage: ~!relics list [page] or ~!relics sell <short name> [count] or ~!relics info <short name>")
	}
	if args[0] == "list" {
		page := 1
		if len(args) > 1 {
			var err error
			page, err = strconv.Atoi(args[1])
			if err != nil {
				return ctx.Send("Unable to parse page")
			}
		}
		uid, _ := strconv.ParseUint(ctx.Author.ID, 10, 64)
		var total int
		tx, err := ctx.Database.Begin()
		if err != nil {
			return err
		}
		defer tx.Commit()
		row := tx.QueryRow("SELECT COUNT(*) FROM gachaItems WHERE uid=?001;", uid)
		row.Scan(&total)
		results, err := tx.Query("SELECT itemId, count FROM gachaItems WHERE uid=?001 ORDER BY itemId LIMIT 100 OFFSET ?002 * 20;", uid, page-1)
		if err != nil {
			return err
		}
		if !results.Next() {
			if (total+19)/20 < page {
				return ctx.Send("Page number is too big.")
			}
			return ctx.Send("You don't have any relics.")
		}
		i := 0
		output := new(strings.Builder)
		var itemId, count uint
		for i < 20 {
			i++
			results.Scan(&itemId, &count)
			if count != 0 {
				output.WriteString(fmt.Sprintf("%s (%s) x%d\n", gachaItems[itemId][0], gachaItems[itemId][3], count))
			}
			if !results.Next() {
				break
			}
		}
		results.Close()
		embed := new(discordgo.MessageEmbed)
		embed.Title = fmt.Sprintf("%s's Relics (Page %d of %d)", ctx.Author.Username, page, (total+19)/20)
		embed.Description = output.String()
		embed.Footer = new(discordgo.MessageEmbedFooter)
		result := queryGetData.QueryRow(uid)
		var tokens uint
		result.Scan(&tokens, &sql.NullTime{})
		embed.Footer.Text = fmt.Sprintf("You have %d tokens", tokens)
		_, err = ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
		return err
	} else if args[0] == "sell" {
		if len(args) == 1 {
			return ctx.Send("Usage: ~!relics sell <short name> [count]")
		}
		id, ok := gachaShortNames[args[1]]
		if !ok {
			return ctx.Send("No such relic " + args[1])
		}
		count := uint64(1)
		if len(args) > 2 {
			var err error
			count, err = strconv.ParseUint(args[2], 10, 16)
			if err != nil {
				return ctx.Send("Unable to parse count")
			}
		}
		uid, _ := strconv.ParseUint(ctx.Author.ID, 10, 64)
		results := queryGetItem.QueryRow(uid, id)
		var total uint
		if results.Scan(&total) != nil || uint(count) > total {
			return ctx.Send("You are trying to sell more than you have.")
		}
		tx, err := ctx.Database.Begin()
		if err != nil {
			return err
		}
		defer tx.Commit()
		tx.Exec("UPDATE gachaItems SET count = count - ?003 WHERE uid=?001 AND itemID=?002;", uid, id, count)
		tx.Exec("UPDATE gachaPlayer SET tokens = tokens + ?002 WHERE uid=?001;", uid, count)
		return ctx.Send(fmt.Sprintf("Sold %d of %s and recieved %d tokens.", count, gachaItems[id][0], count))
	} else if args[0] == "info" || args[0] == "show" {
		if len(args) < 2 {
			return ctx.Send("Usage: ~!relics info <short name>")
		}
		id, ok := gachaShortNames[args[1]]
		if !ok {
			return ctx.Send("No such relic " + args[1])
		}
		uid, _ := strconv.ParseUint(ctx.Author.ID, 10, 64)
		results := queryGetItem.QueryRow(uid, id)
		var total uint
		results.Scan(&total)
		if total == 0 {
			return ctx.Send("You don't have this relic.")
		}
		_, err := ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, makeItemEmbed(id))
		return err
	} else {
		return ctx.Send("Usage: ~!relics list [page] [user]\n    or ~!relics sell <id> [count]\n    or ~!relics info <id>")
	}
}

// ~!trade create <name to give> <count> <user> <name to take> <count>
// Offers to trade relics or tokens with another user. You will be given a code which can be used to accept the trade.
// Use "token" as the name to give or the name to take to trade reroll tokens.
// ~!trade info <code> will show info about the trade.
// ~!trade accept <code> or ~!trade reject <code> will end a trade.
// Trades expire after some time, so you should ensure the other person is online. Maybe even @ them.
func trade(ctx commands.Context, args []string) error {
	if ctx.GuildID == "" {
		return ctx.Send("This command can only be used in servers.")
	}
	if len(args) < 2 {
		return ctx.Send("Usage: ~!trade create <name to give> <count> <to> <name to take> <count>")
	}
	switch args[0] {
	case "create":
		if len(args) < 6 {
			return ctx.Send("Usage: ~!trade create <name to give> <count> <to> <name to take> <count>")
		}
		trade := new(TradeData)
		trade.From = ctx.Author.ID
		target, err := commands.FindMember(ctx.Bot, args[3], ctx.GuildID)
		if err != nil {
			return ctx.Send("No such member " + args[3])
		}
		trade.To = target.User.ID
		trade.Expire = time.Now().Add(5 * time.Minute)
		var ok bool
		if args[1] == "token" {
			trade.Giving = -1
		} else {
			trade.Giving, ok = gachaShortNames[args[1]]
			if !ok {
				return ctx.Send("No such relic " + args[1])
			}
		}
		if args[4] == "token" {
			trade.Getting = -1
		} else {
			trade.Getting, ok = gachaShortNames[args[4]]
			if !ok {
				return ctx.Send("No such relic " + args[4])
			}
		}
		gc, err := strconv.ParseUint(args[2], 10, 16)
		if err != nil {
			return ctx.Send("Unable to parse give count")
		}
		trade.GiveCount = uint16(gc)
		gc, err = strconv.ParseUint(args[5], 10, 16)
		if err != nil {
			return ctx.Send("Unable to parse get count")
		}
		trade.GetCount = uint16(gc)

		fromId, _ := strconv.ParseUint(trade.From, 10, 64)
		toId, _ := strconv.ParseUint(trade.To, 10, 64)
		var temp uint16
		if trade.Giving >= 0 {
			result := queryGetItem.QueryRow(fromId, trade.Giving)
			result.Scan(&temp)
			if temp < trade.GiveCount {
				return ctx.Send("Trade sender does not have enough relics.")
			}
		} else {
			result := queryGetData.QueryRow(fromId)
			result.Scan(&temp, &sql.NullTime{})
			if temp < trade.GiveCount {
				return ctx.Send("Trade sender does not have enough tokens.")
			}
		}
		if trade.Getting >= 0 {
			result := queryGetItem.QueryRow(toId, trade.Giving)
			result.Scan(&temp)
			if temp < trade.GetCount {
				return ctx.Send("Trade recipient does not have enough relics.")
			}
		} else {
			result := queryGetData.QueryRow(toId)
			result.Scan(&temp, &sql.NullTime{})
			if temp < trade.GetCount {
				return ctx.Send("Trade recipient does not have enough tokens.")
			}
		}
		tradeLock.Lock()
		tcode := rand.Intn(9999)
		for trades[tcode] != nil && time.Now().Before(trades[tcode].Expire) {
			tcode = rand.Intn(9999)
		}
		trades[tcode] = trade
		tradeLock.Unlock()
		return ctx.Send(fmt.Sprintf("Trade created. <@%s>, accept the trade with ~!trade accept %04d\nEither of you can cancel the trade with ~!trade reject %04d", trade.To, tcode, tcode))
	case "info":
		tcode, err := strconv.Atoi(args[1])
		if err != nil {
			return ctx.Send("Unable to parse code")
		}
		tradeLock.RLock()
		trade := trades[tcode]
		tradeLock.RUnlock()
		if trade == nil {
			return ctx.Send("Invalid code")
		}
		if ctx.Author.ID != trade.To && ctx.Author.ID != trade.From {
			return ctx.Send("You are not a participant in this trade")
		}
		embed := new(discordgo.MessageEmbed)
		embed.Title = "Trade offer"
		embed.Fields = make([]*discordgo.MessageEmbedField, 2)
		embed.Fields[0] = new(discordgo.MessageEmbedField)
		embed.Fields[0].Name = "Offering"
		if trade.Giving == -1 {
			embed.Fields[0].Value = fmt.Sprintf("Token x%d", trade.GiveCount)
		} else {
			embed.Fields[0].Value = fmt.Sprintf("%s x%d", gachaItems[trade.Giving][0], trade.GiveCount)
		}
		embed.Fields[0].Inline = true
		embed.Fields[1] = new(discordgo.MessageEmbedField)
		embed.Fields[1].Name = "For"
		if trade.Getting == -1 {
			embed.Fields[1].Value = fmt.Sprintf("Token x%d", trade.GetCount)
		} else {
			embed.Fields[1].Value = fmt.Sprintf("%s x%d", gachaItems[trade.Getting][0], trade.GetCount)
		}
		embed.Fields[1].Inline = true
		_, err = ctx.Bot.ChannelMessageSendEmbed(ctx.ChanID, embed)
		return err
	case "accept":
		fallthrough
	case "reject":
		tcode, err := strconv.Atoi(args[1])
		if err != nil {
			return ctx.Send("Unable to parse code")
		}
		tradeLock.RLock()
		trade := trades[tcode]
		tradeLock.RUnlock()
		if trade == nil {
			return ctx.Send("Invalid code")
		}
		if ctx.Author.ID != trade.To && ctx.Author.ID != trade.From {
			return ctx.Send("You are not a participant in this trade")
		}
		if args[0] == "accept" && ctx.Author.ID == trade.From {
			return ctx.Send("Sender cannot force accept trade.")
		}
		tradeLock.Lock()
		trades[tcode] = nil
		tradeLock.Unlock()
		if time.Now().After(trade.Expire) {
			return ctx.Send("Trade expired")
		}
		if args[0] == "reject" {
			return ctx.Send("Trade rejected/cancelled.")
		}
		tx, err := ctx.Database.Begin()
		if err != nil {
			return err
		}
		defer tx.Commit()
		fromId, _ := strconv.ParseUint(trade.From, 10, 64)
		toId, _ := strconv.ParseUint(trade.To, 10, 64)
		var temp uint16
		if trade.Giving >= 0 {
			result := tx.Stmt(queryGetItem).QueryRow(fromId, trade.Giving)
			result.Scan(&temp)
			if temp < trade.GiveCount {
				return ctx.Send("Trade sender no longer has enough relics.")
			}
		} else {
			result := tx.Stmt(queryGetData).QueryRow(fromId)
			result.Scan(&temp, &sql.NullTime{})
			if temp < trade.GiveCount {
				return ctx.Send("Trade sender no longer has enough tokens.")
			}
		}
		if trade.Getting >= 0 {
			result := tx.Stmt(queryGetItem).QueryRow(toId, trade.Giving)
			result.Scan(&temp)
			if temp < trade.GetCount {
				return ctx.Send("Trade recipient no longer has enough relics.")
			}
		} else {
			result := tx.Stmt(queryGetData).QueryRow(toId)
			result.Scan(&temp, &sql.NullTime{})
			if temp < trade.GetCount {
				return ctx.Send("Trade recipient no longer has enough tokens.")
			}
		}
		moveItem1, _ := tx.Prepare("UPDATE gachaItems SET count = count - ?003 WHERE uid=?001 AND itemId=?002;")
		moveItem2, _ := tx.Prepare(`INSERT INTO gachaItems (uid, itemId, count) VALUES (?001, ?002, ?003)
		ON CONFLICT DO UPDATE SET count = count + ?003;`)
		moveToken1, _ := tx.Prepare("UPDATE gachaPlayer SET tokens = tokens - ?002 WHERE uid=?001;")
		moveToken2, _ := tx.Prepare(`INSERT INTO gachaPlayer (uid, tokens) VALUES (?001, ?002)
		ON CONFLICT DO UPDATE SET tokens = tokens + ?002;`)
		if trade.Giving >= 0 {
			moveItem1.Exec(fromId, trade.Giving, trade.GiveCount)
			moveItem2.Exec(toId, trade.Giving, trade.GiveCount)
		} else {
			moveToken1.Exec(fromId, trade.GiveCount)
			moveToken2.Exec(toId, trade.GiveCount)
		}
		if trade.Getting >= 0 {
			moveItem1.Exec(toId, trade.Giving, trade.GiveCount)
			moveItem2.Exec(fromId, trade.Giving, trade.GiveCount)
		} else {
			moveToken1.Exec(toId, trade.GiveCount)
			moveToken2.Exec(fromId, trade.GiveCount)
		}
		moveItem1.Close()
		moveItem2.Close()
		moveToken1.Close()
		moveToken2.Close()
		return ctx.Send("Trade successful.")
	default:
		return ctx.Send("Usage: ~!trade create <id to give> <count> <to> <id to take> <count>") // \n    or ~!trade gift <id to give> <count> <to>")
	}
}

func makeItemEmbed(id int) *discordgo.MessageEmbed {
	embed := new(discordgo.MessageEmbed)
	embed.Title = gachaItems[id][0]
	embed.Description = gachaItems[id][1]
	embed.Image = new(discordgo.MessageEmbedImage)
	embed.Image.URL = gachaItems[id][2]
	embed.Footer = new(discordgo.MessageEmbedFooter)
	embed.Footer.Text = gachaItems[id][3]
	return embed
}

// Init is defined in the command interface to initalize a module. This includes registering commands, making structures, and loading persistent data.
// Here, it also loads the gacha data into memory, including user data and the item schema.
func Init(self *discordgo.Session) {
	data, err := os.ReadFile("modules/gacha/relics.json")
	if err != nil {
		log.Error(err)
		return
	}
	err = json.Unmarshal(data, &gachaItems)
	if err != nil {
		log.Error(err)
		return
	}
	trades = make(map[int]*TradeData)
	gachaShortNames = make(map[string]int)
	for k, v := range gachaItems {
		gachaShortNames[v[3]] = k
	}
	if len(gachaShortNames) != len(gachaItems) {
		log.Warn("gacha: duplicate short name!")
	}

	commands.RegisterCommand(pull, "pull")
	commands.RegisterCommand(relics, "relics")
	commands.RegisterCommand(relics, "relic")
	commands.RegisterCommand(trade, "trade")
	db := commands.GetDatabase()
	queryGetData, err = db.Prepare("SELECT tokens, nextPull FROM gachaPlayer WHERE uid=?001;")
	if err != nil {
		log.Error(err)
		return
	}
	queryGetItem, err = db.Prepare("SELECT count FROM gachaItems WHERE uid=?001 AND itemId=?002;")
	if err != nil {
		log.Error(err)
	}
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the kek data to disk.
func Cleanup(_ *discordgo.Session) {
	commands.GetDatabase().Exec("DELETE FROM gachaItems WHERE count=0;")
	queryGetData.Close()
	queryGetItem.Close()
}
