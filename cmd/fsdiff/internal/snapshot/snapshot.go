package snapshot

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"os"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/merkle"
	"pkg.jsn.cam/jsn/cmd/fsdiff/pkg/data"
)

// Save saves a snapshot to disk with compression
func Save(snapshot *data.Snapshot, filename string) error {
	snapshot.Version = data.SnapshotVersion

	// Create the file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer file.Close()

	// Create gzip writer for compression
	gzWriter, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %v", err)
	}
	defer gzWriter.Close()

	// Set gzip header metadata
	gzWriter.Name = filename
	gzWriter.Comment = fmt.Sprintf("fsdiff snapshot v%s - %s",
		data.SnapshotVersion, snapshot.SystemInfo.Hostname)
	gzWriter.ModTime = time.Now()

	// Encode the snapshot
	encoder := gob.NewEncoder(gzWriter)
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to encode snapshot: %v", err)
	}

	// Ensure all data is written
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %v", err)
	}

	// Get final file size
	stat, err := file.Stat()
	if err == nil {
		compressionRatio := float64(stat.Size()) / float64(snapshot.Stats.TotalSize) * 100
		fmt.Printf("💾 Snapshot saved: %s (%.1f MB, %.1f%% compression)\n",
			filename, float64(stat.Size())/1024/1024, compressionRatio)
	}

	return nil
}

// Load loads a snapshot from disk
func Load(filename string) (*data.Snapshot, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot file: %v", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	// Decode the snapshot
	decoder := gob.NewDecoder(gzReader)
	var snapshot data.Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %v", err)
	}

	// Rebuild Merkle tree if needed
	if snapshot.Tree == nil {
		fmt.Printf("🌳 Rebuilding Merkle tree...\n")
		snapshot.Tree = merkle.New()
		for path, record := range snapshot.Files {
			snapshot.Tree.AddFile(path, record)
		}
		root := snapshot.Tree.BuildTree()

		// Verify the root matches
		if root != snapshot.MerkleRoot {
			fmt.Printf("⚠️  Warning: Merkle root mismatch (expected %x, got %x)\n",
				snapshot.MerkleRoot[:8], root[:8])
		}
	}

	fmt.Printf("📖 Loaded snapshot: %s (%s) - %d files, %d dirs\n",
		snapshot.SystemInfo.Hostname,
		snapshot.SystemInfo.Timestamp.Format("2006-01-02 15:04:05"),
		snapshot.Stats.FileCount,
		snapshot.Stats.DirCount)

	return &snapshot, nil
}

// LoadHeader loads only the header information from a snapshot
func LoadHeader(filename string) (*data.SnapshotHeader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot file: %v", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	// Try to read just enough to get the header
	decoder := gob.NewDecoder(gzReader)
	var snapshot data.Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot header: %v", err)
	}

	header := &data.SnapshotHeader{
		Version:    snapshot.Version,
		SystemInfo: snapshot.SystemInfo,
		Stats:      snapshot.Stats,
		MerkleRoot: snapshot.MerkleRoot,
		Created:    snapshot.SystemInfo.Timestamp,
	}

	return header, nil
}
