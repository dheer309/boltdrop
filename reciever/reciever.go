package main

import (
	"boltdrop/chunker"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grandcat/zeroconf"
	"io"
	"net"
	"os"
	"slices"
)

type ResumeState struct {
	ResumeFileName  string
	CompletedChunks []int
}

// 4 mb
var fourMB = 4 << 20

func main() {
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
	manifest, err := readManifest(conn)

	if err != nil {
		return
	}

	// save the resume file's path and the resume state from it
	resumeFilePath := "." + manifest.Filename + ".resume"
	resumeState := loadResumeState(resumeFilePath, manifest.Filename)

	// delete resume file after sending
	defer func() {
		if len(resumeState.CompletedChunks) == len(manifest.Chunks) {
			os.Remove(resumeFilePath)
		}
	}()

	sendResumeState(conn, resumeState)

	// send the actual file
	file, err := os.OpenFile(manifest.Filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	defer file.Close()

	// a 4 mb bucket
	bucket := make([]byte, fourMB)

	// only loop for the quantity of chunks not yet processed
	expectedChunks := len(manifest.Chunks) - len(resumeState.CompletedChunks)

	for range expectedChunks {
		index, n, err := receiveChunk(conn, bucket)

		if err != nil {
			fmt.Println("Error when reading chunk: ", err)
			return
		}

		// NOTE: this is an edge case and may rarely run, as the sender itself skips
		// sending the completed chunks
		if slices.Contains(resumeState.CompletedChunks, int(index)) {
			// if that chunk is already in the resume file, then move on to next chunk
			fmt.Printf("chunk %d already received, skipping\n", index)
			continue
		}

		ok := verifyChunk(manifest, index, n, bucket)

		if ok {
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

func readManifest(conn net.Conn) (chunker.Manifest, error) {
	// defining the header of being 8 bytes long
	header := make([]byte, 8)

	// reading only the first 8 bytes from the connection
	_, err := io.ReadFull(conn, header)

	if err != nil {
		fmt.Println("Cannot fetch header", err)
		return chunker.Manifest{}, err
	}

	// fetching the manifest's actual length from the header
	manifestLength := binary.BigEndian.Uint64(header)

	// reading only the manifest
	manifestJson := make([]byte, manifestLength)
	_, err = io.ReadFull(conn, manifestJson)

	if err != nil {
		fmt.Println("Cannot fetch manifest", err)
		return chunker.Manifest{}, err
	}

	// populating that json data into the manifest data structure
	var manifest chunker.Manifest
	err = json.Unmarshal(manifestJson, &manifest)

	if err != nil {
		fmt.Println("Cannot convert into Manifest struct", err)
		return chunker.Manifest{}, err
	}

	// returning the manifest as there are no errors anymore
	return manifest, nil
}

func receiveChunk(conn net.Conn, bucket []byte) (index uint64, n int, err error) {
	// read chunk index
	chunkIndex := make([]byte, 8)
	_, err = io.ReadFull(conn, chunkIndex)

	// close connection after file transfer complete
	if err != nil {
		fmt.Println("Connection closed or err: ", err)
		return
	}

	// convert chunk index to readable int
	index = binary.BigEndian.Uint64(chunkIndex)

	// read chunk size
	chunkSize := make([]byte, 8)
	_, err = io.ReadFull(conn, chunkSize)

	if err != nil {
		fmt.Println("Error reading chunk size: ", err)
		return
	}

	size := binary.BigEndian.Uint64(chunkSize)

	// unlikely to occur, rare edge case
	if size > uint64(fourMB) {
		fmt.Println("Chunk size exceeds buffer, aborting")
		return
	}

	// read chunk data
	n, err = io.ReadFull(conn, bucket[:size])

	if err != nil {
		fmt.Println("Error reading chunk data: ", err)
		return
	}

	// return the index and number of bytes
	return index, n, err
}

func verifyChunk(manifest chunker.Manifest, index uint64, n int, bucket []byte) bool {
	// create a new hash
	h := sha256.New()

	// write chunk data into the hasher
	h.Write(bucket[:n])
	hashed := fmt.Sprintf("%x", h.Sum(nil))

	// if hash is equal, then write changes, else discard
	if manifest.Chunks[index].Hash == hashed {
		return true
	} else {
		return false
	}
}

func sendResumeState(conn net.Conn, state ResumeState) error {
	// convert the data about completed chunks to json, to send it to the sender
	completedJson, _ := json.Marshal(state.CompletedChunks)

	// create a header and send its size
	recievedChunks := make([]byte, 8) // header is 8 bytes long
	binary.BigEndian.PutUint64(recievedChunks, uint64(len(completedJson)))
	conn.Write(recievedChunks)

	// send the completed chunks json data itself
	conn.Write(completedJson)

	return nil
}

func loadResumeState(resumeFilePath string, filename string) ResumeState {
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

	// make sure the resume file is of the file being received
	if state.ResumeFileName != filename {
		fmt.Println("Resume file mismatch!")
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
