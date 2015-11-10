package core

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"simplejsondb/dbio"
	"sort"
)

type BTreeLeaf interface {
	BTreeNode
	Add(searchKey uint32, rowID RowID)
	Remove(searchKey uint32)
	Shift() RowID
	Find(searchKey uint32) RowID
	First() RowID
	All() []RowID
	IsFull() bool
}

const (
	BTREE_LEAF_MAX_ENTRIES     = 510
	BTREE_LEAF_ENTRY_SIZE      = 8
	BTREE_LEAF_OFFSET_KEY      = 0
	BTREE_LEAF_OFFSET_BLOCK_ID = 4
	BTREE_LEAF_OFFSET_LOCAL_ID = 6
)

type bTreeLeaf struct {
	*bTreeNode
}

func CreateBTreeLeaf(block *dbio.DataBlock) BTreeLeaf {
	block.Write(BTREE_POS_TYPE, BTREE_TYPE_LEAF)
	node := &bTreeNode{block}
	return &bTreeLeaf{node}
}

func (l *bTreeLeaf) Add(searchKey uint32, rowID RowID) {
	entriesCount := l.block.ReadUint16(BTREE_POS_ENTRIES_COUNT)

	addPosition := sort.Search(int(entriesCount), func(i int) bool {
		offset := int(BTREE_POS_ENTRIES_OFFSET) + int(i)*int(BTREE_LEAF_ENTRY_SIZE)
		keyFound := l.block.ReadUint32(offset + BTREE_LEAF_OFFSET_KEY)
		return keyFound >= searchKey
	})
	writeOffset := int(BTREE_POS_ENTRIES_OFFSET) + int(addPosition)*int(BTREE_LEAF_ENTRY_SIZE)
	if uint16(addPosition) < entriesCount && searchKey == l.block.ReadUint32(writeOffset+BTREE_LEAF_OFFSET_KEY) {
		panic(fmt.Sprintf("Duplicate key detected: %d", searchKey))
	}

	l.block.Unshift(writeOffset, BTREE_LEAF_ENTRY_SIZE)
	l.block.Write(writeOffset+BTREE_LEAF_OFFSET_KEY, searchKey)
	l.block.Write(writeOffset+BTREE_LEAF_OFFSET_BLOCK_ID, rowID.DataBlockID)
	l.block.Write(writeOffset+BTREE_LEAF_OFFSET_LOCAL_ID, rowID.LocalID)

	entriesCount += 1
	l.block.Write(BTREE_POS_ENTRIES_COUNT, entriesCount)

	log.Infof("IDX_LEAF_ADDED blockID=%d, searchKey=%d, position=%d, entriesCount=%d, rowID=%+v", l.block.ID, searchKey, addPosition, entriesCount, rowID)
}

func (l *bTreeLeaf) Find(searchKey uint32) RowID {
	log.Debugf("LEAF_FIND blockid=%d, searchkey=%d", l.block.ID, searchKey)
	entriesCount := int(l.block.ReadUint16(BTREE_POS_ENTRIES_COUNT))

	// XXX: Should we perform a binary search here?
	for i := 0; i < entriesCount; i++ {
		ptr := int(BTREE_POS_ENTRIES_OFFSET + (i * BTREE_LEAF_ENTRY_SIZE))
		keyFound := l.block.ReadUint32(ptr + BTREE_LEAF_OFFSET_KEY)
		if keyFound != searchKey {
			continue
		}
		return RowID{
			RecordID:    searchKey,
			DataBlockID: l.block.ReadUint16(ptr + BTREE_LEAF_OFFSET_BLOCK_ID),
			LocalID:     l.block.ReadUint16(ptr + BTREE_LEAF_OFFSET_LOCAL_ID),
		}
	}
	return RowID{}
}

func (l *bTreeLeaf) Remove(searchKey uint32) {
	entriesCount := int(l.block.ReadUint16(BTREE_POS_ENTRIES_COUNT))

	log.Debugf("LEAF_REMOVE blockid=%d, searchkey=%d, entriescount=%d", l.block.ID, searchKey, entriesCount)

	// TODO: Shortcut remove on last entry

	// XXX: Should we perform a binary search here?
	entryPtrToRemove := -1
	entryPosition := 0
	for i := 0; i < entriesCount; i++ {
		ptr := int(BTREE_POS_ENTRIES_OFFSET) + int(i*BTREE_LEAF_ENTRY_SIZE)
		keyFound := l.block.ReadUint32(ptr + BTREE_LEAF_OFFSET_KEY)
		log.Debugf("LEAF_REMOVE_KEY_CANDIDATE block=%d, key=%d, ptr=%d", l.block.ID, keyFound, ptr)
		if keyFound == searchKey {
			log.Debugf("LEAF_REMOVE_KEY block=%d, key=%d, ptr=%d", l.block.ID, keyFound, ptr)
			entryPtrToRemove = ptr
			entryPosition = i
			break
		}
	}

	if entryPtrToRemove == -1 {
		panic(fmt.Sprintf("Tried to remove an entry that does not exist on the index: %d", searchKey))
	}

	// Write back the amount of entries on this block
	l.block.Write(BTREE_POS_ENTRIES_COUNT, uint16(entriesCount-1))

	// If we are removing the last key, just update the entries count and call it a day
	if entryPosition == (entriesCount - 1) {
		return
	}

	// Copy data over
	lastByteToOverwrite := int(BTREE_POS_ENTRIES_OFFSET) + (entriesCount-1)*BTREE_LEAF_ENTRY_SIZE
	for i := entryPtrToRemove; i < lastByteToOverwrite; i++ {
		l.block.Data[i] = l.block.Data[i+BTREE_LEAF_ENTRY_SIZE]
	}
}

func (l *bTreeLeaf) First() RowID {
	entriesCount := int(l.block.ReadUint16(BTREE_POS_ENTRIES_COUNT))
	if entriesCount == 0 {
		panic("Called First() on a leaf that has no entries")
	}
	ptr := int(BTREE_POS_ENTRIES_OFFSET)
	return RowID{
		RecordID:    l.block.ReadUint32(ptr + BTREE_LEAF_OFFSET_KEY),
		DataBlockID: l.block.ReadUint16(ptr + BTREE_LEAF_OFFSET_BLOCK_ID),
		LocalID:     l.block.ReadUint16(ptr + BTREE_LEAF_OFFSET_LOCAL_ID),
	}
}

func (l *bTreeLeaf) Shift() RowID {
	entriesCount := int(l.block.ReadUint16(BTREE_POS_ENTRIES_COUNT))
	if entriesCount == 0 {
		panic("Called Shift() on a leaf that has no entries")
	}
	first := l.First()
	l.Remove(first.RecordID)
	return first
}

func (l *bTreeLeaf) All() []RowID {
	entriesCount := int(l.block.ReadUint16(BTREE_POS_ENTRIES_COUNT))
	all := make([]RowID, 0, entriesCount)
	for i := 0; i < entriesCount; i++ {
		ptr := int(BTREE_POS_ENTRIES_OFFSET + (i * BTREE_LEAF_ENTRY_SIZE))
		all = append(all, RowID{
			RecordID:    l.block.ReadUint32(ptr + BTREE_LEAF_OFFSET_KEY),
			DataBlockID: l.block.ReadUint16(ptr + BTREE_LEAF_OFFSET_BLOCK_ID),
			LocalID:     l.block.ReadUint16(ptr + BTREE_LEAF_OFFSET_LOCAL_ID),
		})
	}
	return all
}

func (n *bTreeLeaf) IsFull() bool {
	return n.block.ReadUint16(BTREE_POS_ENTRIES_COUNT) == BTREE_LEAF_MAX_ENTRIES
}
