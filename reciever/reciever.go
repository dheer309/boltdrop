package main

import (
	"boltdrop/chunker"
	"crypto/sha256"
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

	// 4 mb
	var fourMB = 4 << 20

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
	file, err := os.OpenFile(manifest.Filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	defer file.Close()

	bucket := make([]byte, fourMB)

	for range manifest.Chunks {
		// read chunk index
		chunkIndex := make([]byte, 8)
		io.ReadFull(conn, chunkIndex)
		index := binary.BigEndian.Uint64(chunkIndex)

		// read chunk size
		chunkSize := make([]byte, 8)
		io.ReadFull(conn, chunkSize)
		size := binary.BigEndian.Uint64(chunkSize)

		// read chunk data
		n, _ := io.ReadFull(conn, bucket[:size])

		// create a new hash
		h := sha256.New()
		if n > 0 {
			// write chunk data into the hasher
			h.Write(bucket[:n])
			hashed := fmt.Sprintf("%x", h.Sum(nil))

			// if hash is equal, then write changes, else discard
			if manifest.Chunks[index].Hash == hashed {
				file.WriteAt(bucket[:n], manifest.Chunks[index].Offset)
				fmt.Printf("chunk %d match, successful\n", index)
			} else {
				fmt.Printf("chunk %d hash mismatch, discarding\n", index)
			}
		}
	}
}
