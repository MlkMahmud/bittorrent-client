package torrent

import (
	"bufio"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/app/utils"
	"github.com/mitchellh/mapstructure"
)

type Peer struct {
	IpAddress string
	Port      uint16
}

type Piece struct {
	Index  int
	Hash   []byte
	Length int
}

type TorrentInfo struct {
	Length int     `mapstructure:"length"`
	Name   string  `mapstructure:"name"`
	Pieces []Piece `mapstructure:"-"`
}

type Torrent struct {
	Info       TorrentInfo `mapstructure:"info"`
	InfoHash   [sha1.Size]byte
	TrackerUrl string `mapstructure:"announce"`
}

func parseTorrentPieces(metainfo map[string]any) ([]Piece, error) {
	infoValue, ok := metainfo["info"].(map[string]any)

	if !ok {
		return nil, fmt.Errorf("expected the 'info' property of the metainfo dict to be a dict, but got %T", infoValue)
	}

	pieceHashes, ok := infoValue["pieces"].(string)

	if !ok {
		return nil, fmt.Errorf("expected the 'pieces' property of the info dict to be a string, but got %T", pieceHashes)
	}

	pieceHashesLen := len(pieceHashes)

	if pieceHashesLen == 0 {
		return nil, fmt.Errorf("the 'pieces' property cannot be an empty string")
	}

	if pieceHashesLen%sha1.Size != 0 {
		return nil, fmt.Errorf("pieces length must be a multiple of %d", sha1.Size)
	}

	pieceLen, ok := infoValue["piece length"].(int)

	if !ok {
		return nil, fmt.Errorf("expected the 'piece length' property of the info dict to be an integer, but got %T", pieceLen)
	}

	fileLen, ok := infoValue["length"].(int)

	if !ok {
		return nil, fmt.Errorf("expected the 'length' property of the info dict to be an integer, but got %T", fileLen)
	}

	numOfPieces := pieceHashesLen / sha1.Size
	piecesArr := make([]Piece, numOfPieces)

	for i, j := 0, 0; i < pieceHashesLen; i += sha1.Size {
		piece := Piece{Index: j, Hash: []byte(pieceHashes[i : sha1.Size+i])}

		// All pieces have the same fixed length, except the last piece which may be truncated
		// The truncated length of the last piece can be generated by subtracting the sum of all other pieces from the total length of the file.
		if j == numOfPieces-1 {
			piece.Length = fileLen - (pieceLen * (numOfPieces - 1))
		} else {
			piece.Length = pieceLen
		}

		piecesArr[j] = piece
		j++
	}

	return piecesArr, nil
}

func NewTorrent(src string) (*Torrent, error) {
	fileContent, err := os.ReadFile(src)

	if err != nil {
		return nil, err
	}

	decodedValue, _, err := bencode.DecodeValue(fileContent)

	if err != nil {
		return nil, err
	}

	trrntDict, ok := decodedValue.(map[string]any)

	if !ok {
		return nil, fmt.Errorf("expected decoded object to be a dict got %T", decodedValue)
	}

	var torrent Torrent

	if err := mapstructure.Decode(trrntDict, &torrent); err != nil {
		return nil, err
	}

	encodedValue, err := bencode.EncodeValue(trrntDict["info"])

	if err != nil {
		return nil, err
	}

	torrent.InfoHash = sha1.Sum([]byte(encodedValue))

	pieces, err := parseTorrentPieces(trrntDict)

	if err != nil {
		return nil, err
	}

	torrent.Info.Pieces = pieces

	return &torrent, nil

}

func (t *Torrent) ConnectToPeer(peer Peer) ([]byte, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(peer.IpAddress, strconv.Itoa(int(peer.Port))), 3*time.Second)

	if err != nil {
		return nil, err
	}

	defer conn.Close()

	peerId := []byte(utils.GenerateRandomString(20, ""))
	pstr := "BitTorrent protocol"
	messageBuf := make([]byte, len(pstr)+49)
	messageBuf[0] = byte(len(pstr))

	index := 1
	index += copy(messageBuf[index:], []byte(pstr))
	index += copy(messageBuf[index:], make([]byte, 8))
	index += copy(messageBuf[index:], t.InfoHash[:])
	index += copy(messageBuf[index:], peerId[:])

	_, writeErr := conn.Write(messageBuf)

	if writeErr != nil {
		return nil, writeErr
	}

	respBuf := make([]byte, cap(messageBuf))
	reader := bufio.NewReader(conn)

	if _, err := io.ReadFull(reader, respBuf); err != nil {
		return nil, err
	}

	return respBuf, nil
}

func (t *Torrent) getTrackerUrlWithParams() string {
	params := url.Values{}

	params.Add("info_hash", string(t.InfoHash[:]))
	params.Add("peer_id", utils.GenerateRandomString(20, ""))
	params.Add("port", "6881")
	params.Add("downloaded", "0")
	params.Add("uploaded", "0")
	params.Add("left", strconv.Itoa(t.Info.Length))
	params.Add("compact", "1")

	queryString := params.Encode()

	return fmt.Sprintf("%s?%s", t.TrackerUrl, queryString)

}

func (t *Torrent) GetPeers() ([]Peer, error) {
	peerSize := 6
	trackerUrl := t.getTrackerUrlWithParams()

	req, err := http.NewRequest("GET", trackerUrl, nil)

	if err != nil {
		return nil, err
	}

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	var trackerResponse []byte

	if res.StatusCode == http.StatusOK {
		trackerResponse, err = io.ReadAll(res.Body)

		if err != nil {
			return nil, err
		}
	}

	decodedResponse, _, err := bencode.DecodeValue(trackerResponse)

	if err != nil {
		return nil, err
	}

	dict, ok := decodedResponse.(map[string]any)

	if !ok {
		return nil, fmt.Errorf("decoded response type \"%T\" is invalid", decodedResponse)
	}

	peers, exists := dict["peers"]

	if !exists {
		return nil, fmt.Errorf("decoded response does not include a \"peers\" key")
	}

	peersValue, ok := peers.(string)

	if !ok {
		return nil, fmt.Errorf("decoded value of \"peers\" is invalid. expected a string got %T", peers)
	}

	peersStringLen := len(peersValue)

	if peersStringLen%peerSize != 0 {
		return nil, fmt.Errorf("peers value must be a multiple of '%d' bytes", peerSize)
	}

	numOfPeers := peersStringLen / peerSize
	peersArr := make([]Peer, numOfPeers)

	for i, j := 0, 0; i < peersStringLen; i += peerSize {
		IpAddress := fmt.Sprintf("%d.%d.%d.%d", byte(peersValue[i]), byte(peersValue[i+1]), byte(peersValue[i+2]), byte(peersValue[i+3]))
		Port := binary.BigEndian.Uint16([]byte(peersValue[i+4 : i+6]))
		peersArr[j] = Peer{IpAddress, Port}

		j++
	}

	return peersArr, nil
}
