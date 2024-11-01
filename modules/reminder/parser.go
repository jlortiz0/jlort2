package reminder

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var regRel = regexp.MustCompile(`(\d+) ?(m[io]?|[dhwy])[a-z]*`)
var regAbs = regexp.MustCompile(`^(?:([a-z]{3,}) (\d+)(?:[thndrs]+)?)? ?(?:(\d\d?)(?::(\d\d))?(?: ([ap])m?)?)?$`)

func parseTime(s string, zone *time.Location) (t time.Time) {
	s = strings.ToLower(s)
	t = time.Now().In(zone)
	match := regRel.FindAllStringSubmatch(s, -1)
	if len(match) > 0 {
		var acc int
		for _, x := range match {
			acc, _ = strconv.Atoi(x[1])
			switch x[2][0] {
			case 'd':
				t = t.AddDate(0, 0, acc)
			case 'h':
				t = t.Add(time.Hour * time.Duration(acc))
			case 'w':
				t = t.AddDate(0, 0, acc*7)
			case 'y':
				t = t.AddDate(acc, 0, 0)
			case 'm':
				if len(x[2]) == 1 || x[2][1] == 'i' {
					t = t.Add(time.Minute * time.Duration(acc))
				} else {
					t = t.AddDate(0, acc, 0)
				}
			}
		}
		return
	}
	match2 := regAbs.FindStringSubmatch(s)
	if len(match2) > 0 {
		match2 = match2[1:]
		t2 := t
		month := t.Month()
		day := t.Day()
		hour := t.Hour()
		minute := t.Minute()
		if len(match2[0]) > 2 {
			switch match2[0][:3] {
			case "feb":
				month = time.February
			case "sep":
				month = time.September
			case "oct":
				month = time.October
			case "nov":
				month = time.November
			case "dec":
				month = time.December
			case "jan":
				month = time.January
			case "jun":
				month = time.June
			case "jul":
				month = time.July
			case "apr":
				month = time.April
			case "aug":
				month = time.August
			case "mar":
				month = time.March
			case "may":
				month = time.May
			default:
				// illegal month, bail
				t = time.Time{}
				return
			}
		}
		if len(match2[1]) > 0 {
			day, _ = strconv.Atoi(match2[1])
		}
		if len(match2[2]) > 0 {
			hour, _ = strconv.Atoi(match2[2])
			if match2[4] == "p" {
				hour += 12
			}
		}
		if len(match2[3]) > 0 {
			minute, _ = strconv.Atoi(match2[3])
		}
		t = time.Date(t.Year(), month, day, hour, minute, t.Second(), t.Nanosecond()+5, t.Location())
		if t.Before(t2) {
			if !t.AddDate(0, 0, 1).Before(t2) {
				t = t.AddDate(0, 0, 1)
			} else if !t.AddDate(0, 1, 0).Before(t2) {
				t = t.AddDate(0, 1, 0)
			} else {
				t = t.AddDate(1, 0, 0)
			}
		}
		return
	}
	ok := strings.HasPrefix(s, "tom")
	if ok {
		t = t.AddDate(0, 0, 1)
		ind := strings.IndexByte(s, ' ')
		if ind == -1 {
			return
		}
		s = s[ind+1:]
	}
	hour := -1
	if strings.HasPrefix(s, "morn") {
		hour = 8
	} else if strings.HasPrefix(s, "noon") {
		hour = 12
	} else if strings.HasPrefix(s, "aft") {
		hour = 15
	} else if strings.HasPrefix(s, "eve") {
		hour = 18
	} else if strings.HasPrefix(s, "night") {
		hour = 20
	} else if !ok {
		t = time.Time{}
	}
	if hour != -1 {
		t = time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, t.Location())
	}
	return
}
