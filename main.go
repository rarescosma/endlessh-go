package main

//  echo -n "test out the server" | nc localhost 3333

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync/atomic"
	"time"
)

var (
	numCurrentClients int64
	numTotalClients   uint64
	numTotalBytes     uint64
)

var letterBytes = []byte(" abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890!@#$%^&*()-=_+[]{}|;:',./<>?")

func randStringBytes(n int64) []byte {
	b := make([]byte, n+1)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	b[n] = '\n'
	return b
}

type client struct {
	conn       net.Conn
	next       time.Time
	start      time.Time
	bytes_sent int
}

func NewClient(conn net.Conn, interval time.Duration, maxClient int64) *client {
	atomic.AddInt64(&numCurrentClients, 1)
	atomic.AddUint64(&numTotalClients, 1)
	fmt.Printf("%v ACCEPT host=%v n=%v/%v\n", time.Now(), conn.RemoteAddr(), numCurrentClients, maxClient)
	return &client{
		conn:       conn,
		next:       time.Now().Add(interval),
		start:      time.Now(),
		bytes_sent: 0,
	}
}

func (c *client) Send(bannerMaxLength int64) error {
	length := rand.Int63n(bannerMaxLength)
	bytes_sent, err := c.conn.Write(randStringBytes(length))
	if err != nil {
		return err
	}
	c.bytes_sent += bytes_sent
	atomic.AddUint64(&numTotalBytes, uint64(bytes_sent))
	return nil
}

func (c *client) Close() {
	atomic.AddInt64(&numCurrentClients, -1)
	fmt.Printf("%v CLOSE host=%v time=%v bytes=%v\n", time.Now(), c.conn.RemoteAddr(), time.Now().Sub(c.start), c.bytes_sent)
	c.conn.Close()
}

func main() {
	intervalMs := flag.Int("interval_ms", 1000, "Message millisecond delay")
	bannerMaxLength := flag.Int64("line_length", 32, "Maximum banner line length")
	maxClients := flag.Int64("max_clients", 4096, "Maximum number of clients")
	connType := flag.String("conn_type", "tcp", "Connection type. Possible values are tcp, tcp4, tcp6")
	connHost := flag.String("host", "localhost", "Listening address")
	connPort := flag.String("port", "2222", "Listening port")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %v \n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	interval := time.Duration(*intervalMs) * time.Millisecond
	// Listen for incoming connections.
	l, err := net.Listen(*connType, *connHost+":"+*connPort)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()
	fmt.Println("Listening on " + *connHost + ":" + *connPort)

	clients := make(chan *client, *maxClients)
	go func(clients chan *client, interval time.Duration, bannerMaxLength int64) {
		for {
			c, more := <-clients
			if !more {
				return
			}
			if time.Now().Before(c.next) {
				time.Sleep(c.next.Sub(time.Now()))
			}
			err := c.Send(bannerMaxLength)
			if err != nil {
				c.Close()
				continue
			}
			c.next = time.Now().Add(interval)
			go func() { clients <- c }()
		}
	}(clients, interval, *bannerMaxLength)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		for numCurrentClients >= *maxClients {
			time.Sleep(interval)
		}
		clients <- NewClient(conn, interval, *maxClients)
	}
}