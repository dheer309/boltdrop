package main

import (
	"boltdrop/chunker"
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	// get file path from user
	filename := os.Args[1]
	file, err := os.Open(filename)

	if err != nil {
		fmt.Println("Error reading file")
		return
	}

	defer file.Close()

	manifest, err := chunker.GenerateManifest(filename)
	if err != nil {
		fmt.Println("Error while generating manifest:", err)
		return
	}

	fmt.Printf("File: %s, Size: %d, Chunks: %d\n", manifest.Filename, manifest.FileSize, len(manifest.Chunks))

	// try to connect to localhost thru tcp
	conn, err := net.Dial("tcp", "localhost:8000")

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer conn.Close()

	// copy file to connection
	io.Copy(conn, file)
}
