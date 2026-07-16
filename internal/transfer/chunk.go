package transfer

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/dheer309/boltdrop/chunker"
	"github.com/schollz/progressbar/v3"
	"io"
	"net"
	"os"
)

func SendChunk(conn net.Conn, file *os.File, buf []byte, chunk chunker.Chunk, bar *progressbar.ProgressBar) error {
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

func ReceiveChunk(conn net.Conn, bucket []byte) (index uint64, n int, err error) {
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
	if size > uint64(cap(bucket)) {
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

func VerifyChunk(manifest chunker.Manifest, index uint64, n int, bucket []byte) bool {
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
