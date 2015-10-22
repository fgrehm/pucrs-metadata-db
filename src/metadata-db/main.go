package main

import (
	"fmt"
	"log"

	_ "github.com/icrowley/fake"

	"core"
)

func main() {
	db, err := core.NewMetaDB("metadata-db.dat")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()

	// now := time.Now().Format("2006-01-02T15:04:05MST")
	// json := `{"name":"` + fake.FullName() + `","company":"` + fake.Company() + `","city":"` + fake.City() + `","created_at":"` + now + `"}`
	// log.Println("Will insert", json)
	// id, err := db.InsertRecord(json)
	// log.Printf("ID:     %d", id)
	// log.Printf("Error:  %+v", err)

	// record, err := db.FindRecord(1)
	// log.Printf("Record: %+v", record)
	// log.Printf("Error:  %+v", err)

	// record, err = db.FindRecord(2)
	// log.Printf("Record: %+v", record)
	// log.Printf("Error:  %+v", err)

	for i := 0; i < 100; i++ {
		data := fmt.Sprintf(`{"a":%d}`, i)
		id, err := db.InsertRecord(data)
		if err != nil {
			log.Fatalf("Unexpected error returned '%s'", err)
		}
		log.Println("New record ID", id)

		record, err := db.FindRecord(id)
		if err != nil {
			log.Fatalf("Unexpected error returned '%s'", err)
		}
		if record.Data != data {
			log.Fatalf("Unexpected data returned (%s)", record.Data)
		}
	}

	// Test reading / writing blocks and bitmaps
	//
	// block, err := df.ReadBlock(0)
	// if err != nil {
	// 	panic(err)
	// }
	// log.Println(core.DatablockByteOrder.Uint16(block.Data[0:2]))
	// log.Println(core.DatablockByteOrder.Uint16(block.Data[2:4]))

	// core.DatablockByteOrder.PutUint16(block.Data[0:2], uint16(1))
	// core.DatablockByteOrder.PutUint64(block.Data[2:10], uint64(9999))
	// core.DatablockByteOrder.PutUint16(block.Data[14:16], uint16(4))

	// log.Printf("%x", block.Data[32:34])

	// bmap := core.NewBitMapFromBytes(block.Data[32:34])
	// //bmap := core.NewBitMap(16)

	// // bmap.Set(0)
	// // bmap.Set(3)
	// // bmap.Set(4)
	// // bmap.Unset(3)
	// // bmap.Set(15)
	// // bmap.Set(14)
	// // bmap.Set(13)

	// for i := 0; i < 16; i++ {
	// 	set, _ := bmap.Get(i)
	// 	println(i, set)
	// }

	// block.Data[32] = bmap.Bytes()[0]
	// block.Data[33] = bmap.Bytes()[1]

	// df.WriteBlock(block)
	// log.Println("Done")
}
