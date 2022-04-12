package mpt

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func hexEqual(t *testing.T, hex string, bytes []byte) {
	require.Equal(t, hex, fmt.Sprintf("%x", bytes))
}

// check basic key-value mapping
func TestGetPut(t *testing.T) {
	t.Run("should get nothing if key does not exist", func(t *testing.T) {
		trie := NewTrie(MODE_NORMAL)
		value := trie.Get([]byte("notexist"))
		require.Nil(t, value)
	})

	t.Run("should get value if key exist", func(t *testing.T) {
		trie := NewTrie(MODE_NORMAL)
		trie.Put([]byte{1, 2, 3, 4}, []byte("hello"))
		value := trie.Get([]byte{1, 2, 3, 4})
		require.Equal(t, value, []byte("hello"))
	})

	t.Run("should get updated value", func(t *testing.T) {
		trie := NewTrie(MODE_NORMAL)
		trie.Put([]byte{1, 2, 3, 4}, []byte("hello"))
		trie.Put([]byte{1, 2, 3, 4}, []byte("world"))
		value := trie.Get([]byte{1, 2, 3, 4})
		require.Equal(t, value, []byte("world"))
	})
}

// verify data integrity
func TestDataIntegrity(t *testing.T) {
	t.Run("should get a different hash if a new key-value pair was added or updated", func(t *testing.T) {
		trie := NewTrie(MODE_NORMAL)
		hash0 := trie.Hash()

		trie.Put([]byte{1, 2, 3, 4}, []byte("hello"))
		hash1 := trie.Hash()

		trie.Put([]byte{1, 2}, []byte("world"))
		hash2 := trie.Hash()

		trie.Put([]byte{1, 2}, []byte("trie"))
		hash3 := trie.Hash()

		require.NotEqual(t, hash0, hash1)
		require.NotEqual(t, hash1, hash2)
		require.NotEqual(t, hash2, hash3)
	})

	t.Run("should get the same hash if two tries have the identicial key-value pairs", func(t *testing.T) {
		trie1 := NewTrie(MODE_NORMAL)
		trie1.Put([]byte{1, 2, 3, 4}, []byte("hello"))
		trie1.Put([]byte{1, 2}, []byte("world"))

		trie2 := NewTrie(MODE_NORMAL)
		trie2.Put([]byte{1, 2, 3, 4}, []byte("hello"))
		trie2.Put([]byte{1, 2}, []byte("world"))

		require.Equal(t, trie1.Hash(), trie2.Hash())
	})
}

func TestPut2Pairs(t *testing.T) {
	trie := NewTrie(MODE_NORMAL)
	trie.Put([]byte{1, 2, 3, 4}, []byte("verb"))
	trie.Put([]byte{1, 2, 3, 4, 5, 6}, []byte("coin"))

	verb := trie.Get([]byte{1, 2, 3, 4})
	require.Equal(t, []byte("verb"), verb)

	coin := trie.Get([]byte{1, 2, 3, 4, 5, 6})
	require.Equal(t, []byte("coin"), coin)

	fmt.Printf("%T\n", trie.root)
	ext, ok := trie.root.(*ExtensionNode)
	require.True(t, ok)
	branch, ok := ext.next.(*BranchNode)
	require.True(t, ok)
	leaf, ok := branch.branches[0].(*LeafNode)
	require.True(t, ok)

	hexEqual(t, "c37ec985b7a88c2c62beb268750efe657c36a585beb435eb9f43b839846682ce", leaf.asHash())
	hexEqual(t, "ddc882350684636f696e8080808080808080808080808080808476657262", branch.asSerialBytes())
	hexEqual(t, "d757709f08f7a81da64a969200e59ff7e6cd6b06674c3f668ce151e84298aa79", branch.asHash())
	hexEqual(t, "64d67c5318a714d08de6958c0e63a05522642f3f1087c6fd68a97837f203d359", ext.asHash())
}

func TestPut(t *testing.T) {
	trie := NewTrie(MODE_NORMAL)
	require.Equal(t, nilNodeHash, trie.Hash())
	trie.Put([]byte{1, 2, 3, 4}, []byte("hello"))
	ns := NewLeafNodeFromBytes([]byte{1, 2, 3, 4}, []byte("hello"))
	require.Equal(t, ns.asHash(), trie.Hash())
}

func TestPutLeafShorter(t *testing.T) {
	trie := NewTrie(MODE_NORMAL)
	trie.Put([]byte{1, 2, 3, 4}, []byte("hello"))
	trie.Put([]byte{1, 2, 3}, []byte("world"))

	leaf := NewLeafNodeFromNibbles([]Nibble{4}, []byte("hello"))

	branch := NewBranchNode()
	branch.setBranch(Nibble(0), leaf)
	branch.setValue([]byte("world"))

	ext := NewExtensionNode([]Nibble{0, 1, 0, 2, 0, 3}, branch)

	require.Equal(t, ext.asHash(), trie.Hash())
}

func TestPutLeafAllMatched(t *testing.T) {
	trie := NewTrie(MODE_NORMAL)
	trie.Put([]byte{1, 2, 3, 4}, []byte("hello"))
	trie.Put([]byte{1, 2, 3, 4}, []byte("world"))

	ns := NewLeafNodeFromBytes([]byte{1, 2, 3, 4}, []byte("world"))
	require.Equal(t, ns.asHash(), trie.Hash())
}

func TestPutLeafMore(t *testing.T) {
	trie := NewTrie(MODE_NORMAL)
	trie.Put([]byte{1, 2, 3, 4}, []byte("hello"))
	trie.Put([]byte{1, 2, 3, 4, 5, 6}, []byte("world"))

	leaf := NewLeafNodeFromNibbles([]Nibble{5, 0, 6}, []byte("world"))

	branch := NewBranchNode()
	branch.setValue([]byte("hello"))
	branch.setBranch(Nibble(0), leaf)

	ext := NewExtensionNode([]Nibble{0, 1, 0, 2, 0, 3, 0, 4}, branch)

	require.Equal(t, ext.asHash(), trie.Hash())
}

func TestPutOrder(t *testing.T) {
	trie1, trie2 := NewTrie(MODE_NORMAL), NewTrie(MODE_NORMAL)

	trie1.Put([]byte{1, 2, 3, 4, 5, 6}, []byte("world"))
	trie1.Put([]byte{1, 2, 3, 4}, []byte("hello"))

	trie2.Put([]byte{1, 2, 3, 4}, []byte("hello"))
	trie2.Put([]byte{1, 2, 3, 4, 5, 6}, []byte("world"))

	require.Equal(t, trie1.Hash(), trie2.Hash())
}

func TestPersistInDB(t *testing.T) {
	trie := NewTrie(MODE_NORMAL)

	trie.Put([]byte{1, 2, 3, 4}, []byte("verb"))
	trie.Put([]byte{1, 2, 3, 4, 5, 6}, []byte("coin"))

	mockDB := NewMockDB()

	trie.SaveToDB(mockDB)

	hexEqual(t, "64d67c5318a714d08de6958c0e63a05522642f3f1087c6fd68a97837f203d359", crypto.Keccak256(mockDB.keyValueStore[fmt.Sprintf("%x", "root")]))

	ext, ok := trie.root.(*ExtensionNode)
	require.True(t, ok)
	branch, ok := ext.next.(*BranchNode)
	require.True(t, ok)
	leaf, ok := branch.branches[0].(*LeafNode)
	require.True(t, ok)

	expectedKeyValueStore := map[string][]byte{
		fmt.Sprintf("%x", "root"):          ext.asSerialBytes(),
		fmt.Sprintf("%x", branch.asHash()): branch.asSerialBytes(),
		fmt.Sprintf("%x", leaf.asHash()):   leaf.asSerialBytes(),
	}

	require.True(t, reflect.DeepEqual(expectedKeyValueStore, mockDB.keyValueStore))
}

func TestGenerateFromDB(t *testing.T) {
	trie := NewTrie(MODE_NORMAL)

	trie.Put([]byte{1, 2, 3, 4}, []byte("verb"))
	trie.Put([]byte{1, 2, 3, 4, 5, 6}, []byte("coin"))
	trie.Put([]byte{1, 2, 3, 10}, []byte("crash"))

	mockDB := NewMockDB()

	trie.SaveToDB(mockDB)

	newTrie := NewTrie(MODE_NORMAL)
	newTrie.LoadFromDB(mockDB)
	require.Equal(t, trie.root.asHash(), newTrie.root.asHash())

	require.True(t, reflect.DeepEqual(trie, newTrie))
}
