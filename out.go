package main

import (
	"crypto/sha256"
	"path/filepath"
	"encoding/binary"
	"strconv"
	"time"
	"net"
	"log"
	"os"
)

const maxBytesInMessage int = 9000
var totalPackeges uint32 = 0
var Packets map[uint32][]byte

func main() {
	if len(os.Args)<3{
		log.Println("Adress and name needed")
		os.Exit(1)
	}
	
	remoteAddr, err:= net.ResolveUDPAddr("udp", os.Args[1])
	
	if err != nil{
		log.Fatal(err)
	}
	
	conn, err := net.ListenPacket("udp", "")
	
	if err != nil{
		log.Fatal(err)
	}
	
	defer conn.Close()
	
	fp := os.Args[2]
	name := []byte(filepath.Base(fp))
	id := makeID(name)
	
	log.Printf("Sending %s", fp)
	
	file, err := os.ReadFile(fp)
	if err != nil {
	   log.Panic(err)
	}
	
	Packets = make(map[uint32][]byte)
	
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, totalPackeges)
	
	for{
		curmess:=append(id, bs...)
		curmess=append(curmess, byte(0))
		curmess=append(curmess, name...)
		bytesWritten, err := conn.WriteTo(Packets[0], remoteAddr)
		if err != nil{
			log.Fatal(err)
		}
		log.Printf("Write %v bytes to %v", bytesWritten, remoteAddr)
		
		buffer := make([]byte, 4)
		n, _, err := conn.ReadFrom(buffer)
		if err != nil{
			log.Fatal(err)
		}
		log.Printf("Read %v bytes from %s",n, "server")
		pn := binary.LittleEndian.Uint32(buffer)
		if pn==0{
			break
		}
	}
	
	curmess:=append(id, bs...)
	curmess=append(curmess, byte(0))
	curmess=append(curmess, name...)
	
	send(curmess, conn, remoteAddr)
	
	binary.LittleEndian.PutUint32(bs, totalPackeges)
	
	curmess=append(id, bs...)
	curmess=append(curmess, byte(1))
	curmess=append(curmess, getHash(file)...)
	
	send(curmess, conn, remoteAddr)
	
	sendFile(id, file, conn, remoteAddr)
	
	log.Println(totalPackeges)
}

func makeID(name []byte) []byte {
	ans := append([]byte(strconv.Itoa(int(time.Now().UnixNano()))), name...)
	ans = getHash(ans)
	return ans
}

func sendFile(id []byte, file []byte, conn net.PacketConn, remoteAddr *net.UDPAddr){
	ln, s, e := len(file), 0, 0
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, totalPackeges)
	
	if e+maxBytesInMessage>ln{
		toSend:=append(id, bs...)
		toSend=append(toSend, byte(154))
		toSend=append(toSend, file[0:ln]...)
		send(toSend, conn, remoteAddr)
	}else{
		e+=maxBytesInMessage
	
		for{
			toSend:=append(id, bs...)
			toSend=append(toSend, byte(154))
			toSend=append(toSend, file[s:e]...)
			send(toSend, conn, remoteAddr)
			
			s=e
			
			binary.LittleEndian.PutUint32(bs, totalPackeges)
			
			if e+maxBytesInMessage>ln{
				toSend=append(id, bs...)
				toSend=append(toSend, byte(154))
				toSend=append(toSend, file[s:ln]...)
				send(toSend, conn, remoteAddr)
				break
			}else{
				e+=maxBytesInMessage
			}
		}
	}
	
	binary.LittleEndian.PutUint32(bs, totalPackeges)
	
	curmess:=append(id, bs...)
	curmess=append(curmess, byte(2))
	
	send(curmess, conn, remoteAddr)
	
	end := time.After(15 * time.Second)
	buffer := make([]byte, 4)
	go func(){for {
		select {
			case <-end:
				log.Println("Ошибка передачи файла, повторите попытку")
				os.Exit(1)
		}
	}}()
	for{
		n, _, err := conn.ReadFrom(buffer)
		if err != nil{
			log.Fatal(err)
		}
		log.Printf("Read %v bytes from %s",n, "server")
		pn := binary.LittleEndian.Uint32(buffer)
		if pn == 0{
			log.Println("Файл отправлен успешно")
			return
		}else{
			bytesWritten, err := conn.WriteTo(Packets[pn], remoteAddr)
			if err != nil{
				log.Fatal(err)
			}
			log.Printf("Write %v bytes to %v", bytesWritten, remoteAddr)
		}
	}
}

func send(file []byte, conn net.PacketConn, remoteAddr *net.UDPAddr){
	pn := binary.LittleEndian.Uint32(file[32:36])
	log.Printf("Packet %v sizeof %v", pn, len(file))
	Packets[pn] = file
	bytesWritten, err := conn.WriteTo(file, remoteAddr)
	
	
	if err != nil{
		log.Fatal(err)
	}
	
	log.Printf("Write %v bytes to %v", bytesWritten, remoteAddr)
	totalPackeges++
}

func getHash(in []byte) []byte {
	hasher := sha256.New()
	hasher.Write(in)
	sha := hasher.Sum(nil)
	return sha
}