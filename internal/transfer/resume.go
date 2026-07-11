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

func SendResumeState(conn net.Conn, state ResumeState) error {
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
