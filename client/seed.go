package client


import (
	"net"
	"log"
	"fmt"
	"os"
	"time"
	"encoding/binary"

	"torrent-dsp/model"
	"torrent-dsp/constant"
	// "time"
    // "encoding/binary"
    // "encoding/hex"
)

func SeederMain() {
	// start a server listening on port 6881
	torrent, err := model.ParseTorrentFile("./torrent-files/debian-11.6.0-amd64-netinst.iso.torrent")
	file, err := os.Open("/Users/kemerhabesha/Desktop/torrent-dsp/debian-11.6.0-amd64-netinst.iso")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

    ln, err := net.Listen("tcp", ":6881")

    if err != nil {
        log.Fatalf("Failed to listen: %s", err)
    }

    for {
        if conn, err := ln.Accept(); err == nil {
			fmt.Println("Accepted connection")
            go handleConnection(conn, torrent, file)
        }
    }
}


func handleConnection(conn net.Conn, torrent model.Torrent, file *os.File) {
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer conn.Close()
	fmt.Println("Handling connection")
	fmt.Println()

	fmt.Println("------------------- Waiting for handshake -------------------")
	_, err := ReceiveHandShake(conn)
	fmt.Println("Received handshake")
	fmt.Println()
	if err != nil {
		fmt.Println("Error receiving handshake")
		return
	}

	// TODO: change byte to actual bit field of the data
	if err != nil {
		log.Fatal(err)
	}

	// send the handshake request
	SendHandShake(conn, torrent)
	fmt.Println("------------------- Sending bit field -------------------")
	SendBitField(conn)
	fmt.Println("------------------- Sent bit field -------------------")
	fmt.Println()
	
	// listen to unchoke message
	ReceiveUnchoke(conn)
	fmt.Println("Received unchoke message")
	// listen to interested message
	ReceiveInterested(conn)
	fmt.Println("Received interested message")

	// listen to other request messages
	for {
		fmt.Println("Waiting for request...")
		go ReceiveRequest(conn, file)
		fmt.Println("Received request and sent piece")
	}
}


func ReceiveHandShake(conn net.Conn) (*model.HandShake, error) {
	// read the handshake response
	// handshake response is 68 bytes long <length of Protocol> + <Protocol> + <Reserved> + <InfoHash> + <PeerID>
	// TODO: increase time out
	buffer := make([]byte, 68)
	_, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading handshake response")
		return &model.HandShake{}, err
	}

	// deserialize the handshake response
	handShake, err := model.DeserializeHandShake(buffer)
	if err != nil {
		fmt.Println("Error deserializing handshake response")
		return &model.HandShake{}, err
	}

	fmt.Println("Handshake sent successfully")
	return handShake, nil
}


func SendHandShake(conn net.Conn, torrent model.Torrent) error {
	// convert the client id to a byte array
	clientIDByte := [20]byte{}
	copy(clientIDByte[:], []byte(constant.CLIENT_ID))

	// create a handshake request
	handshakeRequest := model.HandShake{
		Pstr:     "BitTorrent protocol",
		InfoHash: torrent.InfoHash,
		PeerID:   clientIDByte,
	}

	// send the handshake request
	// serialize the handshake request
	buffer := handshakeRequest.Serialize()

	// send the handshake request
	_, err := conn.Write(buffer)
	if err != nil {
		fmt.Println("Error sending handshake request SEEDER")
		return err
	}
	return nil
}


func SendBitField(conn net.Conn) error {
	// send the bitfield
	bitField := make([]byte, 255)
	for i := 0; i < len(bitField); i++ {
		bitField[i] = 255
	}

	msg := model.Message{MessageID: constant.BIT_FIELD, Payload: bitField}
	_, err := conn.Write(msg.Serialize())
	if err != nil {
		return err
	}
	return nil
}


func ReceiveUnchoke(conn net.Conn) error {
	buffer := make([]byte, 5)
	_, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading unchoke message")
		return err
	}

	return nil
}


func ReceiveInterested(conn net.Conn) error {
	buffer := make([]byte, 5)
	_, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading interested message")
		return err
	}

	return nil
}


func ReceiveRequest(conn net.Conn, file *os.File) error {
	// buffer := make([]byte, 17)
	conn.SetDeadline(time.Now().Add(constant.PIECE_DOWNLOAD_TIMEOUT))
    defer conn.SetDeadline(time.Time{})

	requestMsg, err := model.DeserializeMessage(conn)
	if err != nil {
		fmt.Println("Error reading request message")
		return err
	}

	if err != nil {
		fmt.Println("Error opening file")
		return err
	}

	// parse payload
	if requestMsg.MessageID != constant.REQUEST {
		fmt.Println("Error: received message is not a request")
		return nil
	}
	_, begin, size := ParseRequestPayload(requestMsg.Payload)

	// read the piece from the file
	piece := make([]byte, int64(begin))
	_, err = file.ReadAt(piece, int64(size))
	if err != nil {
		fmt.Println("Error reading piece from file")
		return err
	}

	// send the piece
	err = SendPiece(conn, piece)
	if err != nil {
		fmt.Println("Error sending piece")
		return err
	}

	return nil
}


func SendPiece(conn net.Conn, piece []byte) error {
	msg := model.Message{MessageID: constant.PIECE, Payload: piece}
	_, err := conn.Write(msg.Serialize())
	if err != nil {
		fmt.Println("Error sending piece")
		return err
	}

	return nil
}


func ParseRequestPayload(payload []byte) (int, int, int) {
	index := int(binary.BigEndian.Uint32(payload[0:4]))
	blockStart := int(binary.BigEndian.Uint32(payload[4:8]))
	blockSize := int(binary.BigEndian.Uint32(payload[8:12]))
	pieceSize := 262144
	fileSize := 471859200
	begin := index*pieceSize + blockStart
	end := begin + blockSize

	if end > fileSize {
		end = fileSize
		blockSize = end - begin
	}
	// request := Request{
	// 	Index:      index,
	// 	BlockBegin: blockStart,
	// 	Begin:      begin,
	// 	End:        end,
	// 	BlockSize:  blockSize,
	// }

	return index, begin, blockSize
}
