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
	if len(os.Args) < 2 {
		fmt.Println("Enter filename when running the program")
		return
	}

	// get file path from user
	filename := os.Args[1]
	// open the file
	file, err := os.Open(filename)

	if err != nil {
		fmt.Println("Error reading file")
		return
	}

	defer file.Close()

	// generate manifest for the target file
	manifest, err := chunker.GenerateManifest(filename)

	if err != nil {
		fmt.Println("Error while generating manifest:", err)
		return
	}

	// convert the manifest in JSON
	manifestJson, err := json.Marshal(manifest)

	if err != nil {
		fmt.Println("Error when converting manifest to JSON:", err)
		return
	}

	// try to connect to localhost thru tcp
	conn, err := net.Dial("tcp", "localhost:8000")

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer conn.Close()

	// create a header and send the manifest's length through it
	header := make([]byte, 8) // header is 8 bytes long
	binary.BigEndian.PutUint64(header, uint64(len(manifestJson)))
	conn.Write(header)

	// after sending the header, send the manifest json itself
	n, err := conn.Write(manifestJson)

	if err != nil {
		fmt.Printf("Error while sending manifest of %d bytes: %v", n, err)
		return
	}

	// copy file to connection
	io.Copy(conn, file)
}
