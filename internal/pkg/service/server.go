package service

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

type Message struct {
	remoteAddr string
	payload    []byte
}

type Server struct {
	listenAddr string
	ln         net.Listener
	quitch     chan struct{}
	msgch      chan *Message
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
		msgch:      make(chan *Message, 16),
	}
}

func (s *Server) Start() error {
	log.Printf("starting server")

	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()

	s.ln = ln

	go s.handleMsg() // one worker handling the message, probably should scale this
	go s.Accept()

	<-s.quitch
	close(s.quitch)

	return nil
}

func (s *Server) Stop() {
	log.Printf("stopping server")

	s.quitch <- struct{}{}
}

func (s *Server) Accept() {
	log.Printf("accepting incoming requests")

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			log.Printf("error on accepting: %s", err)
			continue
		}

		log.Printf("new connection to the server: %s", conn.RemoteAddr())

		go s.handleConn(conn)
	}
}

// func (s *Server) handleConn(conn net.Conn) {
// 	log.Printf("handling connection")

// 	defer conn.Close()

// 	buf := make([]byte, 1024)
// 	for {
// 		n, err := conn.Read(buf)
// 		if errors.Is(err, io.EOF) {
// 			log.Printf("error is: %s", err)
// 			break // we got EOF, we will end the connection
// 		}
// 		if err != nil {
// 			log.Printf("error while reading: %s", err)
// 			continue
// 		}

// 		s.msgch <- &Message{
// 			remoteAddr: conn.RemoteAddr().String(),
// 			payload:    buf[:n],
// 		}
// 	}
// }

func getError(err error) {
	switch {
	case
		errors.Is(err, net.ErrClosed),
		errors.Is(err, os.ErrDeadlineExceeded),
		errors.Is(err, io.EOF),
		errors.Is(err, syscall.EPIPE):
		log.Printf("error connection is timed out or closed")
	}
}

func (s *Server) handleConn(src net.Conn) {
	log.Printf("handling connection")

	src.SetDeadline(time.Now().Add(1 * time.Minute)) // timeout the connection after some time
	defer src.Close()

	// dst, err := net.Dial("tcp", "localhost:8000")

	// currently doesn't work with sites that have redirection
	dialer := net.Dialer{Timeout: 5 * time.Second} // timeout for destination
	dst, err := dialer.Dial("tcp", "localhost:8000")
	// dst, err := dialer.Dial("tcp", "water.com:80") // water.com has 11 second timeout
	if err != nil {
		log.Printf("error while connecting to target server: %s", err)
	}

	closeDst := func() {
		if err := dst.Close(); err != nil {
			log.Printf("error while closing destination: %s", err)
		}
		log.Printf("closing connection to destination server")
	}
	defer closeDst()

	/*
		https://stackoverflow.com/questions/62522276/io-copy-in-goroutine-to-prevent-blocking

		basically, as TCP is a duplex connection, we block till we get EOF or erorr, so one of the below should be in a go-routine to prevent from blocking
	*/
	go func() {
		defer closeDst() // close the destination, so that the other side also closes this connection; io.Copy blocks until EOF or error, so this will be called at last

		// we make request on behalf of the client to the target server
		if _, err := io.Copy(dst, src); err != nil {
			getError(err)
			log.Printf("error while copying response from source to destination: %s", err)
		}
	}()

	// we get the response from the target server to the client; when destination closes or is errored, we get error here too, and finally close the src connection
	if _, err := io.Copy(src, dst); err != nil {
		getError(err)
		log.Printf("error while copying response from destination to source: %s", err)
	}
}

func (s *Server) handleMsg() {
	log.Printf("handling incoming message")

	defer close(s.msgch)

	for msg := range s.msgch {
		log.Printf("received message from connection: %s", string(msg.payload))
	}
}
