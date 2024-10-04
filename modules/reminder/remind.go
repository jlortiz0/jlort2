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

var stmtIns, stmtSel, stmtSelU, stmtCount, stmtClean, stmtGetTz *sql.Stmt
var channelCache map[string]string
var runStopper chan struct{}

const shortTsFormat = "Jan _2 3:04 PM"
const tsFormat = shortTsFormat + " MST"
const max_reminders_per_user = 32

func loadTz(uid string) (*time.Location, bool, error) {
	zone := time.Local
	row := stmtGetTz.QueryRow(uid)
	var zoneS string
	row.Scan(&zoneS)
	if zoneS != "" {
		var err error
		zone, err = time.LoadLocation(zoneS)
		if err != nil {
			return nil, true, fmt.Errorf("failed to load tz %s: %w", zoneS, err)
		}
	}
	return zone, zoneS != "", nil
}

func remind(ctx *commands.Context) error {
	when := ctx.ApplicationCommandData().Options[0].StringValue()
	what := ctx.ApplicationCommandData().Options[1].StringValue()
	if len(what) > 2000 {
		return ctx.RespondPrivate("Reminder is too long, max 2000 chars")
	}
	zone, hasZone, err := loadTz(ctx.Interaction.User.ID)
	if err != nil {
		return err
	}
	t := parseTime(when, zone)
	if t.IsZero() {
		log.Debug(when)
		return ctx.RespondPrivate("Unable to parse time: " + when)
	}
	row := stmtCount.QueryRow(ctx.Interaction.User.ID)
	var count int
	row.Scan(&count)
	if count >= max_reminders_per_user {
		return ctx.RespondPrivate("Reached limit of " + strconv.Itoa(max_reminders_per_user) + " reminders")
	}
	now := time.Now()
	stmtIns.Exec(t.In(time.Local), ctx.Interaction.User.ID, now, what)
	msg := "I will remind you on " + t.Format(tsFormat) + ". To cancel, do /remindcancel " + strconv.Itoa(count+1)
	if !hasZone {
		msg += "\nIf the above time zone is incorrect, use /settz to set it"
	}
	return ctx.RespondPrivate(msg)
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
	zone, _, err := loadTz(ctx.Interaction.User.ID)
	if err != nil {
		return err
	}
	builder := new(strings.Builder)
	var i int
	var ts time.Time
	var what string
	for results.Next() {
		i += 1
		results.Scan(&ts, &what)
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(ts.In(zone).Format(". [" + tsFormat + "] "))
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

func settz(ctx *commands.Context) error {
	where := strings.ToUpper(ctx.ApplicationCommandData().Options[0].StringValue())
	zoneS, ok := timezones[where]
	if !ok {
		return ctx.RespondPrivate("Time zone not recognized, use the abbrevation (GMT, PST, NZT, etc)")
	}
	zone, err := time.LoadLocation(zoneS)
	if err != nil {
		return fmt.Errorf("failed to load tz %s: %w", zoneS, err)
	}
	row := stmtCount.QueryRow(ctx.Interaction.User.ID)
	var rCount int
	row.Scan(&rCount)
	var suffix string
	if rCount != 0 {
		suffix = "\nCheck existing reminders with /reminders to ensure that the times are stil correct."
	}
	ctx.Database.Exec("INSERT OR REPLACE INTO userTz (uid, tz) VALUES (?001, ?002);", ctx.Interaction.User.ID, zoneS)
	if where == zoneS {
		return ctx.RespondPrivate("Set timezone to " + where + suffix)
	}
	return ctx.RespondPrivate("Set timezone to " + where + ", aka " + zone.String() + suffix)
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
		var tzS string
		for rows.Next() {
			empty = false
			tz := time.Local
			rows.Scan(&uid, &created, &what, &tzS)
			if tzS != "" {
				tz, err = time.LoadLocation(tzS)
				if err != nil {
					tz = time.Local
				}
			}
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
			_, err = self.ChannelMessageSend(chanId, "A reminder for you, from "+created.In(tz).Format(shortTsFormat)+":\n\n"+what)
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
	stmtSel, _ = commands.GetDatabase().Prepare(`SELECT reminders.uid, reminders.created, reminders.what, userTz.tz
												 FROM reminders LEFT JOIN userTz ON reminders.uid = userTz.uid
												 WHERE reminders.ts < ?001;`)
	stmtSelU, _ = commands.GetDatabase().Prepare(`SELECT ts, what FROM reminders WHERE uid = ?001 ORDER BY created ASC;`)
	stmtClean, _ = commands.GetDatabase().Prepare(`DELETE FROM reminders WHERE ts < ?001;`)
	stmtGetTz, _ = commands.GetDatabase().Prepare("SELECT tz FROM userTz WHERE uid = ?001;")
	channelCache = make(map[string]string)
	commands.PrepareCommand("remind", "Set a reminder").Register(remind, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("when", "When to send the reminder, accepts \"1d\", \"5h3m\", \"8pm\", \"25th\", \"March 7th 5:55 AM\"").AsString().Required().Finalize(),
		commands.NewCommandOption("what", "What to remind you about").AsString().Required().Finalize(),
	})
	commands.PrepareCommand("remnindcancel", "Cancel a reminder").Register(remindcancel, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("id", "Index of reminder to cancel").AsInt().SetMinMax(1, max_reminders_per_user).Required().Finalize(),
	})
	commands.PrepareCommand("reminders", "See all your reminders").Register(reminders, nil)
	commands.PrepareCommand("settz", "Set time zone").Register(settz, []*discordgo.ApplicationCommandOption{
		commands.NewCommandOption("zone", "Time zone abbreviation (GMT, PST, NZT, etc)").AsString().Required().Finalize(),
	})
	runStopper = make(chan struct{})
	go runner(self, runStopper)
}

func Cleanup(self *discordgo.Session) {
	stmtIns.Close()
	stmtCount.Close()
	stmtSel.Close()
	stmtSelU.Close()
	stmtClean.Close()
	stmtGetTz.Close()
	close(runStopper)
}
