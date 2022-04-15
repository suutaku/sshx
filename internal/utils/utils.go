package utils

import (
	"log"
	"net"
)

func chanFromConn(conn net.Conn) chan []byte {
	c := make(chan []byte)

	go func() {
		b := make([]byte, 1024)

		for {
			n, err := conn.Read(b)
			if n > 0 {
				res := make([]byte, n)
				// Copy the buffer so it doesn't get changed while read by the recipient.
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				c <- nil
				break
			}
		}
	}()

	return c
}

func Pipe(conn1 net.Conn, conn2 net.Conn) {
	chan1 := chanFromConn(conn1)
	chan2 := chanFromConn(conn2)

	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return
			} else {
				_, err := conn2.Write(b1)
				if err != nil {
					log.Fatal(err)
					return
				}
			}
		case b2 := <-chan2:
			if b2 == nil {
				return
			} else {
				_, err := conn1.Write(b2)
				if err != nil {
					log.Fatal(err)
					return
				}
			}
		}
	}
}
