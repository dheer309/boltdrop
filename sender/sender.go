package main

import (
	"boltdrop/chunker"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Enter filename when running the program")
		return
	}

	var fourMB = 4 << 20

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

	// try to connect to localhost thru tcp
	conn, err := net.Dial("tcp", "localhost:8000")

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer conn.Close()

	// send the manifest to the receiver
	if err = sendManifest(conn, manifest); err != nil {
		return
	}

	// read data about which chunks are completed
	completedChunks, err := readCompletedChunks(conn)

	if err != nil {
		return
	}

	// send the individual chunks one-by-one
	buf := make([]byte, fourMB)

	for _, chunk := range manifest.Chunks {
		// move file cursor to chunk's starting position
		_, err := file.Seek(chunk.Offset, 0)

		if err != nil {
			fmt.Println("Error moving cursor to chunk offset: ", err)
			return
		}

		// skip sending chunk if already present in the receiver
		if slices.Contains(completedChunks, chunk.Index) {
			fmt.Printf("Chunk %d already received by the reciever, skip \n", chunk.Index)
			continue
		}

		if err := sendChunk(conn, file, buf, chunk); err != nil {
			return
		}

		fmt.Printf("Sent chunk %d\n", chunk.Index)
	}

	fmt.Println("Transfer complete")
}

func sendManifest(conn net.Conn, manifest chunker.Manifest) error {
	// convert the manifest in JSON
	manifestJson, err := json.Marshal(manifest)

	if err != nil {
		fmt.Println("Error when converting manifest to JSON:", err)
		return err
	}

	// create a header and send the manifest's length through it
	header := make([]byte, 8) // header is 8 bytes long
	binary.BigEndian.PutUint64(header, uint64(len(manifestJson)))
	conn.Write(header)

	// after sending the header, send the manifest json itself
	n, err := conn.Write(manifestJson)

	if err != nil {
		fmt.Printf("Error while sending manifest of %d bytes: %v", n, err)
		return err
	}

	// no errors
	return nil
}

func readCompletedChunks(conn net.Conn) ([]int, error) {
	// read the 8 byte header first
	completedHeader := make([]byte, 8)
	_, err := io.ReadFull(conn, completedHeader)

	if err != nil {
		fmt.Println("Cannot fetch completed chunk header", err)
		return nil, err
	}

	// decode the completed chunk length from the header
	completedChunkLength := binary.BigEndian.Uint64(completedHeader)

	// reading only the completed chunk data
	completedChunksJson := make([]byte, completedChunkLength)
	_, err = io.ReadFull(conn, completedChunksJson)

	if err != nil {
		fmt.Println("Cannot fetch completed chunk data", err)
		return nil, err
	}

	// converting the json back to int list
	var completedChunks []int
	err = json.Unmarshal(completedChunksJson, &completedChunks)

	if err != nil {
		fmt.Println("Cannot convert completed chunk data", err)
		return nil, err
	}

	return completedChunks, nil
}

func sendChunk(conn net.Conn, file *os.File, buf []byte, chunk chunker.Chunk) error {
	// send chunk index
	indexHeader := make([]byte, 8)
	binary.BigEndian.PutUint64(indexHeader, uint64(chunk.Index))
	conn.Write(indexHeader)

	// send chunk size
	sizeHeader := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeHeader, uint64(chunk.Size))
	conn.Write(sizeHeader)

	// send chunk data
	n, err := io.ReadFull(file, buf[:chunk.Size])

	if err != nil {
		fmt.Println("Error while reading chunk data")
		return err
	}

	_, err = conn.Write(buf[:n])

	if err != nil {
		fmt.Println("Error while sending chunk data")
		return err
	}

	return nil
}
