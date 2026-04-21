package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	// wait until another terminal connects to port 8000
	listener, err := net.Listen("tcp", ":8000")

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	defer listener.Close()

	fmt.Println("Listening on 8000")

	for {
		// accept connection
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("Error: ", err)
		}

		go handleConnection(conn)
	}
}

// does all the heavy lifting
func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("New connection from ", conn.RemoteAddr())

	file, err := os.Create("recieved.txt")
	if err != nil {
		fmt.Println("Error: ", err)
	}

	defer file.Close()

	// copy the file sent through the sender and create a new one
	io.Copy(file, conn)
	fmt.Println("File recieved")
}
