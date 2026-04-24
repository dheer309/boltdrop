package chunker

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

type Chunk struct {
	Index  int
	Offset int64
	Size   int
	Hash   string
}

type Manifest struct {
	Filename  string
	FileSize  int64
	ChunkSize int
	Chunks    []Chunk
}

func GenerateManifest(filepath string) (Manifest, error) {
	var fourMB = 4 << 20 // 4 MB in go

	// open the file and handle all errors
	file, err := os.Open(filepath)

	if err != nil {
		return Manifest{}, err
	}
	defer file.Close() // close file after doing all work

	// get some file statistics
	filestats, err := file.Stat()

	// if error in grabbing file statistics
	if err != nil {
		return Manifest{}, err
	}

	// get file size
	filesize := filestats.Size()

	// fill the file manifest with basic info,
	// and initialise empty chunk list
	manifest := Manifest{
		Filename:  filestats.Name(),
		FileSize:  filesize,
		ChunkSize: fourMB,
		Chunks:    []Chunk{},
	}

	// initialise the sha256 hashing
	h := sha256.New()

	// create a single 4mb bucket
	bucket := make([]byte, fourMB)

	// initialise the offset (later to be filled in with chunk)
	offset := int64(0)

	for i := 0; ; i++ {
		// read 4mb from a file
		// NOTE: it's made sure that it won't start over, there's an invisible
		// "cursor" mechanism implemented by this library
		n, err := file.Read(bucket)

		// if there's actual content in the buffer, hash it and populate the chunk
		if n > 0 {
			h.Reset() // imp: reset so that it reads the next 4mb of data
			h.Write(bucket[:n])
			hashSum := fmt.Sprintf("%x", h.Sum(nil))

			// populate the info about the chunk
			currentChunk := Chunk{
				Index:  i,
				Offset: offset,
				Size:   n,
				Hash:   hashSum,
			}

			// add that chunk to the chunks list in the manifest
			manifest.Chunks = append(manifest.Chunks, currentChunk)
		}

		// if the file is fully processed, exit the loop
		if err == io.EOF {
			break
		}

		// notify if error when reading the file
		if err != nil {
			return manifest, err
		}

		// update the new offset after reading and processing 4mb
		offset += int64(n)
	}

	return manifest, nil
}
