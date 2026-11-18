package tlog

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestLogAppendAndGetLeaf(t *testing.T) {
	log := NewLog()

	// Append some data
	data1 := []byte("stamp chain 1")
	data2 := []byte("stamp chain 2")
	data3 := []byte("stamp chain 3")

	leaf1 := log.Append(data1)
	leaf2 := log.Append(data2)
	leaf3 := log.Append(data3)

	// Check indices
	if leaf1.Index != 0 {
		t.Errorf("leaf1 index = %d, want 0", leaf1.Index)
	}
	if leaf2.Index != 1 {
		t.Errorf("leaf2 index = %d, want 1", leaf2.Index)
	}
	if leaf3.Index != 2 {
		t.Errorf("leaf3 index = %d, want 2", leaf3.Index)
	}

	// Check hashes
	hash1 := sha256.Sum256(data1)
	expected1 := hex.EncodeToString(hash1[:])
	if leaf1.Hash != expected1 {
		t.Errorf("leaf1 hash = %s, want %s", leaf1.Hash, expected1)
	}

	// Check size
	if log.Size() != 3 {
		t.Errorf("log size = %d, want 3", log.Size())
	}

	// Get leaf
	retrieved, err := log.GetLeaf(1)
	if err != nil {
		t.Fatalf("GetLeaf(1) error = %v", err)
	}
	if retrieved.Index != 1 {
		t.Errorf("retrieved index = %d, want 1", retrieved.Index)
	}
	if string(retrieved.Data) != "stamp chain 2" {
		t.Errorf("retrieved data = %s, want 'stamp chain 2'", string(retrieved.Data))
	}

	// Get non-existent leaf
	_, err = log.GetLeaf(10)
	if err != ErrLeafNotFound {
		t.Errorf("GetLeaf(10) error = %v, want ErrLeafNotFound", err)
	}
}

func TestLogRoot(t *testing.T) {
	log := NewLog()

	// Empty log
	if log.GetRoot() != "" {
		t.Errorf("empty log root = %s, want empty", log.GetRoot())
	}

	// Single leaf
	log.Append([]byte("leaf 0"))
	root1 := log.GetRoot()
	if root1 == "" {
		t.Error("single leaf root should not be empty")
	}

	// Two leaves
	log.Append([]byte("leaf 1"))
	root2 := log.GetRoot()
	if root2 == root1 {
		t.Error("root should change after adding second leaf")
	}

	// Three leaves
	log.Append([]byte("leaf 2"))
	root3 := log.GetRoot()
	if root3 == root2 {
		t.Error("root should change after adding third leaf")
	}
}

func TestLogProof(t *testing.T) {
	log := NewLog()

	// Add several leaves
	for i := 0; i < 8; i++ {
		log.Append([]byte{byte(i)})
	}

	// Get proof for leaf 3
	proof, err := log.GetProof(3)
	if err != nil {
		t.Fatalf("GetProof(3) error = %v", err)
	}

	if proof.LeafIndex != 3 {
		t.Errorf("proof leaf index = %d, want 3", proof.LeafIndex)
	}

	if proof.LeafHash == "" {
		t.Error("proof leaf hash should not be empty")
	}

	// Verify proof
	root := log.GetRoot()
	if !VerifyProof(proof, root) {
		t.Error("proof verification failed")
	}

	// Tamper with proof
	tamperedProof := proof
	tamperedProof.LeafHash = "0000000000000000000000000000000000000000000000000000000000000000"
	if VerifyProof(tamperedProof, root) {
		t.Error("tampered proof should not verify")
	}
}

func TestLogProofSingleLeaf(t *testing.T) {
	log := NewLog()
	log.Append([]byte("only leaf"))

	proof, err := log.GetProof(0)
	if err != nil {
		t.Fatalf("GetProof(0) error = %v", err)
	}

	if len(proof.Hashes) != 0 {
		t.Errorf("single leaf proof should have no hashes, got %d", len(proof.Hashes))
	}

	root := log.GetRoot()
	if !VerifyProof(proof, root) {
		t.Error("single leaf proof should verify")
	}
}

func TestLogProofTwoLeaves(t *testing.T) {
	log := NewLog()
	log.Append([]byte("leaf 0"))
	log.Append([]byte("leaf 1"))

	// Proof for leaf 0
	proof0, err := log.GetProof(0)
	if err != nil {
		t.Fatalf("GetProof(0) error = %v", err)
	}

	root := log.GetRoot()
	if !VerifyProof(proof0, root) {
		t.Error("proof for leaf 0 should verify")
	}

	// Proof for leaf 1
	proof1, err := log.GetProof(1)
	if err != nil {
		t.Fatalf("GetProof(1) error = %v", err)
	}

	if !VerifyProof(proof1, root) {
		t.Error("proof for leaf 1 should verify")
	}
}

func TestLogDeterministic(t *testing.T) {
	// Two logs with same data should have same root
	log1 := NewLog()
	log2 := NewLog()

	data := [][]byte{
		[]byte("stamp 1"),
		[]byte("stamp 2"),
		[]byte("stamp 3"),
	}

	for _, d := range data {
		log1.Append(d)
		log2.Append(d)
	}

	if log1.GetRoot() != log2.GetRoot() {
		t.Error("logs with same data should have same root")
	}
}
