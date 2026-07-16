package transfer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/dheer309/boltdrop/chunker"
	"io"
	"net"
)

func ReadManifest(conn net.Conn) (chunker.Manifest, error) {
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

func SendManifest(conn net.Conn, manifest chunker.Manifest) error {
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

func ReadCompletedChunks(conn net.Conn) ([]int, error) {
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
	bitmask := make([]byte, completedChunkLength)
	_, err = io.ReadFull(conn, bitmask)

	if err != nil {
		fmt.Println("Cannot fetch completed chunk data", err)
		return nil, err
	}

	// converting the json back to int list
	var completedChunks []int

	// for every byte, loop through every bit to determine which chunk is received already
	for byteIndex, b := range bitmask {
		for bitPos := range 8 {
			if b&(1<<bitPos) != 0 {
				completedChunks = append(completedChunks, byteIndex*8+bitPos)
			}
		}
	}

	return completedChunks, nil
}
