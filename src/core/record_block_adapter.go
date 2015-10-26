package core

import "errors"

type RecordBlockAdapter interface {
	FreeSpace() uint16
	Utilization() uint16
	Add(recordID uint32, data []byte) (uint16, uint16)
	NextBlockID() uint16
	SetNextBlockID(blockID uint16)
	SetPrevBlockID(blockID uint16)
	ReadRecordData(localID uint16) string
}

const (
	HEADER_OFFSET_RECORD_ID    = 0
	HEADER_OFFSET_RECORD_START = 4
	HEADER_OFFSET_RECORD_SIZE  = HEADER_OFFSET_RECORD_START + 2
	RECORD_HEADER_SIZE         = uint16(12)

	// A datablock will have at least 8 bytes to store its utilization, total
	// records count and prev / next datablock pointers
	MIN_UTILIZATION = 8

	POS_UTILIZATION   = DATABLOCK_SIZE - 2
	POS_TOTAL_RECORDS = POS_UTILIZATION - 2
	POS_NEXT_BLOCK    = POS_TOTAL_RECORDS - 2
	POS_PREV_BLOCK    = POS_NEXT_BLOCK - 2
	POS_FIRST_HEADER  = POS_PREV_BLOCK - RECORD_HEADER_SIZE - 1
)

type recordBlockAdapter struct {
	block *DataBlock
}

type recordHeader struct {
	localID  uint16
	recordID uint32
	startsAt uint16
	size     uint16
}

func newRecordBlockAdapter(block *DataBlock) RecordBlockAdapter {
	return &recordBlockAdapter{block}
}

func (rba *recordBlockAdapter) Add(recordID uint32, data []byte) (uint16, uint16) {
	utilization := rba.Utilization()
	recordSize := uint16(len(data))

	// Records present on the block
	totalRecords := rba.block.ReadUint16(POS_TOTAL_RECORDS)

	// Calculate where the record starts
	var recordPtr int
	if totalRecords == 0 {
		recordPtr = 0
	} else {
		// Starts where the last record ends
		lastHeaderPtr := int(POS_FIRST_HEADER) - int((totalRecords-1)*RECORD_HEADER_SIZE)
		// FIXME: This will fail once we have deletion implemented
		recordPtr = int(rba.block.ReadUint16(lastHeaderPtr+4) + rba.block.ReadUint16(lastHeaderPtr+6))
	}

	// Header
	newHeaderPtr := int(POS_FIRST_HEADER - totalRecords*RECORD_HEADER_SIZE)

	// Le ID
	rba.block.Write(newHeaderPtr+HEADER_OFFSET_RECORD_ID, recordID)

	// Where the record starts
	rba.block.Write(newHeaderPtr+HEADER_OFFSET_RECORD_START, uint16(recordPtr))

	// Record size
	rba.block.Write(newHeaderPtr+HEADER_OFFSET_RECORD_SIZE, recordSize)

	// TODO: 4 bytes for chained rows

	// Le data
	rba.block.Write(recordPtr, data)
	totalRecords += 1
	utilization += RECORD_HEADER_SIZE + recordSize
	rba.block.Write(POS_UTILIZATION, utilization)
	rba.block.Write(POS_TOTAL_RECORDS, totalRecords)

	// Used as the rowid
	localID := totalRecords - 1
	bytesWritten := recordSize
	return bytesWritten, localID
}

func (rba *recordBlockAdapter) Remove(localID uint16) error {
	// Records present on the block
	totalRecords := rba.block.ReadUint16(POS_TOTAL_RECORDS)
	if localID >= totalRecords {
		return errors.New("Invalid local ID provided to `RecordBlockAdapter.Remove`")
	}

	headerPtr := int(POS_FIRST_HEADER) - int(localID*RECORD_HEADER_SIZE)
	rba.block.Write(headerPtr+HEADER_OFFSET_RECORD_ID, uint32(0))

	// Utilization goes down just by the amount of data taken by the record, the
	// header is kept around so we do not "free" up the space taken by it
	utilization := rba.Utilization() - rba.block.ReadUint16(headerPtr+HEADER_OFFSET_RECORD_SIZE)
	rba.block.Write(POS_UTILIZATION, utilization)

	return nil
}

func (rba *recordBlockAdapter) Utilization() uint16 {
	utilization := rba.block.ReadUint16(POS_UTILIZATION)
	if utilization == 0 {
		utilization = MIN_UTILIZATION
	}
	return utilization
}

func (rba *recordBlockAdapter) NextBlockID() uint16 {
	return rba.block.ReadUint16(POS_NEXT_BLOCK)
}

func (rba *recordBlockAdapter) SetNextBlockID(blockID uint16) {
	rba.block.Write(POS_NEXT_BLOCK, blockID)
}

func (rba *recordBlockAdapter) SetPrevBlockID(blockID uint16) {
	rba.block.Write(POS_PREV_BLOCK, blockID)
}

func (rba *recordBlockAdapter) ReadRecordData(localID uint16) string {
	headerPtr := int(POS_FIRST_HEADER) - int(localID*RECORD_HEADER_SIZE)
	start := rba.block.ReadUint16(headerPtr + HEADER_OFFSET_RECORD_START)
	end := start + rba.block.ReadUint16(headerPtr+HEADER_OFFSET_RECORD_SIZE)
	return string(rba.block.Data[start:end])
}

func (rba *recordBlockAdapter) FreeSpace() uint16 {
	return DATABLOCK_SIZE - rba.Utilization()
}

// HACK: Temporary, meant to be around while we don't have a btree in place
func (rba *recordBlockAdapter) IDs() []uint32 {
	totalRecords := rba.block.ReadUint16(POS_TOTAL_RECORDS)
	ids := []uint32{}

	for i := uint16(0); i < totalRecords; i++ {
		headerPtr := int(POS_FIRST_HEADER - i*RECORD_HEADER_SIZE)
		id := rba.block.ReadUint32(headerPtr + HEADER_OFFSET_RECORD_ID)
		ids = append(ids, id)
	}

	return ids
}
