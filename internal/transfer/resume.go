package transfer

import (
	"boltdrop/internal/utils"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type ResumeState struct {
	ResumeFileName  string
	CompletedChunks []int
}

func SendResumeState(conn net.Conn, state ResumeState, totalChunks int) error {
	// make a bitmask with the appropriate length
	bitmask := make([]byte, (totalChunks+7)/8)

	// mark chunk as complete: byte index = chunkIndex/8, bit position = chunkIndex%8
	for _, chunkIndex := range state.CompletedChunks {
		bitmask[chunkIndex/8] |= 1 << (chunkIndex % 8)
	}

	// create a header and send its size
	recievedChunks := make([]byte, 8) // header is 8 bytes long
	binary.BigEndian.PutUint64(recievedChunks, uint64(len(bitmask)))
	conn.Write(recievedChunks)

	// send the bitmask
	conn.Write(bitmask)

	return nil
}

func LoadResumeState(resumeFilePath string, filename string) ResumeState {
	// if file doesn't exist create a new resume state
	if !utils.CheckFileExists(resumeFilePath) {
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
		fmt.Println("Cannot convert into ResumeState struct", err)
		return ResumeState{}
	}

	// make sure the resume file is of the file being received
	if state.ResumeFileName != filename {
		fmt.Println("Resume file mismatch!")
		return ResumeState{}
	}

	return state
}

func SaveResumeState(resumeFilePath string, state ResumeState) error {
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
