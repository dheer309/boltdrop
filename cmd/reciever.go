package main

import (
	"boltdrop/internal/transfer"
	"fmt"
	"net"
	"os"
	"slices"

	"github.com/grandcat/zeroconf"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func receiveFile(cmd *cobra.Command, args []string) {
	// broadcast receiver's ip to anyone looking for boltdrop
	server, err := zeroconf.Register("boltdrop", "_boltdrop._tcp", "local.", 8000, nil, nil)
	if err != nil {
		panic(err)
	}
	defer server.Shutdown()

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

	// get the manifest data
	manifest, err := transfer.ReadManifest(conn)

	if err != nil {
		return
	}

	// save the resume file's path and the resume state from it
	resumeFilePath := "." + manifest.Filename + ".resume"
	resumeState := transfer.LoadResumeState(resumeFilePath, manifest.Filename)

	// delete resume file after sending
	defer func() {
		if len(resumeState.CompletedChunks) == len(manifest.Chunks) {
			os.Remove(resumeFilePath)
		}
	}()

	transfer.SendResumeState(conn, resumeState)

	// send the actual file
	file, err := os.OpenFile(manifest.Filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	defer file.Close()

	// a 4 mb bucket
	bucket := make([]byte, manifest.ChunkSize)

	// calculate remaining size
	alreadyReceived := int64(len(resumeState.CompletedChunks)) * int64(manifest.ChunkSize)
	remainingSize := manifest.FileSize - alreadyReceived

	// initialise progress bar
	bar := progressbar.DefaultBytes(remainingSize, "receiving file")

	// only loop for the quantity of chunks not yet processed
	expectedChunks := len(manifest.Chunks) - len(resumeState.CompletedChunks)

	for range expectedChunks {
		index, n, err := transfer.ReceiveChunk(conn, bucket)

		if err != nil {
			fmt.Println("Error when reading chunk: ", err)
			return
		}

		// NOTE: this is an edge case and may rarely run, as the sender itself skips
		// sending the completed chunks
		if slices.Contains(resumeState.CompletedChunks, int(index)) {
			// if that chunk is already in the resume file, then move on to next chunk
			//fmt.Printf("chunk %d already received, skipping\n", index)
			continue
		}

		ok := transfer.VerifyChunk(manifest, index, n, bucket)

		if ok {
			file.WriteAt(bucket[:n], manifest.Chunks[index].Offset)

			// update progress bar
			bar.Add(n)

			resumeState.CompletedChunks = append(resumeState.CompletedChunks, int(index))
			resumeState.ResumeFileName = manifest.Filename
			transfer.SaveResumeState(resumeFilePath, resumeState)

			//fmt.Printf("chunk %d match, successful\n", index)
		} else {
			//fmt.Printf("chunk %d hash mismatch, discarding\n", index)
		}
	}

	fmt.Println("\nFile received")
}
