package gacha

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

type UserData struct {
	Items  map[int]uint16
	Tokens uint16
	Wait   time.Time
}
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
var gachaData map[string]*UserData
var trades map[int]*TradeData
var dirty bool
var gachaLock *sync.RWMutex = new(sync.RWMutex)
var tradeLock *sync.RWMutex = new(sync.RWMutex)

// ~!pull [use token]
// Pull on the general banner.
// If you have already pulled today, pulling again will cost 3 reroll tokens.
// You must confirm the spending of reroll tokens with ~!pull yes
func pull(ctx commands.Context) error {
	gachaLock.RLock()
	data := gachaData[ctx.User.ID]
	gachaLock.RUnlock()
	useToken := false
	if len(ctx.ApplicationCommandData().Options) > 0 {
		useToken = ctx.ApplicationCommandData().Options[0].BoolValue()
	}
	if data == nil {
		if useToken {
			return ctx.RespondPrivate("You don't have enough tokens.")
		}
		data = new(UserData)
		data.Items = make(map[int]uint16)
		// DEBUG CODE
		// data.Tokens = 999
	} else if !useToken && time.Now().Before(data.Wait) {
		diff := time.Until(data.Wait)
		if diff > time.Hour {
			return ctx.RespondPrivate(fmt.Sprintf("Wait %d hours or use tokens to pull again.", diff/time.Hour))
		} else if diff > time.Minute {
			return ctx.RespondPrivate(fmt.Sprintf("Wait %d minutes or use tokens to pull again.", diff/time.Minute))
		} else {
			return ctx.RespondPrivate("Wait a minute or two to pull again.")
		}
	} else if useToken && data.Tokens < 3 {
		return ctx.RespondPrivate("You don't have enough tokens.")
	}
	choice := rand.Intn(len(gachaItems))
	embed := makeItemEmbed(choice)
	gachaLock.Lock()
	data.Items[choice] += 1
	dirty = true
	if data.Wait.IsZero() {
		gachaData[ctx.User.ID] = data
	}
	if time.Now().Before(data.Wait) {
		data.Tokens -= 3
	} else {
		data.Wait = time.Now().Add(24 * time.Hour)
	}
	gachaLock.Unlock()
	return ctx.RespondEmbed(embed, true)
}

// ~!relics list [page] [user] or ~!relics sell <short name> [count] or ~!relics info <short name>
// Get info about the relics you currently have.
// You can see another user's relics with ~!relics list 1 <user>
// By default, ~!relic sell will sell one relic. Selling gives you reroll tokens.
// The "short names" of relics are displayed in parenthesis on the list, and will never contain spaces.
func relics(ctx commands.Context) error {
	op := ctx.ApplicationCommandData().Options[0].StringValue()
	args := ctx.ApplicationCommandData().Options[0].Options
	if op == "list" {
		page := 1
		if len(args) > 0 {
			page = int(args[1].IntValue())
		}
		gachaLock.RLock()
		data := gachaData[ctx.User.ID]
		if data == nil || len(data.Items) == 0 {
			gachaLock.RUnlock()
			return ctx.RespondPrivate("You don't have any relics.")
		}
		if page > 1 || (len(data.Items)+19)/20 < page {
			gachaLock.RUnlock()
			return ctx.RespondPrivate(fmt.Sprintf("Page number out of range, expected 1-%d.", (len(data.Items)+19)/20))
		}
		i := 0
		output := new(strings.Builder)
		for k, v := range data.Items {
			if v == 0 {
				continue
			}
			i++
			if i <= (page-1)*20 {
				continue
			}
			if i > page*20 {
				break
			}
			output.WriteString(fmt.Sprintf("%s (%s) x%d\n", gachaItems[k][0], gachaItems[k][3], v))
		}
		gachaLock.RUnlock()
		embed := new(discordgo.MessageEmbed)
		embed.Title = fmt.Sprintf("%s's Relics (Page %d of %d)", ctx.User.Username, page, (len(data.Items)+19)/20)
		embed.Description = output.String()
		embed.Footer = new(discordgo.MessageEmbedFooter)
		embed.Footer.Text = fmt.Sprintf("You have %d tokens", data.Tokens)
		return ctx.RespondEmbed(embed, false)
	} else if op == "sell" {
		name := args[0].StringValue()
		id, ok := gachaShortNames[name]
		if !ok {
			return ctx.RespondPrivate("No such relic " + name)
		}
		count := int64(1)
		if len(args) > 1 {
			count = args[1].IntValue()
		}
		gachaLock.RLock()
		data := gachaData[ctx.User.ID]
		if data == nil || uint16(count) > data.Items[id] {
			gachaLock.RUnlock()
			return ctx.RespondPrivate("You are trying to sell more than you have.")
		}
		gachaLock.RUnlock()
		gachaLock.Lock()
		data.Tokens += uint16(count)
		data.Items[id] -= uint16(count)
		dirty = true
		gachaLock.Unlock()
		return ctx.RespondPrivate(fmt.Sprintf("Sold %d of %s and recieved %d tokens.", count, gachaItems[id][0], count))
	} else if op == "show" {
		name := args[0].StringValue()
		id, ok := gachaShortNames[name]
		if !ok {
			return ctx.RespondPrivate("No such relic " + name)
		}
		gachaLock.RLock()
		data := gachaData[ctx.User.ID]
		if data == nil || data.Items[id] == 0 {
			gachaLock.RUnlock()
			return ctx.RespondPrivate("You don't have this relic.")
		}
		gachaLock.RUnlock()
		return ctx.RespondEmbed(makeItemEmbed(id), true)
	}
	return fmt.Errorf("illegal subcommand: %s", op)
}

// ~!trade create <name to give> <count> <user> <name to take> <count>
// Offers to trade relics or tokens with another user. You will be given a code which can be used to accept the trade.
// Use "token" as the name to give or the name to take to trade reroll tokens.
// ~!trade info <code> will show info about the trade.
// ~!trade accept <code> or ~!trade reject <code> will end a trade.
// Trades expire after some time, so you should ensure the other person is online. Maybe even @ them.
func trade(ctx commands.Context) error {
	op := ctx.ApplicationCommandData().Options[0].StringValue()
	args := ctx.ApplicationCommandData().Options[0].Options
	switch op {
	case "create":
		trade := new(TradeData)
		trade.From = ctx.User.ID
		target := args[2].UserValue(ctx.Bot)
		trade.To = target.ID
		trade.Expire = time.Now().Add(5 * time.Minute)
		var ok bool
		trade.GiveCount = uint16(args[1].IntValue())
		trade.GetCount = uint16(args[4].IntValue())
		if trade.GiveCount != 0 {
			name := args[0].StringValue()
			if name == "token" {
				trade.Giving = -1
			} else {
				trade.Giving, ok = gachaShortNames[name]
				if !ok {
					return ctx.RespondPrivate("No such relic " + name)
				}
			}
		}
		if trade.GetCount != 0 {
			name := args[3].StringValue()
			if name == "token" {
				trade.Getting = -1
			} else {
				trade.Getting, ok = gachaShortNames[name]
				if !ok {
					return ctx.RespondPrivate("No such relic " + name)
				}
			}
		}

		gachaLock.RLock()
		dataFrom := gachaData[trade.From]
		dataTo := gachaData[trade.To]
		if dataFrom == nil {
			dataFrom = new(UserData)
			dataFrom.Items = make(map[int]uint16)
			// DEBUG CODE
			// dataFrom.Tokens = 999
		}
		if dataTo == nil {
			dataTo = new(UserData)
			dataTo.Items = make(map[int]uint16)
			// DEBUG CODE
			// dataTo.Tokens = 999
		}
		if trade.Giving >= 0 {
			if dataFrom.Items[trade.Giving] < trade.GiveCount {
				gachaLock.RUnlock()
				return ctx.RespondPrivate("Trade sender does not have enough relics.")
			}
		} else if dataFrom.Tokens < trade.GiveCount {
			gachaLock.RUnlock()
			return ctx.RespondPrivate("Trade sender does not have enough tokens.")
		}
		if trade.Getting >= 0 {
			if dataTo.Items[trade.Getting] < trade.GetCount {
				gachaLock.RUnlock()
				return ctx.RespondPrivate("Trade recipient does not have enough relics.")
			}
		} else if dataTo.Tokens < trade.GetCount {
			gachaLock.RUnlock()
			return ctx.RespondPrivate("Trade recipient does not have enough tokens.")
		}
		gachaLock.RUnlock()
		if dataFrom.Wait.IsZero() || dataTo.Wait.IsZero() {
			gachaLock.Lock()
			gachaData[trade.From] = dataFrom
			gachaData[trade.To] = dataTo
			gachaLock.Unlock()
		}
		tradeLock.Lock()
		tcode := rand.Intn(9999)
		for trades[tcode] != nil && time.Now().Before(trades[tcode].Expire) {
			tcode = rand.Intn(9999)
		}
		trades[tcode] = trade
		tradeLock.Unlock()
		fallthrough
	case "info":
		tcode := int(args[0].IntValue())
		tradeLock.RLock()
		trade := trades[tcode]
		tradeLock.RUnlock()
		if trade == nil {
			return ctx.RespondPrivate("Invalid code")
		}
		if ctx.User.ID != trade.To && ctx.User.ID != trade.From {
			return ctx.RespondPrivate("You are not a participant in this trade")
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
		if op != "info" {
			embed.Description = fmt.Sprintf("Trade created. <@%s>, accept with /trade accept %04d\nCancel with /trade reject %04d", trade.To, tcode, tcode)
		}
		return ctx.RespondEmbed(embed, false)
	case "accept":
		fallthrough
	case "reject":
		tcode := int(args[0].IntValue())
		tradeLock.RLock()
		trade := trades[tcode]
		tradeLock.RUnlock()
		if trade == nil {
			return ctx.RespondPrivate("Invalid code")
		}
		if ctx.User.ID != trade.To && ctx.User.ID != trade.From {
			return ctx.RespondPrivate("You are not a participant in this trade")
		}
		if op == "accept" && ctx.User.ID == trade.From {
			return ctx.RespondPrivate("Sender cannot force accept trade.")
		}
		tradeLock.Lock()
		trades[tcode] = nil
		tradeLock.Unlock()
		if time.Now().After(trade.Expire) {
			return ctx.RespondPrivate("Trade expired")
		}
		if op == "reject" {
			return ctx.RespondPrivate("Trade rejected/cancelled.")
		}
		gachaLock.RLock()
		dataFrom := gachaData[trade.From]
		dataTo := gachaData[trade.To]
		if trade.Giving >= 0 {
			if dataFrom.Items[trade.Giving] < trade.GiveCount {
				gachaLock.RUnlock()
				return ctx.RespondPrivate("Trade sender no longer has enough relics.")
			}
		} else if dataFrom.Tokens < trade.GiveCount {
			gachaLock.RUnlock()
			return ctx.RespondPrivate("Trade sender no longer has enough tokens.")
		}
		if trade.Getting >= 0 {
			if dataTo.Items[trade.Getting] < trade.GetCount {
				gachaLock.RUnlock()
				return ctx.RespondPrivate("Trade recipient no longer has enough relics.")
			}
		} else if dataTo.Tokens < trade.GetCount {
			gachaLock.RUnlock()
			return ctx.RespondPrivate("Trade recipient no longer has enough tokens.")
		}
		gachaLock.RUnlock()
		gachaLock.Lock()
		dirty = true
		if trade.Giving >= 0 {
			dataFrom.Items[trade.Giving] -= trade.GiveCount
			dataTo.Items[trade.Giving] += trade.GiveCount
		} else {
			dataFrom.Tokens -= trade.GiveCount
			dataTo.Tokens += trade.GiveCount
		}
		if trade.Getting >= 0 {
			dataTo.Items[trade.Getting] -= trade.GetCount
			dataFrom.Items[trade.Getting] += trade.GetCount
		} else {
			dataTo.Tokens -= trade.GetCount
			dataFrom.Tokens += trade.GetCount
		}
		gachaLock.Unlock()
		return ctx.RespondPrivate("Trade successful.")
	default:
		return fmt.Errorf("illegal subcommand: %s", op)
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
	err := commands.LoadPersistent("../modules/gacha/relics.json", &gachaItems)
	if err != nil {
		log.Error(err)
		return
	}
	err = commands.LoadPersistent("gacha", &gachaData)
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

	commands.PrepareCommand("pull", "Pull a relic").Register(pull, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("usetoken", "If true, tokens will be used").AsBool().Finalize(),
	})
	shortName := commands.NewCommandOption("relic", "Short name of a relic").AsString().Required().Finalize()
	commands.PrepareCommand("relic", "Manage your collection").Register(relics, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("sell", "Sell relics for tokens").AsSubcommand([]*discordgo.ApplicationCommandOption{
			shortName,
			commands.NewCommandOption("count", "Number to sell, default 1").AsInt().SetMinMax(1, 65535).Finalize(),
		}),
		commands.NewCommandOption("show", "Show info about a relic").AsSubcommand([]*discordgo.ApplicationCommandOption{shortName}),
		commands.NewCommandOption("list", "List your relics").AsSubcommand([]*discordgo.ApplicationCommandOption{
			commands.NewCommandOption("page", "Page number").AsInt().SetMinMax(1, 65535).Finalize(),
		}),
	})
	tradeCode := commands.NewCommandOption("code", "Trade code").AsInt().Required().Finalize()
	commands.PrepareCommand("trade", "Trade relics with others").Guild().Register(trade, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("create", "Create a new trade").AsSubcommand([]*discordgo.ApplicationCommandOption{
			commands.NewCommandOption("give", "Short name of relic or \"token\" for tokens").AsString().Required().Finalize(),
			commands.NewCommandOption("givecount", "Amount to give").AsInt().SetMinMax(1, 65535).Required().Finalize(),
			commands.NewCommandOption("user", "User to trade with").AsUser().Required().Finalize(),
			commands.NewCommandOption("get", "Short name of relic or \"token\" for tokens").AsString().Required().Finalize(),
			commands.NewCommandOption("getcount", "Amount to get").AsInt().SetMinMax(1, 65535).Required().Finalize(),
		}),
		commands.NewCommandOption("info", "Show info about a trade").AsSubcommand([]*discordgo.ApplicationCommandOption{tradeCode}),
		commands.NewCommandOption("accept", "Accept a trade").AsSubcommand([]*discordgo.ApplicationCommandOption{tradeCode}),
		commands.NewCommandOption("cancel", "Cancel or reject trade").AsSubcommand([]*discordgo.ApplicationCommandOption{tradeCode}),
	})
	commands.RegisterSaver(saveGacha)
}

func saveGacha() error {
	if !dirty {
		return nil
	}
	gachaLock.Lock()
	for _, x := range gachaData {
		for k, v := range x.Items {
			if v == 0 {
				delete(x.Items, k)
			}
		}
	}
	err := commands.SavePersistent("gacha", &gachaData)
	if err == nil {
		dirty = false
	}
	gachaLock.Unlock()
	return err
}

// Cleanup is defined in the command interface to clean up the module when the bot unloads.
// Here, it saves the kek data to disk.
func Cleanup(_ *discordgo.Session) {
	err := saveGacha()
	if err != nil {
		log.Error(err)
	}
}
