/*
Copyright (C) 2021-2023 jlortiz

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"jlortiz.org/jlort2/modules/log"
)

func updatePfp(self *discordgo.Session) {
	t := time.Tick(6 * time.Hour)
	for {
		f, err := os.Open("pfps" + string(os.PathSeparator) + "defs.dat")
		if err != nil {
			log.Error(err)
			return
		}
		defer f.Close()
		rd := bufio.NewReader(f)
		ts := time.Now()
		ots := ts.Add(-6 * time.Hour)
		// fmt.Println(ts, ots)
		var buf [4]byte
		var dFlag bool
		var name string
		var n int
		for {
			n, err = rd.Read(buf[:])
			if n != 4 {
				break
			}
			name, err = rd.ReadString(0)
			if err != nil {
				break
			}
			name = name[:len(name)-1]
			startts := time.Date(ts.Year(), time.Month(buf[0]), int(buf[1]), 0, 0, 0, 0, ts.Location())
			endts := time.Date(ts.Year(), time.Month(buf[2]), int(buf[3]), 0, 0, 0, 0, ts.Location())
			// fmt.Println(buf, startts.Before(ts), endts.Before(ts), startts.Before(ots), endts.Before(ots))
			if startts.Before(ts) && !endts.Before(ts) {
				if dFlag || ots.Before(startts) {
					avatar, err := os.ReadFile("pfps" + string(os.PathSeparator) + name)
					if err == nil {
						_, err = self.UserUpdate("", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(avatar))
						if err != nil {
							log.Error(fmt.Errorf("could not set avatar: %w", err))
						} else {
							log.Warn("Updated profile picture")
						}
					} else {
						log.Error(fmt.Errorf("could not read avatar: %w", err))
					}
				}
				return
			} else if !dFlag && startts.Before(ots) && !endts.Before(ots) {
				dFlag = true
			}
		}
		if err != io.EOF {
			log.Error(fmt.Errorf("unable to update PFP: %w", err))
		} else {
			log.Warn("PFP defs missing default!")
		}
		<-t
	}
}
