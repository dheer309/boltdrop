package main

import (
	"boltdrop/chunker"
	"encoding/binary"
	"encoding/json"
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

	// defining the header of being 8 bytes long
	header := make([]byte, 8)

	// reading only the first 8 bytes from the connection
	_, err := io.ReadFull(conn, header)

	if err != nil {
		fmt.Println("Cannot fetch header", err)
		return
	}

	// fetching the manifest's actual length from the header
	manifestLength := binary.BigEndian.Uint64(header)

	// reading only the manifest
	manifestJson := make([]byte, manifestLength)
	_, err = io.ReadFull(conn, manifestJson)

	if err != nil {
		fmt.Println("Cannot fetch manifest", err)
		return
	}

	// populating that json data into the manifest data structure
	var manifest chunker.Manifest
	err = json.Unmarshal(manifestJson, &manifest)

	if err != nil {
		fmt.Println("Cannot convert into Manifest struct", err)
		return
	}

	// send the actual file
	file, err := os.Create(manifest.Filename)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	defer file.Close()

	// copy the file sent through the sender and create a new one
	io.Copy(file, conn)
	fmt.Println("File recieved")
}
