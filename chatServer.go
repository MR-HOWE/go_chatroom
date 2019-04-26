package main

import (
	"log"
	"net"
	"fmt"
	"bufio"
	"time"
)


// 在timeout时间内未发任何消息，自动断开连接
const timeout = 10 * time.Minute
// const timeout = 20 * time.Second

type client struct{
	Out chan<- string // an outgoing message channel
	Name string
	Addr string
}

type cliAndCommand struct{
	cli client
	command string
}

var (
	entering = make(chan client)
	leaving = make(chan client)
	messages = make(chan string) // all incoming client messages
	cliAndCommands = make(chan cliAndCommand, 5000)
)

var clients = make(map[client]bool)

// key为命令，value为函数指针
var commands = map[string]func(client){
	"/ls": listAllOnlineUsers,
}

func listAllOnlineUsers(cli client) {
	cli.Out <- "当前在线成员:"
	for c := range clients {
		cli.Out <- c.Name
	}
}

func broadcaster() {
	// clients := make(map[client]bool) // all connected clients
	for {
		select {
			case msg := <-messages:
				// Broadcast incoming message to all
				// clients' outgoing message channels.
				for cli := range clients {
					cli.Out <- msg
				}
			case cli := <-entering:
				clients[cli] = true
				cli.Out <- "当前在线成员:"
				for c := range clients {
					cli.Out <- c.Name
				}
			
			case obj := <-cliAndCommands:
				(commands[obj.command])(obj.cli)

			case cli := <-leaving:
				delete(clients, cli)
				close(cli.Out)
		}
	}
}

func handleConn(conn net.Conn) {
	ch := make(chan string) // outgoing client messages
	go clientWriter(conn, ch)
	
	addr := conn.RemoteAddr().String()
	var who string
	
	ch <- "[System] Please enter your name:"
	input := bufio.NewScanner(conn)
	if input.Scan() {
		who = input.Text()
	}

	ch <- "OK, now you are " + who
	ch <- "Start chatting~"

	cli := client{ch, who, addr}
	messages <- "[System] " + who + " has arrived"
	entering <- cli

	quit := make(chan int)

	timer := time.NewTimer(timeout)
	go func() {
		<-timer.C
		cli.Out <- "[System] Idle too long. You are kicked out."
		quit <- 1
	}()

	go func() {
		<- quit
		leaving <- cli
		messages <- "[System] " + who + " has left"
		conn.Close()
	}()

	for input.Scan() {
		if commands[input.Text()] != nil {
			cliAndCommands <- cliAndCommand{cli, input.Text()}
		} else {
			messages <- who + ": " + input.Text()
		}
		timer.Reset(timeout)
	}
	
	timer.Stop() // 计时器要停掉，防止close连接后，还重新close一次
	quit <- 1
}

// 往 client 写入信息
func clientWriter(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		fmt.Fprintln(conn, msg) // NOTE: ignoring network errors
	}
}

func main() {
	listener, err := net.Listen("tcp", "localhost:8000")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Server successfully starts!")
	go broadcaster()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleConn(conn)
	}
}