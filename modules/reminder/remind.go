package reminder

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/commands"
	"jlortiz.org/jlort2/modules/log"
)

var stmtIns, stmtSel, stmtSelU, stmtCount, stmtClean *sql.Stmt
var channelCache map[string]string
var runStopper chan struct{}

const tsFormat = "Jan _2 3:04 PM MST"
const max_reminders_per_user = 32

func remind(ctx *commands.Context) error {
	when := ctx.ApplicationCommandData().Options[0].StringValue()
	what := ctx.ApplicationCommandData().Options[1].StringValue()
	if len(what) > 2000 {
		return ctx.RespondPrivate("Reminder is too long, max 2000 chars")
	}
	t := parseTime(when)
	if t.IsZero() {
		log.Debug(when)
		return ctx.RespondPrivate("Unable to parse time: " + when)
	}
	count := stmtCount.QueryRow(ctx.Interaction.User.ID)
	var rCount int
	count.Scan(&rCount)
	if rCount >= max_reminders_per_user {
		return ctx.RespondPrivate("Reached limit of " + strconv.Itoa(max_reminders_per_user) + " reminders")
	}
	now := time.Now()
	stmtIns.Exec(t, ctx.Interaction.User.ID, now, what)
	return ctx.RespondPrivate("I will remind you on " + t.Format(tsFormat) + ". To cancel, do /remindcancel " + strconv.Itoa(rCount+1))
}

func remindcancel(ctx *commands.Context) error {
	ind := ctx.ApplicationCommandData().Options[0].IntValue()
	row := stmtCount.QueryRow(ctx.User.ID)
	var count int
	row.Scan(&count)
	if count < int(ind) {
		return ctx.RespondPrivate("Index too large, expected < " + strconv.Itoa(count))
	}
	ctx.Database.Exec(`DELETE FROM reminders WHERE rowid IN (SELECT rowid FROM reminders WHERE uid = ?001 ORDER BY created ASC LIMIT 1 OFFSET ?002);`, ctx.User.ID, ind-1)
	return ctx.RespondPrivate("Reminder has been removed.")
}

func reminders(ctx *commands.Context) error {
	results, err := stmtSelU.Query(ctx.User.ID)
	if err != nil {
		return err
	}
	defer results.Close()
	builder := new(strings.Builder)
	var i int
	var ts time.Time
	var what string
	for results.Next() {
		i += 1
		results.Scan(&ts, &what)
		builder.WriteString(strconv.Itoa(i))
		// .Local is used so that the UTC offset is converted into a proper Location with a name
		builder.WriteString(ts.Local().Format(". [" + tsFormat + "] "))
		builder.WriteString(what)
		builder.WriteByte('\n')
	}
	if builder.Len() == 0 {
		return ctx.RespondPrivate("You have no reminders.")
	}
	output := new(discordgo.MessageEmbed)
	output.Title = "Reminders"
	output.Description = builder.String()[:builder.Len()-1]
	output.Color = 0x7289da
	return ctx.RespondEmbed(output, true)
}

func runner(self *discordgo.Session, stopper <-chan struct{}) {
	timer := time.NewTicker(time.Minute)
	var t time.Time
	for {
		select {
		case t = <-timer.C:
		case <-stopper:
			return
		}
		rows, err := stmtSel.Query(t)
		if err != nil {
			log.Error(fmt.Errorf("failed to query reminder table: %w", err))
			return
		}
		defer rows.Close()
		empty := true
		var uid, what string
		var created time.Time
		for rows.Next() {
			empty = false
			rows.Scan(&uid, &created, &what)
			chanId, ok := channelCache[uid]
			if ok && chanId == "0" {
				// we previously failed to create this channel, skip this one
				continue
			} else if !ok {
				channel, err := self.UserChannelCreate(uid)
				if err != nil {
					log.Error(fmt.Errorf("failed to create dm channel: %w", err))
					channelCache[uid] = "0"
					continue
				}
				channelCache[uid] = channel.ID
				chanId = channel.ID
			}
			_, err = self.ChannelMessageSend(chanId, "A reminder for you, from "+created.Format(tsFormat)+":\n\n"+what)
			if err != nil {
				log.Error(fmt.Errorf("failed to send message: %w", err))
				channelCache[uid] = "0"
			}
		}
		if !empty {
			stmtClean.Exec(t)
		}
	}
}

func Init(self *discordgo.Session) {
	stmtIns, _ = commands.GetDatabase().Prepare(`INSERT INTO reminders (ts, uid, created, what) VALUES (?001, ?002, ?003, ?004);`)
	stmtCount, _ = commands.GetDatabase().Prepare(`SELECT COUNT(*) FROM reminders WHERE uid = ?001;`)
	stmtSel, _ = commands.GetDatabase().Prepare(`SELECT uid, created, what FROM reminders WHERE ts < ?001;`)
	stmtSelU, _ = commands.GetDatabase().Prepare(`SELECT ts, what FROM reminders WHERE uid = ?001 ORDER BY created ASC;`)
	stmtClean, _ = commands.GetDatabase().Prepare(`DELETE FROM reminders WHERE ts < ?001;`)
	channelCache = make(map[string]string)
	commands.PrepareCommand("remind", "Set a reminder").Register(remind, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("when", "When to send the reminder, accepts \"1d\", \"5h\", \"10mo\", and combinations").AsString().Required().Finalize(),
		commands.NewCommandOption("what", "What to remind you about").AsString().Required().Finalize(),
	})
	commands.PrepareCommand("remnindcancel", "Cancel a reminder").Register(remindcancel, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("id", "Index of reminder to cancel").AsInt().SetMinMax(1, max_reminders_per_user).Required().Finalize(),
	})
	commands.PrepareCommand("reminders", "See all your reminders").Register(reminders, nil)
	runStopper = make(chan struct{})
	go runner(self, runStopper)
}

func Cleanup(self *discordgo.Session) {
	stmtIns.Close()
	stmtCount.Close()
	stmtSel.Close()
	stmtSelU.Close()
	stmtClean.Close()
	close(runStopper)
}
