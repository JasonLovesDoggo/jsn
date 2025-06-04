package merkle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

// FileInfo defines the interface for file information needed by the merkle package
type FileInfo interface {
	GetPath() string
	GetHash() string
	GetSize() int64
	GetMode() fs.FileMode
	GetModTime() time.Time
	GetUID() int
	GetGID() int
}

// Node represents a node in the Merkle tree
type Node struct {
	Hash     [32]byte
	IsLeaf   bool
	Path     string
	Children []*Node
	Parent   *Node
	FileInfo FileInfo
}

// Tree represents a Merkle tree for filesystem integrity
type Tree struct {
	Root  *Node
	Nodes map[string]*Node // Path to node mapping
	Depth int
}

// PathNode represents a path component in the tree
type PathNode struct {
	Name     string
	Children map[string]*PathNode
	Files    []FileInfo
	Hash     [32]byte
}

// New creates a new Merkle tree
func New() *Tree {
	return &Tree{
		Nodes: make(map[string]*Node),
	}
}

// AddFile adds a file record to the tree
func (t *Tree) AddFile(path string, record FileInfo) {
	node := &Node{
		IsLeaf:   true,
		Path:     path,
		FileInfo: record,
	}

	// Calculate leaf hash from file content and metadata
	node.Hash = t.calculateLeafHash(record)
	t.Nodes[path] = node
}

// BuildTree constructs the Merkle tree from all added files
func (t *Tree) BuildTree() [32]byte {
	if len(t.Nodes) == 0 {
		return [32]byte{}
	}

	// Build directory tree structure
	pathTree := t.buildPathTree()

	// Build Merkle tree from path tree
	t.Root = t.buildMerkleNode(pathTree, "")

	// Calculate tree depth
	t.Depth = t.calculateDepth(t.Root)

	return t.Root.Hash
}

// buildPathTree creates a hierarchical representation of the filesystem
func (t *Tree) buildPathTree() *PathNode {
	root := &PathNode{
		Name:     "",
		Children: make(map[string]*PathNode),
		Files:    make([]FileInfo, 0),
	}

	// Add all files to the path tree
	for _, node := range t.Nodes {
		t.addToPathTree(root, node.Path, node.FileInfo)
	}

	return root
}

// addToPathTree adds a file to the hierarchical path tree
func (t *Tree) addToPathTree(root *PathNode, path string, record FileInfo) {
	// Clean and split the path
	cleanPath := strings.Trim(path, "/")
	if cleanPath == "" {
		root.Files = append(root.Files, record)
		return
	}

	parts := strings.Split(cleanPath, "/")
	current := root

	// Navigate/create the path structure
	for i, part := range parts {
		if i == len(parts)-1 {
			// This is the file/final directory
			current.Files = append(current.Files, record)
		} else {
			// This is an intermediate directory
			if current.Children[part] == nil {
				current.Children[part] = &PathNode{
					Name:     part,
					Children: make(map[string]*PathNode),
					Files:    make([]FileInfo, 0),
				}
			}
			current = current.Children[part]
		}
	}
}

// buildMerkleNode recursively builds Merkle tree nodes from path tree
func (t *Tree) buildMerkleNode(pathNode *PathNode, fullPath string) *Node {
	node := &Node{
		Path:     fullPath,
		Children: make([]*Node, 0),
		IsLeaf:   len(pathNode.Children) == 0,
	}

	var hashData []byte

	// Process files in this directory
	if len(pathNode.Files) > 0 {
		// Sort files for consistent hashing
		sort.Slice(pathNode.Files, func(i, j int) bool {
			return pathNode.Files[i].GetPath() < pathNode.Files[j].GetPath()
		})

		for _, file := range pathNode.Files {
			fileHash := t.calculateLeafHash(file)
			hashData = append(hashData, fileHash[:]...)
		}
	}

	// Process subdirectories
	if len(pathNode.Children) > 0 {
		node.IsLeaf = false

		// Sort children for consistent hashing
		childNames := make([]string, 0, len(pathNode.Children))
		for name := range pathNode.Children {
			childNames = append(childNames, name)
		}
		sort.Strings(childNames)

		for _, name := range childNames {
			childPathNode := pathNode.Children[name]
			childPath := fullPath
			if childPath != "" {
				childPath += "/"
			}
			childPath += name

			childNode := t.buildMerkleNode(childPathNode, childPath)
			childNode.Parent = node
			node.Children = append(node.Children, childNode)

			// Add child hash to our hash data
			hashData = append(hashData, childNode.Hash[:]...)
		}
	}

	// Calculate this node's hash
	node.Hash = sha256.Sum256(hashData)

	// Store in nodes map
	if fullPath != "" {
		t.Nodes[fullPath] = node
	}

	return node
}

// calculateLeafHash calculates hash for a file record
func (t *Tree) calculateLeafHash(record FileInfo) [32]byte {
	var hashData []byte

	// Include file path
	hashData = append(hashData, []byte(record.GetPath())...)

	// Include file hash if available
	if record.GetHash() != "" && record.GetHash() != "ERROR" {
		if hashBytes, err := hex.DecodeString(record.GetHash()); err == nil {
			hashData = append(hashData, hashBytes...)
		}
	}

	// Include metadata for integrity
	hashData = append(hashData, []byte(fmt.Sprintf("%d", record.GetSize()))...)
	hashData = append(hashData, []byte(fmt.Sprintf("%d", record.GetMode()))...)
	hashData = append(hashData, []byte(record.GetModTime().Format("2006-01-02T15:04:05Z"))...)

	if record.GetUID() != 0 || record.GetGID() != 0 {
		hashData = append(hashData, []byte(fmt.Sprintf("%d:%d", record.GetUID(), record.GetGID()))...)
	}

	return sha256.Sum256(hashData)
}

// calculateDepth calculates the maximum depth of the tree
func (t *Tree) calculateDepth(node *Node) int {
	if node == nil || node.IsLeaf {
		return 1
	}

	maxChildDepth := 0
	for _, child := range node.Children {
		childDepth := t.calculateDepth(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return maxChildDepth + 1
}

// VerifyIntegrity verifies the integrity of the tree
func (t *Tree) VerifyIntegrity() bool {
	if t.Root == nil {
		return false
	}
	return t.verifyNode(t.Root)
}

// verifyNode recursively verifies a node and its children
func (t *Tree) verifyNode(node *Node) bool {
	if node.IsLeaf {
		// For leaf nodes, verify the hash matches the file record
		if node.FileInfo != nil {
			expectedHash := t.calculateLeafHash(node.FileInfo)
			return node.Hash == expectedHash
		}
		return true
	}

	// For internal nodes, verify hash matches children
	var hashData []byte
	for _, child := range node.Children {
		if !t.verifyNode(child) {
			return false
		}
		hashData = append(hashData, child.Hash[:]...)
	}

	expectedHash := sha256.Sum256(hashData)
	return node.Hash == expectedHash
}

// GetProof generates a Merkle proof for a given file path
func (t *Tree) GetProof(path string) (*MerkleProof, error) {
	node, exists := t.Nodes[path]
	if !exists {
		return nil, fmt.Errorf("path not found in tree: %s", path)
	}

	proof := &MerkleProof{
		Path:     path,
		LeafHash: node.Hash,
		Proof:    make([]ProofElement, 0),
	}

	// Traverse up the tree collecting sibling hashes
	current := node
	for current.Parent != nil {
		parent := current.Parent

		// Find sibling hashes
		for _, sibling := range parent.Children {
			if sibling != current {
				proof.Proof = append(proof.Proof, ProofElement{
					Hash:     sibling.Hash,
					IsLeft:   t.isLeftSibling(sibling, current),
					NodePath: sibling.Path,
				})
			}
		}
		current = parent
	}

	proof.RootHash = t.Root.Hash
	return proof, nil
}

// isLeftSibling determines if a node is to the left of another node
func (t *Tree) isLeftSibling(node1, node2 *Node) bool {
	return strings.Compare(node1.Path, node2.Path) < 0
}

// CompareWith compares this tree with another tree
func (t *Tree) CompareWith(other *Tree) *TreeComparison {
	comparison := &TreeComparison{
		LeftRoot:    t.Root.Hash,
		RightRoot:   other.Root.Hash,
		Differences: make([]PathDifference, 0),
	}

	// Compare all paths
	allPaths := make(map[string]bool)
	for path := range t.Nodes {
		allPaths[path] = true
	}
	for path := range other.Nodes {
		allPaths[path] = true
	}

	for path := range allPaths {
		leftNode, leftExists := t.Nodes[path]
		rightNode, rightExists := other.Nodes[path]

		if !leftExists {
			comparison.Differences = append(comparison.Differences, PathDifference{
				Path:  path,
				Type:  DiffAdded,
				Right: rightNode.Hash,
			})
		} else if !rightExists {
			comparison.Differences = append(comparison.Differences, PathDifference{
				Path: path,
				Type: DiffDeleted,
				Left: leftNode.Hash,
			})
		} else if leftNode.Hash != rightNode.Hash {
			comparison.Differences = append(comparison.Differences, PathDifference{
				Path:  path,
				Type:  DiffModified,
				Left:  leftNode.Hash,
				Right: rightNode.Hash,
			})
		}
	}

	return comparison
}

// MerkleProof represents a proof of inclusion in the Merkle tree
type MerkleProof struct {
	Path     string
	LeafHash [32]byte
	RootHash [32]byte
	Proof    []ProofElement
}

// ProofElement represents one element in a Merkle proof
type ProofElement struct {
	Hash     [32]byte
	IsLeft   bool
	NodePath string
}

// Verify verifies the Merkle proof
func (p *MerkleProof) Verify() bool {
	currentHash := p.LeafHash

	for _, element := range p.Proof {
		var hashData []byte
		if element.IsLeft {
			hashData = append(hashData, element.Hash[:]...)
			hashData = append(hashData, currentHash[:]...)
		} else {
			hashData = append(hashData, currentHash[:]...)
			hashData = append(hashData, element.Hash[:]...)
		}
		currentHash = sha256.Sum256(hashData)
	}

	return currentHash == p.RootHash
}

// TreeComparison represents the result of comparing two Merkle trees
type TreeComparison struct {
	LeftRoot    [32]byte
	RightRoot   [32]byte
	Differences []PathDifference
}

// PathDifference represents a difference between two trees
type PathDifference struct {
	Path  string
	Type  DiffType
	Left  [32]byte
	Right [32]byte
}

// DiffType represents the type of difference
type DiffType int

const (
	DiffAdded DiffType = iota
	DiffDeleted
	DiffModified
)

// String returns string representation of diff type
func (d DiffType) String() string {
	switch d {
	case DiffAdded:
		return "added"
	case DiffDeleted:
		return "deleted"
	case DiffModified:
		return "modified"
	default:
		return "unknown"
	}
}

// PrintTree prints the tree structure for debugging
func (t *Tree) PrintTree() {
	if t.Root == nil {
		fmt.Println("Empty tree")
		return
	}
	t.printNode(t.Root, "", true)
}

// printNode recursively prints a node and its children
func (t *Tree) printNode(node *Node, prefix string, isLast bool) {
	if node == nil {
		return
	}

	// Print current node
	marker := "├── "
	if isLast {
		marker = "└── "
	}

	name := node.Path
	if name == "" {
		name = "/"
	}

	fmt.Printf("%s%s%s (hash: %x)\n", prefix, marker, name, node.Hash[:8])

	// Print children
	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		t.printNode(child, childPrefix, isLastChild)
	}
}
