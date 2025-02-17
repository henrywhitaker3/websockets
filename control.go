package websockets

import "sync"

type control struct {
	conn *sync.Mutex
	tx   *sync.Mutex
	rx   *sync.Mutex
}

func newControl() *control {
	return &control{
		conn: &sync.Mutex{},
		tx:   &sync.Mutex{},
		rx:   &sync.Mutex{},
	}
}

func (c *control) lockConn() {
	c.lockRx()
	c.lockTx()
	c.conn.Lock()
}

func (c *control) unlockConn() {
	c.unlockRx()
	c.unlockTx()
	c.conn.Unlock()
}

func (c *control) lockTx() {
	c.tx.Lock()
}

func (c *control) unlockTx() {
	c.tx.Unlock()
}

func (c *control) lockRx() {
	c.rx.Lock()
}

func (c *control) unlockRx() {
	c.rx.Unlock()
}
