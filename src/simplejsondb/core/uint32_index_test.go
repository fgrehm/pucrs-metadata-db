package core_test

import (
	"sort"
	"testing"

	"simplejsondb/core"
	"simplejsondb/dbio"

	utils "test_utils"
)

// This is basically a copy and paste from the tests for the in memory B+Tree code

func TestUint32Index_BasicOperations(t *testing.T) {
	branchCapacity := 6
	leafCapacity := 4
	index := createIndex(t, 30, 20, branchCapacity, leafCapacity)

	err := index.All(func(rowID core.RowID) {
		t.Fatalf("Index should be blank but found the following rowid: %+v", rowID)
	})
	if err != nil {
		t.Fatalf("Error while reading all entries from the tree: %s", err)
	}

	totalEntries := branchCapacity * leafCapacity

	// Fill in the tree in descending / ascending order, one half at a time
	secondHalf := []core.RowID{}
	for i := totalEntries - 1; i >= (totalEntries/2)-1; i-- {
		key := i + 1
		rowID := core.RowID{LocalID: uint16(key)}
		assertIndexCanInsertAndFind(t, index, key, rowID)
		secondHalf = append([]core.RowID{rowID}, secondHalf...)
	}
	firstHalf := []core.RowID{}
	for i := 0; i < (totalEntries/2)-1; i++ {
		key := i + 1
		rowID := core.RowID{LocalID: uint16(key)}
		assertIndexCanInsertAndFind(t, index, key, rowID)
		firstHalf = append(firstHalf, rowID)
	}
	rowIDsInOrder := append(firstHalf, secondHalf...)

	// Can we retrieve the rowids from the tree?
	indexAllWasCalled := false
	position := 0
	index.All(func(rowID core.RowID) {
		indexAllWasCalled = true
		if rowID != rowIDsInOrder[position] {
			t.Errorf("Found an invalid RowID at %d, got %+v, expected %+v", position, rowID, rowIDsInOrder[position])
		}
		position++
	})
	if !indexAllWasCalled {
		t.Fatal("The function provided to Index.All was not called")
	}

	// Delete everything from the tree an ensure it has no records
	for i := 0; i < totalEntries; i++ {
		key := i + 1
		assertIndexCanDeleteByKey(t, index, key)
	}
	index.All(func(rowID core.RowID) {
		t.Fatal("No entries should be present on the index but found %+v", rowID)
	})
}

func TestUint32Index_GrowAndShrinkLotsOfEntries(t *testing.T) {
	branchCapacity := 4
	leafCapacity := 4
	index := createIndex(t, 250, 256, branchCapacity, leafCapacity)
	totalEntries := (branchCapacity + 1) * leafCapacity

	keys := make([]int, 0, totalEntries*30)
	for h := 0; h < 30; h++ {
		var start, end int
		if h%2 == 0 {
			start = 0
			end = totalEntries / 2
		} else {
			start = totalEntries/2 + 1
			end = totalEntries
		}
		for i := start; i < end; i++ {
			key := i*50 + h+1
			assertIndexCanInsertAndFind(t, index, key, core.RowID{LocalID: uint16(key)})
			keys = append(keys, key)
		}
	}

	t.Errorf("TEST IF IS ORDERED BY COLLECTING THE ROW IDS INSERTED, CHECK AT EACH REMOVE IF THE ENTRY CAN BE FOUND")

	sort.Ints(keys)
	firstHalf := keys[len(keys)/2-1:]
	secondHalf := keys[:len(keys)/2]
	sort.Sort(sort.Reverse(sort.IntSlice(secondHalf)))

	for _, key := range secondHalf {
		assertIndexCanDeleteByKey(t, index, key)
	}
	for _, key := range firstHalf {
		assertIndexCanDeleteByKey(t, index, key)
	}
}

func createIndex(t *testing.T, totalUsableBlocks, bufferFrames, branchCapacity int, leafCapacity int) core.Uint32Index {
	blocks := [][]byte{
		make([]byte, dbio.DATABLOCK_SIZE),
		make([]byte, dbio.DATABLOCK_SIZE),
		nil,
		nil,
	}
	for i := 0; i < totalUsableBlocks; i++ {
		blocks = append(blocks, make([]byte, dbio.DATABLOCK_SIZE))
	}
	fakeDataFile := utils.NewFakeDataFile(blocks)
	dataBuffer := dbio.NewDataBuffer(fakeDataFile, bufferFrames)
	testRepo := core.NewDataBlockRepository(dataBuffer)

	controlBlock := testRepo.ControlBlock()
	controlBlock.Format()
	dataBuffer.MarkAsDirty(controlBlock.DataBlockID())

	blockMap := testRepo.DataBlocksMap()
	for i := uint16(0); i < 4; i++ {
		blockMap.MarkAsUsed(i)
	}

	index := core.NewUint32Index(dataBuffer, branchCapacity, leafCapacity)
	index.Init()
	return index
}

func insertOnIndex(t *testing.T, index core.Uint32Index, key int, rowID core.RowID) {
	if err := index.Insert(uint32(key), rowID); err != nil {
		t.Fatalf("Error inserting rowID with key %d: %s", key, err)
	}
}

func assertIndexCanDeleteByKey(t *testing.T, index core.Uint32Index, key int) {
	index.Delete(uint32(key))
	assertIndexCantFindByKey(t, index, key)
}

func assertIndexCantFindByKey(t *testing.T, index core.Uint32Index, key int) {
	if _, err := index.Find(uint32(key)); err == nil {
		t.Error("Did not remove key from index")
	}
}

func assertIndexCanInsertAndFind(t *testing.T, index core.Uint32Index, key int, rowID core.RowID) (core.Uint32Key, core.RowID) {
	insertOnIndex(t, index, key, rowID)
	rowIDFound, err := index.Find(uint32(key))
	if err != nil {
		t.Fatalf("Error when trying to find rowID with key=%+v: %s", key, err)
	}
	if rowIDFound == (core.RowID{}) {
		t.Errorf("Could not retrieve %d from index right after inserting it", key)
	}
	if rowIDFound != rowID {
		t.Errorf("Invalid value returned from the index: %+v", rowIDFound)
	}
	return core.Uint32Key(key), rowID
}

func assertIndexItemsAreSame(t *testing.T, index core.Uint32Index, rowIDs []core.RowID) {
	i := 0
	funcCalled := false
	index.All(func(rowID core.RowID) {
		funcCalled = true
		if rowID != rowIDs[i] {
			t.Errorf("Invalid value returned from the index at %d: %+v (expected %+v)", i, rowID, rowIDs[i])
		}
		i++
	})
	if i != len(rowIDs) {
		t.Errorf("Did not iterate over all entries")
	}
	if !funcCalled {
		t.Errorf("Function passed to core.Uint32Index was not called")
	}
}
