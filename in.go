package main

import (
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"sync"                                                             
	"time"
	"net"
	"log"
	"os"
)

type Packet map[uint32][]byte

type FILE struct{
	lastUpdate *time.Timer
	dates Packet
	checknum uint32
}

var emptyFile FILE

type lock struct{
	users map[string]FILE
	mux sync.Mutex
}

var users lock

func (c *lock) Paste(key string, the FILE) {
	c.mux.Lock()
	c.users[key] = the
	c.mux.Unlock()
}

func (c *lock) Copy(key string) (FILE, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()
	user, ok := c.users[key]
	return user, ok
}

func visor(fname string){
	entry, ok := users.Copy(fname)
	if !ok{
		log.Println("Error, no file")
		return
	}
	for{
		select {
			case <- entry.lastUpdate.C:
				users.Paste(fname, emptyFile)
				break
		}
	}
}

func main() {
	totalPackeges:=0

	users = lock{users: make(map[string]FILE)}

	conn, err := net.ListenPacket("udp", ":3000")
	
	if err != nil{
		log.Fatal(err)
	}
	
	defer conn.Close()
	
	buffer := make([]byte, 61000)
	
	if err != nil{
		log.Fatal(err)
	}
	
	doJob := func (tbuffer []byte, n int, remoteAddr *net.Addr){
		curPac := binary.LittleEndian.Uint32(tbuffer[32:36])
		log.Printf("Start working with %v", curPac)
		fname := string(tbuffer[0:32])
		entry, ok := users.Copy(fname)
		if !ok{
			entry = emptyFile
			entry.dates = make(map[uint32][]byte)
			entry.lastUpdate = time.NewTimer(time.Second * 60)
			log.Println("New File")
		}else{
			log.Println("here")
			entry.lastUpdate.Reset(time.Second * 60)
			log.Println("here 2")
		}
		ld:=len(tbuffer[36:n])
		if ld==1 && tbuffer[n-1]==byte(2){
			entry.checknum = curPac
			log.Printf("Checknum now is %v", curPac)
		}else{
			buffer := make([]byte, n-36)
			copy(buffer, tbuffer[36:n])
			entry.dates[curPac] = buffer
		}
		
		if entry.checknum != 0 {  //entry.checknum == uint32(len(entry.dates))
			log.Println("Весь файл получен")
			users.Paste(fname, entry)
			makeFile(fname, conn, remoteAddr)
			return
		}
		
		users.Paste(fname, entry)
		
		if !ok{
			go visor(fname)
		}
		
		log.Println("End working", curPac)
	}
	
	for{
		n, remoteAddr, err := conn.ReadFrom(buffer)
		
		if err!=nil {
			log.Printf("ERROR: %v", err)
			continue
		}
		
		log.Printf("Read %v bytes from %v",n, remoteAddr)
		doJob(buffer, n, &remoteAddr)
		totalPackeges++
	}
}

func makeFile(fname string, conn net.PacketConn, remoteAddr *net.Addr){
	log.Println("making file")
	results, ok := users.Copy(fname)
	if !ok{
		log.Println("Error while making",fname,"file - no such file")
		return
	}
	var data []byte
	tbs := make([]byte, 4)
	for i := uint32(2); i<results.checknum; i++{
		cl := len(results.dates[i])
		log.Println("Working with",i)
		if cl!=0{
			log.Println(results.dates[i][1])
		}
		if cl != 0 && results.dates[i][0] == byte(154){
			data = append(data, results.dates[i][1:]...)
		}else{
			binary.LittleEndian.PutUint32(tbs, i)
			bytesWritten, err := conn.WriteTo(tbs, *remoteAddr)
			if err != nil{
				log.Fatal(err)
			}
			log.Printf("Write %v bytes to %v", bytesWritten, remoteAddr)
			return
		}
	}
	if len(results.dates[0][1:])<1{
		log.Println("Имени не существует!")
		return
	}
	defname := string(results.dates[0][1:])
	name := defname
	plus := 0
	if string(getHash(data)) == string(results.dates[1][1:]){
		for ex, err := exists(name); ex; ex, err = exists(name){
			if err!=nil{
				log.Panic(err)
			}else{
				plus++
				name = strconv.Itoa(plus) + defname
				log.Println("Now name is", name)
			}
		}
		file, err := os.Create(name)
		if err != nil{
			log.Println("Unable to create file:", err) 
			os.Exit(1) 
		}
		defer file.Close() 
		file.Write(data)
		log.Printf("File %s saved", name)
		tbs = make([]byte, 4)
		binary.LittleEndian.PutUint32(tbs, uint32(0))
		conn.WriteTo(tbs, *remoteAddr)
		results.lastUpdate.Stop()
		users.Paste(fname, emptyFile)
	}else{
		log.Println("Save Error, hash not same as control sum")
	}
}

func exists(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil { return true, nil }
    if os.IsNotExist(err) { return false, nil }
    return false, err
}

func getHash(in []byte) []byte {
	hasher := sha256.New()
	hasher.Write(in)
	sha := hasher.Sum(nil)
	return sha
}