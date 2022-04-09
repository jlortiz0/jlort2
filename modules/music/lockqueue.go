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

package music

import "sync"

// import "runtime"
// import "fmt"
// import "jlortiz.org/jlort2/modules/log"

type queueObj struct {
	next   *queueObj
	prev   *queueObj
	Value  *StreamObj
	parent *lockQueue
}

func (o *queueObj) Next() *queueObj {
	return o.next
}

func (o *queueObj) Prev() *queueObj {
	return o.prev
}

type lockQueue struct {
	sync.RWMutex

	head   *queueObj
	tail   *queueObj
	length int
}

func (q *lockQueue) Clear() {
	q.head = nil
	q.tail = nil
	q.length = 0
}

func (q *lockQueue) Head() *queueObj {
	return q.head
}

func (q *lockQueue) Tail() *queueObj {
	return q.tail
}

func (q *lockQueue) Len() int {
	return q.length
}

func (q *lockQueue) PushBack(obj *StreamObj) *queueObj {
	qo := new(queueObj)
	qo.Value = obj
	if q.tail != nil {
		qo.prev = q.tail
		qo.prev.next = qo
	}
	qo.parent = q
	q.length += 1
	q.tail = qo
	if q.length == 1 {
		q.head = qo
	}
	return qo
}

func (q *lockQueue) PushFront(obj *StreamObj) *queueObj {
	qo := new(queueObj)
	qo.Value = obj
	if q.head != nil {
		qo.next = q.head
		qo.next.prev = qo
	}
	qo.parent = q
	q.length += 1
	q.head = qo
	if q.length == 1 {
		q.tail = qo
	}
	return qo
}

func (q *lockQueue) Remove(obj *queueObj) {
	if obj.parent != q {
		return
	}
	if obj == q.tail {
		q.tail = obj.prev
	} else if obj == q.head {
		q.head = obj.next
	} else {
		obj.prev.next = obj.next
		obj.next.prev = obj.prev
	}
	q.length -= 1
}

func (q *lockQueue) Lock() {
	// pc, fi, line, ok := runtime.Caller(1)
	// if ok {
	// 	name := runtime.FuncForPC(pc).Name()
	// 	log.Debug(fmt.Sprintf("%s:%s:%d: Lock", fi, name, line))
	// }
	q.RWMutex.Lock()
}

func (q *lockQueue) Unlock() {
	// pc, fi, line, ok := runtime.Caller(1)
	// if ok {
	// 	name := runtime.FuncForPC(pc).Name()
	// 	log.Debug(fmt.Sprintf("%s:%s:%d: Unlock", fi, name, line))
	// }
	q.RWMutex.Unlock()
}

func (q *lockQueue) RLock() {
	// pc, fi, line, ok := runtime.Caller(1)
	// if ok {
	// 	name := runtime.FuncForPC(pc).Name()
	// 	log.Debug(fmt.Sprintf("%s:%s:%d: RLock", fi, name, line))
	// }
	q.RWMutex.RLock()
}

func (q *lockQueue) RUnlock() {
	// pc, fi, line, ok := runtime.Caller(1)
	// if ok {
	// 	name := runtime.FuncForPC(pc).Name()
	// 	log.Debug(fmt.Sprintf("%s:%s:%d: RUnlock", fi, name, line))
	// }
	q.RWMutex.RUnlock()
}
