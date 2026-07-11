package main

import (
	"boltdrop/chunker"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/schollz/progressbar/v3"
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

	// Discover all services on the network (_boltdrop._tcp)
	resolver, err := zeroconf.NewResolver(nil)

	if err != nil {
		fmt.Println("Failed to initialize resolver:", err.Error())
		return
	}

	// fetch all the ip addresses connected to boltdrop
	var wg sync.WaitGroup
	wg.Add(1)

	fmt.Println("Searching receiver devices...")

	entries := make(chan *zeroconf.ServiceEntry)
	var found []*zeroconf.ServiceEntry

	go func(results <-chan *zeroconf.ServiceEntry) {
		defer wg.Done()
		for entry := range results {
			found = append(found, entry)
		}
	}(entries)

	// stop searching for ip addresses after 10 seconds
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// start browsing for ip address entries
	err = resolver.Browse(ctx, "_boltdrop._tcp", "local.", entries)

	if err != nil {
		fmt.Println("Failed to browse:", err.Error())
		return
	}

	// wait till we are finally done
	<-ctx.Done()
	wg.Wait()

	// no recievers are found
	if len(found) == 0 {
		fmt.Println("No receivers found, quitting")
		return
	}

	// list all the ip addresses found
	for i, entry := range found {
		fmt.Printf("%d: %s:%d\n", i+1, entry.AddrIPv4[0], entry.Port)
	}

	// let user select which ip address to send the file to
	var choice int

	for {
		fmt.Printf("Select receiver between 1 to %d\n", len(found))
		fmt.Scan(&choice)

		if choice >= 1 && choice <= len(found) {
			break
		}

		fmt.Println("Invalid choice, try again!")
	}

	entry := found[choice-1]
	addr := net.JoinHostPort(entry.AddrIPv4[0].String(), fmt.Sprintf("%d", entry.Port))

	// try to connect to selected address through tcp
	conn, err := net.Dial("tcp", addr)

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

	// calculating how many bytes to skip
	var skippedBytes int64
	for _, chunk := range manifest.Chunks {
		if slices.Contains(completedChunks, chunk.Index) {
			skippedBytes += int64(chunk.Size)
		}
	}

	// initialise progress bar
	bar := progressbar.DefaultBytes(manifest.FileSize-skippedBytes, "sending file")

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
			//			fmt.Printf("Chunk %d already received by the reciever, skip \n", chunk.Index)
			continue
		}

		if err := sendChunk(conn, file, buf, chunk, bar); err != nil {
			return
		}

		//		fmt.Printf("Sent chunk %d\n", chunk.Index)
	}

	fmt.Println("\nTransfer complete")
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

func sendChunk(conn net.Conn, file *os.File, buf []byte, chunk chunker.Chunk, bar *progressbar.ProgressBar) error {
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

	// update progress bar
	bar.Add(n)

	return nil
}
