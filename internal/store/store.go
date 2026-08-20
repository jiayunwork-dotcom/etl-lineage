// Package store provides append-only log and snapshot persistence for lineage graphs.
// The storage layout uses a directory containing:
//   - snapshot.json: the latest full graph snapshot
//   - wal.log: append-only write-ahead log of mutations since the last snapshot
//
// On Open, the store replays the WAL on top of the snapshot to reconstruct current state.
// Compact merges all WAL entries into a new snapshot and truncates the log.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"etl-lineage/internal/graph"
)

const (
	snapshotFile = "snapshot.json"
	walFile      = "wal.log"
)

// OpType identifies a WAL operation.
type OpType string

const (
	OpAddNode    OpType = "add_node"
	OpSetAttr    OpType = "set_attr"
	OpAddEdge    OpType = "add_edge"
	OpRemoveEdge OpType = "rm_edge"
	OpRemoveNode OpType = "rm_node"
)

// WalEntry is a single mutation record in the WAL.
type WalEntry struct {
	Op       OpType          `json:"op"`
	NodeID   string          `json:"node_id,omitempty"`
	NodeAttr *graph.NodeAttr `json:"node_attr,omitempty"`
	From     string          `json:"from,omitempty"`
	To       string          `json:"to,omitempty"`
	EdgeAttr *graph.EdgeAttr `json:"edge_attr,omitempty"`
}

// Store manages persistent lineage graph storage.
type Store struct {
	mu      sync.Mutex
	dir     string
	g       *graph.Graph
	walFd   *os.File
	walSize int64
}

// Open opens or creates a store at the given directory path.
// If the directory does not exist, it is created.
// The graph is reconstructed from snapshot + WAL replay.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir: %w", err)
	}
	s := &Store{dir: dir, g: graph.New()}

	// Load snapshot if present
	snapPath := filepath.Join(dir, snapshotFile)
	if data, err := os.ReadFile(snapPath); err == nil {
		if err := s.g.UnmarshalJSON(data); err != nil {
			return nil, fmt.Errorf("store: load snapshot: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("store: read snapshot: %w", err)
	}

	// Replay WAL
	walPath := filepath.Join(dir, walFile)
	if f, err := os.Open(walPath); err == nil {
		if err := s.replayWAL(f); err != nil {
			f.Close()
			return nil, fmt.Errorf("store: replay wal: %w", err)
		}
		f.Close()
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("store: open wal: %w", err)
	}

	// Open WAL for append
	fd, err := os.OpenFile(walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("store: open wal for append: %w", err)
	}
	info, _ := fd.Stat()
	s.walFd = fd
	if info != nil {
		s.walSize = info.Size()
	}
	return s, nil
}

// replayWAL applies WAL entries from reader onto the in-memory graph.
func (s *Store) replayWAL(r io.Reader) error {
	dec := json.NewDecoder(r)
	for {
		var entry WalEntry
		if err := dec.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			// Truncated entry at end of WAL is tolerated (crash recovery)
			return nil
		}
		if err := s.applyEntry(entry); err != nil {
			// Skip invalid entries during replay (idempotent recovery)
			continue
		}
	}
}

// applyEntry applies a single WAL entry to the in-memory graph.
func (s *Store) applyEntry(e WalEntry) error {
	switch e.Op {
	case OpAddNode:
		if e.NodeAttr != nil {
			s.g.AddNodeWithAttr(e.NodeID, *e.NodeAttr)
		} else {
			s.g.AddNode(e.NodeID)
		}
	case OpSetAttr:
		if e.NodeAttr == nil {
			return errors.New("set_attr requires node_attr")
		}
		return s.g.SetNodeAttr(e.NodeID, *e.NodeAttr)
	case OpAddEdge:
		attr := graph.EdgeAttr{Transform: graph.TransformDirect}
		if e.EdgeAttr != nil {
			attr = *e.EdgeAttr
		}
		return s.g.AddEdgeWithAttr(e.From, e.To, attr)
	case OpRemoveEdge:
		return s.g.RemoveEdge(e.From, e.To)
	case OpRemoveNode:
		return s.g.RemoveNode(e.NodeID)
	default:
		return fmt.Errorf("unknown op: %s", e.Op)
	}
	return nil
}

// appendWAL writes an entry to the WAL file.
func (s *Store) appendWAL(e WalEntry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	n, err := s.walFd.Write(data)
	if err != nil {
		return err
	}
	s.walSize += int64(n)
	return s.walFd.Sync()
}

// AddNode adds a node and persists the operation.
func (s *Store) AddNode(id string, attr *graph.NodeAttr) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.g.HasNode(id) {
		return nil
	}
	entry := WalEntry{Op: OpAddNode, NodeID: id, NodeAttr: attr}
	if err := s.appendWAL(entry); err != nil {
		return fmt.Errorf("store: wal write: %w", err)
	}
	if err := s.applyEntry(entry); err != nil {
		return err
	}
	return nil
}

// SetNodeAttr updates node attributes and persists.
func (s *Store) SetNodeAttr(id string, attr graph.NodeAttr) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := WalEntry{Op: OpSetAttr, NodeID: id, NodeAttr: &attr}
	if err := s.applyEntry(entry); err != nil {
		return err
	}
	return s.appendWAL(entry)
}

// AddEdge adds an edge and persists.
func (s *Store) AddEdge(from, to string, attr *graph.EdgeAttr) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := WalEntry{Op: OpAddEdge, From: from, To: to, EdgeAttr: attr}
	if err := s.applyEntry(entry); err != nil {
		return err
	}
	return s.appendWAL(entry)
}

// RemoveEdge removes an edge and persists.
func (s *Store) RemoveEdge(from, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := WalEntry{Op: OpRemoveEdge, From: from, To: to}
	if err := s.applyEntry(entry); err != nil {
		return err
	}
	return s.appendWAL(entry)
}

// RemoveNode removes a node and persists.
func (s *Store) RemoveNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := WalEntry{Op: OpRemoveNode, NodeID: id}
	if err := s.applyEntry(entry); err != nil {
		return err
	}
	return s.appendWAL(entry)
}

// Graph returns a clone of the current in-memory graph.
func (s *Store) Graph() *graph.Graph {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.g.Clone()
}

// Compact writes a new snapshot and truncates the WAL.
// This is the equivalent of a checkpoint operation.
func (s *Store) Compact() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapPath := filepath.Join(s.dir, snapshotFile)
	tmpPath := snapPath + ".tmp"

	data, err := s.g.MarshalJSON()
	if err != nil {
		return fmt.Errorf("store: marshal snapshot: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("store: write tmp snapshot: %w", err)
	}

	if err := os.Rename(tmpPath, snapPath); err != nil {
		return fmt.Errorf("store: rename snapshot: %w", err)
	}

	// Truncate WAL
	s.walFd.Close()
	walPath := filepath.Join(s.dir, walFile)
	fd, err := os.OpenFile(walPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("store: truncate wal: %w", err)
	}
	s.walFd = fd
	s.walSize = 0
	return nil
}

// WALSize returns the current WAL file size in bytes.
func (s *Store) WALSize() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.walSize
}

// Close closes the store, flushing the WAL.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.walFd != nil {
		return s.walFd.Close()
	}
	return nil
}

// Snapshot returns the raw snapshot bytes (for external backup).
func (s *Store) Snapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.g.MarshalJSON()
}

// LoadSnapshot replaces the current graph with data from a snapshot byte slice,
// then compacts (persists snapshot and truncates WAL).
func (s *Store) LoadSnapshot(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g := graph.New()
	if err := g.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("store: load snapshot: %w", err)
	}
	s.g = g

	// Persist
	snapPath := filepath.Join(s.dir, snapshotFile)
	if err := os.WriteFile(snapPath, data, 0o644); err != nil {
		return fmt.Errorf("store: write snapshot: %w", err)
	}
	// Truncate WAL
	s.walFd.Close()
	walPath := filepath.Join(s.dir, walFile)
	fd, err := os.OpenFile(walPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("store: truncate wal: %w", err)
	}
	s.walFd = fd
	s.walSize = 0
	return nil
}
