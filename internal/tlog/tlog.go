package tlog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Leaf represents a single entry in the transparency log.
type Leaf struct {
	Index int64  `json:"index"`
	Hash  string `json:"hash"` // hex-encoded SHA-256
	Data  []byte `json:"data"` // the actual data (stamp chain JSON)
}

// Node represents a node in the Merkle tree.
type Node struct {
	Hash  string `json:"hash"`
	Left  string `json:"left,omitempty"`  // hash of left child
	Right string `json:"right,omitempty"` // hash of right child
}

// Proof represents an inclusion proof for a leaf.
type Proof struct {
	LeafIndex int64    `json:"leaf_index"`
	LeafHash  string   `json:"leaf_hash"`
	Hashes    []string `json:"hashes"` // sibling hashes from leaf to root
}

// Log represents the transparency log.
type Log struct {
	Leaves []Leaf `json:"leaves"`
	Root   string `json:"root"` // current root hash
}

var (
	ErrLeafNotFound = errors.New("leaf not found")
	ErrInvalidProof = errors.New("invalid proof")
)

// NewLog creates a new empty transparency log.
func NewLog() *Log {
	return &Log{
		Leaves: []Leaf{},
		Root:   "",
	}
}

// Append adds a new leaf to the log.
func (l *Log) Append(data []byte) Leaf {
	index := int64(len(l.Leaves))
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	leaf := Leaf{
		Index: index,
		Hash:  hashStr,
		Data:  data,
	}

	l.Leaves = append(l.Leaves, leaf)
	l.updateRoot()
	return leaf
}

// GetLeaf retrieves a leaf by index.
func (l *Log) GetLeaf(index int64) (Leaf, error) {
	if index < 0 || index >= int64(len(l.Leaves)) {
		return Leaf{}, ErrLeafNotFound
	}
	return l.Leaves[index], nil
}

// Size returns the number of leaves in the log.
func (l *Log) Size() int64 {
	return int64(len(l.Leaves))
}

// GetRoot returns the current root hash.
func (l *Log) GetRoot() string {
	return l.Root
}

// GetProof generates an inclusion proof for a leaf at the given index.
func (l *Log) GetProof(index int64) (Proof, error) {
	if index < 0 || index >= int64(len(l.Leaves)) {
		return Proof{}, ErrLeafNotFound
	}

	proof := Proof{
		LeafIndex: index,
		LeafHash:  l.Leaves[index].Hash,
		Hashes:    []string{},
	}

	// Build proof by walking up the tree
	size := int64(len(l.Leaves))
	idx := index
	level := 0

	for size > 1 {
		// Determine sibling
		var siblingIdx int64
		if idx%2 == 0 {
			siblingIdx = idx + 1
		} else {
			siblingIdx = idx - 1
		}

		// Get sibling hash
		var siblingHash string
		if siblingIdx < size {
			siblingHash = l.getNodeHashAtLevel(siblingIdx, level)
		} else {
			// No sibling, use empty hash
			siblingHash = ""
		}

		if siblingHash != "" {
			proof.Hashes = append(proof.Hashes, siblingHash)
		}

		// Move to parent
		idx = idx / 2
		size = (size + 1) / 2
		level++
	}

	return proof, nil
}

// VerifyProof verifies an inclusion proof against a root hash.
func VerifyProof(proof Proof, root string) bool {
	currentHash := proof.LeafHash
	idx := proof.LeafIndex

	for _, siblingHash := range proof.Hashes {
		var combined []byte
		if idx%2 == 0 {
			// Current is left child
			combined = append([]byte(currentHash), []byte(siblingHash)...)
		} else {
			// Current is right child
			combined = append([]byte(siblingHash), []byte(currentHash)...)
		}

		hash := sha256.Sum256(combined)
		currentHash = hex.EncodeToString(hash[:])
		idx = idx / 2
	}

	return currentHash == root
}

// updateRoot recalculates the root hash from all leaves.
func (l *Log) updateRoot() {
	if len(l.Leaves) == 0 {
		l.Root = ""
		return
	}

	// Start with leaf hashes
	hashes := make([]string, len(l.Leaves))
	for i, leaf := range l.Leaves {
		hashes[i] = leaf.Hash
	}

	// Build tree bottom-up
	for len(hashes) > 1 {
		var nextLevel []string
		for i := 0; i < len(hashes); i += 2 {
			if i+1 < len(hashes) {
				// Combine two nodes
				combined := append([]byte(hashes[i]), []byte(hashes[i+1])...)
				hash := sha256.Sum256(combined)
				nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
			} else {
				// Odd node, promote to next level
				nextLevel = append(nextLevel, hashes[i])
			}
		}
		hashes = nextLevel
	}

	l.Root = hashes[0]
}

// getNodeHashAtLevel gets the hash of a node at a specific level.
// Level 0 is the leaf level.
func (l *Log) getNodeHashAtLevel(index int64, level int) string {
	if level == 0 {
		// Leaf level
		if index < int64(len(l.Leaves)) {
			return l.Leaves[index].Hash
		}
		return ""
	}

	// For higher levels, we need to compute the hash
	// This is a simplified version - in production, you'd cache these
	size := int64(len(l.Leaves))
	for i := 0; i < level; i++ {
		size = (size + 1) / 2
	}

	if index >= size {
		return ""
	}

	// Recompute by building up from leaves
	// This is inefficient but correct - in production, cache intermediate nodes
	return l.computeNodeHash(index, level)
}

// computeNodeHash computes the hash of a node at a given level.
func (l *Log) computeNodeHash(index int64, level int) string {
	if level == 0 {
		if index < int64(len(l.Leaves)) {
			return l.Leaves[index].Hash
		}
		return ""
	}

	// Get children
	leftIdx := index * 2
	rightIdx := index*2 + 1

	leftHash := l.computeNodeHash(leftIdx, level-1)
	rightHash := l.computeNodeHash(rightIdx, level-1)

	if leftHash == "" && rightHash == "" {
		return ""
	}
	if rightHash == "" {
		return leftHash // Promote left child
	}

	// Combine
	combined := append([]byte(leftHash), []byte(rightHash)...)
	hash := sha256.Sum256(combined)
	return hex.EncodeToString(hash[:])
}
