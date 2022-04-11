package mpt

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

////////////
// Globals
////////////

var (
	EmptyNodeRaw     = []byte{}
	EmptyNodeHash, _ = hex.DecodeString("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

////////////////////////////
// Node-general definitions
////////////////////////////

type Node interface {
	Hash() []byte // common.Hash
	Raw() []interface{}
}

func Hash(node Node) []byte {
	if IsEmptyNode(node) {
		return EmptyNodeHash
	}
	return node.Hash()
}

func Serialize(node Node) []byte {
	var raw interface{}

	if IsEmptyNode(node) {
		raw = EmptyNodeRaw
	} else {
		raw = node.Raw()
	}

	rlp, err := rlp.EncodeToBytes(raw)
	if err != nil {
		panic(err)
	}

	return rlp
}

func Deserialize(serializedNode []byte, db DB) (Node, error) {
	var rawNode []interface{}
	err := rlp.DecodeBytes(serializedNode, &rawNode)

	if err != nil {
		panic(err)
	}

	return FromRaw(rawNode, db)
}

// TODO [Alice]: explain the difference between a node and a serializedNode.
func FromRaw(node []interface{}, db DB) (Node, error) {
	if len(node) == 0 {
		return nil, fmt.Errorf("serializedNode is empty.")
	}

	if len(node) == 17 {
		////////////////////
		// Is a BranchNode.
		////////////////////
		branchNode := NewBranchNode()

		for i := 0; i < 16; i++ {
			rawBranch := node[i]
			if rawBranchBytes, ok := rawBranch.([]byte); ok {
				if len(rawBranchBytes) != 0 {
					serializedNodeFromDB, err := db.Get(rawBranchBytes)
					// SAFETY: failing to get from database is a fatal error.
					if err != nil {
						panic(err)
					}

					deserializedNode, err := Deserialize(serializedNodeFromDB, db)
					if err != nil {
						return nil, err
					}

					branchNode.branches[i] = deserializedNode
				}
			} else if rawBranchBytes, ok := rawBranch.([]interface{}); ok {
				if len(rawBranchBytes) != 0 {
					deserializedNode, err := FromRaw(rawBranchBytes, db)
					if err != nil {
						return nil, err
					}

					branchNode.branches[i] = deserializedNode
				}
			} else {
				return nil, fmt.Errorf("cannot deserialize node. TODO [Alice]: provide a reason.")
			}
		}

		if value, ok := node[16].([]byte); ok {
			if len(value) == 0 {
				branchNode.value = nil
			} else {
				branchNode.value = value
			}
		} else {
			return nil, fmt.Errorf("cannot deserialize value.")
		}

		return branchNode, nil
	} else {
		// Either extension node or leaf node
		nibbleBytes := node[0]
		prefixedNibbles := NewNibblesFromBytes(nibbleBytes.([]byte))
		nibbles, isLeafNode := RemovePrefixFromNibbles(prefixedNibbles)

		if isLeafNode {
			///////////////////
			// Is a LeafNode.
			///////////////////
			leafNode := NewLeafNodeFromNibbles(nibbles, node[1].([]byte))
			return leafNode, nil

		} else {
			////////////////////////
			// Is an ExtensionNode.
			////////////////////////
			extensionNode := NewExtensionNode(nibbles, nil)
			rawNextNode := node[1]

			if rawNextNodeBytes, ok := rawNextNode.([]byte); ok {
				if len(rawNextNodeBytes) != 0 {
					serializedNodeFromDB, err := db.Get(rawNextNodeBytes)
					// SAFETY: failing to get from database is a fatal error.
					if err != nil {
						panic(err)
					}

					deserializedNode, err := Deserialize(serializedNodeFromDB, db)
					if err != nil {
						return nil, err
					}
					extensionNode.next = deserializedNode
				}
			} else if rawNextNodeBytes, ok := rawNextNode.([]interface{}); ok {
				// raw node itself
				if len(rawNextNodeBytes) != 0 {
					deserializedNode, err := FromRaw(rawNextNodeBytes, db)
					if err != nil {
						return nil, err
					}

					extensionNode.next = deserializedNode
				}
			} else {
				panic("can not deserialize/decode this node")
			}

			return extensionNode, nil
		}
	}
}

//////////////////////////
// Empty node definitions
//////////////////////////

func IsEmptyNode(node Node) bool {
	return node == nil
}

///////////////////////////
// Branch node definitions
///////////////////////////

type BranchNode struct {
	branches [16]Node
	value    []byte
}

func NewBranchNode() *BranchNode {
	return &BranchNode{
		branches: [16]Node{},
	}
}

func (b BranchNode) Hash() []byte {
	return crypto.Keccak256(b.Serialize())
}

func (b *BranchNode) SetBranch(nibble Nibble, node Node) {
	b.branches[int(nibble)] = node
}

func (b *BranchNode) RemoveBranch(nibble Nibble) {
	b.branches[int(nibble)] = nil
}

func (b *BranchNode) SetValue(value []byte) {
	b.value = value
}

func (b *BranchNode) RemoveValue() {
	b.value = nil
}

func (b BranchNode) Raw() []interface{} {
	hashes := make([]interface{}, 17)
	for i := 0; i < 16; i++ {
		if b.branches[i] == nil {
			hashes[i] = EmptyNodeRaw
		} else {
			node := b.branches[i]
			if len(Serialize(node)) >= 32 {
				hashes[i] = node.Hash()
			} else {
				// if node can be serialized to less than 32 bits, then
				// use Serialized directly.
				// it has to be ">=", rather than ">",
				// so that when deserialized, the content can be distinguished
				// by length
				hashes[i] = node.Raw()
			}
		}
	}

	hashes[16] = b.value
	return hashes
}

func (b BranchNode) Serialize() []byte {
	return Serialize(b)
}

func (b BranchNode) HasValue() bool {
	return b.value != nil
}

///////////////////////////////
// Extension node definitions
///////////////////////////////

type ExtensionNode struct {
	path []Nibble
	next Node
}

func NewExtensionNode(nibbles []Nibble, next Node) *ExtensionNode {
	return &ExtensionNode{
		path: nibbles,
		next: next,
	}
}

func (e ExtensionNode) Hash() []byte {
	return crypto.Keccak256(e.Serialize())
}

func (e ExtensionNode) Raw() []interface{} {
	hashes := make([]interface{}, 2)
	hashes[0] = ConvertNibblesToBytes(AppendPrefixToNibbles(e.path, false))
	if len(Serialize(e.next)) >= 32 {
		hashes[1] = e.next.Hash()
	} else {
		hashes[1] = e.next.Raw()
	}
	return hashes
}

func (e ExtensionNode) Serialize() []byte {
	return Serialize(e)
}

//////////////////////////
// Leaf node definitions
//////////////////////////

type LeafNode struct {
	path  []Nibble
	value []byte
}

// TODO [Alice]: Marked for deletion.
func NewLeafNodeFromNibbleBytes(nibbles []byte, value []byte) (*LeafNode, error) {
	ns, err := CastBytesToNibbles(nibbles)
	if err != nil {
		return nil, fmt.Errorf("could not leaf node from nibbles: %w", err)
	}

	return NewLeafNodeFromNibbles(ns, value), nil
}

func NewLeafNodeFromNibbles(nibbles []Nibble, value []byte) *LeafNode {
	return &LeafNode{
		path:  nibbles,
		value: value,
	}
}

// TODO [Alice]: Marked for deletion.
func NewLeafNodeFromBytes(key, value []byte) *LeafNode {
	return NewLeafNodeFromNibbles(NewNibblesFromBytes(key), value)
}

func (l LeafNode) Hash() []byte {
	return crypto.Keccak256(l.Serialize())
}

func (l LeafNode) Raw() []interface{} {
	path := ConvertNibblesToBytes(AppendPrefixToNibbles(l.path, true))
	raw := []interface{}{path, l.value}
	return raw
}

func (l LeafNode) Serialize() []byte {
	return Serialize(l)
}

//////////////////////////
// ProofNode definitions
//////////////////////////

type ProofNode struct {
	path []Nibble
	hash []byte
}
