package main

import (
	"context"
	"fmt"
	"github.com/dheer309/boltdrop/chunker"
	"github.com/dheer309/boltdrop/internal/transfer"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func sendFile(cmd *cobra.Command, args []string) {
	// get file path from user
	filename := args[0]

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

	start := time.Now()

	entries := make(chan *zeroconf.ServiceEntry)
	var found []*zeroconf.ServiceEntry

	go func(results <-chan *zeroconf.ServiceEntry) {
		defer wg.Done()
		for entry := range results {
			if len(found) == 0 {
				fmt.Printf("First device found in: %v\n", time.Since(start))
			}
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

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetWriteBuffer(1024 * 1024)
	}

	// send the manifest to the receiver
	if err = transfer.SendManifest(conn, manifest); err != nil {
		return
	}

	// read data about which chunks are completed
	completedChunks, err := transfer.ReadCompletedChunks(conn)

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
	buf := make([]byte, manifest.ChunkSize)

	for _, chunk := range manifest.Chunks {
		// move file cursor to chunk's starting position
		_, err := file.Seek(chunk.Offset, 0)

		if err != nil {
			fmt.Println("Error moving cursor to chunk offset: ", err)
			return
		}

		// skip sending chunk if already present in the receiver
		if slices.Contains(completedChunks, chunk.Index) {
			continue
		}

		// abort file transfer if any problems in sending the chunk
		if err := transfer.SendChunk(conn, file, buf, chunk, bar); err != nil {
			return
		}
	}

	fmt.Println("\nTransfer complete")
}
