/*
Copyright (C) 2021-2022 jlortiz

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

package commands

import "time"
import "jlortiz.org/jlort2/modules/log"

type saverEntry struct {
	version int
	f       func() error
}

var savers []saverEntry
var saverVersion int

const timeBetweenSaves = 30 * time.Minute

func RegisterSaver(f func() error) int {
	for i, x := range savers {
		if x.f == nil {
			x.f = f
			x.version = saverVersion
			return i
		}
	}
	savers = append(savers, saverEntry{saverVersion, f})
	return len(savers) - 1
}

func UnregisterSaver(i int) {
	if i < len(savers) && i > 0 {
		savers[i].f = nil
	}
}

func saverLoop() {
	version := saverVersion
	for {
		time.Sleep(timeBetweenSaves)
		log.Debug("Saving data!")
		for _, x := range savers {
			if x.version != version {
				return
			}
			if x.f == nil {
				continue
			}
			err := x.f()
			if err != nil {
				log.Error(err)
			}
		}
	}
}
