package main

import (
	"boltdrop/chunker"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
)

type ResumeState struct {
	ResumeFileName  string
	CompletedChunks []int
}

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

	resumeFilePath := "." + manifest.Filename + ".resume"
	resumeState := loadResumeState(resumeFilePath)

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

		// if that chunk is already in the resume file, then move on to next chunk
		if slices.Contains(resumeState.CompletedChunks, int(index)) {
			fmt.Printf("chunk %d already received, skipping\n", index)
			continue
		}

		// create a new hash
		h := sha256.New()
		if n > 0 {
			// write chunk data into the hasher
			h.Write(bucket[:n])
			hashed := fmt.Sprintf("%x", h.Sum(nil))

			// if hash is equal, then write changes, else discard
			if manifest.Chunks[index].Hash == hashed {
				file.WriteAt(bucket[:n], manifest.Chunks[index].Offset)

				resumeState.CompletedChunks = append(resumeState.CompletedChunks, int(index))
				resumeState.ResumeFileName = manifest.Filename
				saveResumeState(resumeFilePath, resumeState)

				fmt.Printf("chunk %d match, successful\n", index)
			} else {
				fmt.Printf("chunk %d hash mismatch, discarding\n", index)
			}
		}
	}
}

func loadResumeState(resumeFilePath string) ResumeState {
	// if file doesn't exist create a new resume state
	if !checkFileExists(resumeFilePath) {
		return ResumeState{}
	}

	// file exists, read it
	data, err := os.ReadFile(resumeFilePath)

	if err != nil {
		fmt.Println("Coudln't read resume file")
		return ResumeState{}
	}

	// save the resume state from the file
	var state ResumeState
	err = json.Unmarshal(data, &state)

	if err != nil {
		fmt.Println("Cannot convert into Manifest struct", err)
		return ResumeState{}
	}

	return state
}

func saveResumeState(resumeFilePath string, state ResumeState) error {
	// convert the ResumeState to json
	stateJson, err := json.Marshal(state)

	if err != nil {
		fmt.Println("Error when converting state to JSON:", err)
		return err
	}

	// write that json data to .resume
	os.WriteFile(resumeFilePath, stateJson, 0644)

	return nil // no errors
}

// a function which checks if the given file path exists
func checkFileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	//return !os.IsNotExist(err)
	return !errors.Is(error, os.ErrNotExist)
}
