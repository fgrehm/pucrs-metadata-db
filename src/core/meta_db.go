package core

import (
	"log"
)

const BUFFER_SIZE = 256

type MetaDB interface {
	InsertRecord(data string) (uint32, error)
	Close() error
	// FindRecord(id uint64) (*Record, error)
	// SearchFor(key, value string) (<-chan Record, error)
}

type metaDb struct {
	dataFile DataFile
	buffer   DataBuffer
}

func NewMetaDB(datafilePath string) (MetaDB, error) {
	df, err := newDatafile(datafilePath)
	if err != nil {
		return nil, err
	}
	return NewMetaDBWithDataFile(df)
}

func NewMetaDBWithDataFile(dataFile DataFile) (MetaDB, error) {
	dataBuffer := NewDataBuffer(dataFile, BUFFER_SIZE)
	block, err := dataBuffer.FetchBlock(0)
	if err != nil {
		return nil, err
	}
	if block.ReadUint32(0) == 0 {
		log.Println("Initializing datafile")

		// Next ID = 1
		block.Write(0, uint32(1))
		// Next Available Datablock = 1
		block.Write(4, uint16(1))

		dataBuffer.MarkAsDirty(block.ID)
		if err = dataBuffer.Sync(); err != nil {
			return nil, err
		}
	}
	return &metaDb{dataFile, dataBuffer}, nil
}

func (m *metaDb) InsertRecord(data string) (uint32, error) {
	// TODO: Find out if data fits in a block in advance (chained rows will come later)

	block, err := m.buffer.FetchBlock(0)
	if err != nil {
		return 0, err
	}

	recordId := block.ReadUint32(0)
	insertBlockId := block.ReadUint16(4)
	// Next ID
	block.Write(0, uint32(recordId+1))
	m.buffer.MarkAsDirty(block.ID)

	block, err = m.buffer.FetchBlock(insertBlockId)
	if err != nil {
		return 0, err
	}

	record := &Record{ID: recordId, Data: data}
	m.allocateRecord(record, block)

	return recordId, nil
}

func (m *metaDb) Close() error {
	if err := m.buffer.Sync(); err != nil {
		return err
	}
	return m.dataFile.Close()
}

func (m *metaDb) allocateRecord(record *Record, initialDataBlock *DataBlock) {
	// A datablock will have at least 2 bytes to store its utilization, if it
	// is currently zero, it means it is a brand new block
	utilization := initialDataBlock.ReadUint16(DATABLOCK_SIZE - 2)
	if utilization == 0 {
		utilization = 2
	}

	recordSize := uint16(len(record.Data))
	headerSize := uint16(12)

	// Records present on the block
	totalRecords := initialDataBlock.ReadUint16(DATABLOCK_SIZE - 4)
	totalRecords += 1

	// Header
	// 2 for utilization, 2 for total records, 4 for next / prev block pointers
	headerPtr := DATABLOCK_SIZE - 8
	headerPtr -= int(totalRecords*headerSize) + 1

	// Le ID
	initialDataBlock.Write(headerPtr, record.ID)
	headerPtr += 4

	// Calculate where the record starts
	var recordPtr int
	if totalRecords == 1 {
		recordPtr = 0
	} else {
		lastHeaderPtr := DATABLOCK_SIZE - 8 - int((totalRecords-1)*headerSize) - 1
		// Starts where the last record ends
		// FIXME: This will fail once we have deletion implemented
		recordPtr = int(initialDataBlock.ReadUint16(lastHeaderPtr+4) + initialDataBlock.ReadUint16(lastHeaderPtr+6))
	}
	initialDataBlock.Write(headerPtr, uint16(recordPtr))
	headerPtr += 2

	// Record size
	initialDataBlock.Write(headerPtr, recordSize)
	headerPtr += 2

	// TODO: 4 bytes for chained rows

	initialDataBlock.Write(recordPtr, record.Data)

	utilization += headerSize + recordSize
	initialDataBlock.Write(DATABLOCK_SIZE-2, utilization)
	initialDataBlock.Write(DATABLOCK_SIZE-4, totalRecords)
	m.buffer.MarkAsDirty(initialDataBlock.ID)

	// - Records data
	// - End the end of the datablock:
	//   - 4 bytes for pointer to previous and next data blocks on the linked list of data blocks of a given type (index or actual data, 2 points each)
}
