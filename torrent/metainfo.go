package torrent

import (
	"context"
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/MlkMahmud/hail/bencode"
	"github.com/MlkMahmud/hail/utils"
)

func parseAnnounceList(list any) (*utils.Set, error) {
	trackers := utils.NewSet()

	announceList, ok := list.([]any)

	if !ok {
		return nil, fmt.Errorf("\"announce-list\" property should be a list, but received '%T'", announceList)
	}

	for listIndex, tier := range announceList {
		tierList, ok := tier.([]any)

		if !ok {
			return nil, fmt.Errorf("announce list contains an invalid entry at index %d", listIndex)
		}

		for tierIndex, url := range tierList {
			urlStr, ok := url.(string)

			if !ok {
				return nil, fmt.Errorf("announce list entry at index %d contains an invalid entry at index %d", listIndex, tierIndex)
			}

			if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") || strings.HasPrefix(urlStr, "udp://") {
				trackers.Add(urlStr)
			}
		}
	}

	return trackers, nil
}

func parseFilesList(infoDict map[string]any, tr Torrent) (*torrentInfo, error) {
	filesList, ok := infoDict["files"].([]any)

	if !ok {
		return nil, fmt.Errorf("expected 'files' property to be a list, but received '%T'", filesList)
	}

	numOfFiles := len(filesList)
	files := make([]file, numOfFiles)

	pieceLength := infoDict["piece length"].(int)
	pieces := infoDict["pieces"].(string)
	piecesArr := []Piece{}

	fileOffset := 0
	piecesIndex := 0

	for i := range numOfFiles {
		entry, ok := filesList[i].(map[string]any)
		isLastFile := i == (numOfFiles - 1)

		if !ok {
			return nil, fmt.Errorf("files list contains an invalid entry at index '%d'", i)
		}

		if _, ok := entry["length"].(int); !ok {
			return nil, fmt.Errorf("files list entry at index '%d' contains an invalid 'length' property", i)
		}

		if _, ok := entry["path"].([]any); !ok {
			return nil, fmt.Errorf("files list entry at index '%d' contains an invalid 'path' property", i)
		}

		paths := entry["path"].([]any)
		pathList := make([]string, len(paths))

		for index, entry := range paths {
			if _, ok := entry.(string); !ok {
				return nil, fmt.Errorf("files list entry at index '%d' contains an invalid 'path' property", i)
			}

			pathList[index] = entry.(string)
		}

		fileLength := entry["length"].(int)
		path := filepath.Join(pathList...)

		pieceStartIndex := piecesIndex / sha1.Size
		pieceEndIndex := pieceStartIndex + (fileLength / pieceLength)

		result, err := parsePiecesHashes(fileLength, pieceLength, pieceStartIndex, pieces[piecesIndex:])

		if err != nil {
			return nil, fmt.Errorf("failed to parse files list entry at index '%d': %w", i, err)
		}

		files[i] = file{
			torrent:         &tr,
			Length:          fileLength,
			Name:            filepath.Join(infoDict["name"].(string), path),
			Offset:          fileOffset,
			pieceEndIndex:   pieceEndIndex,
			pieceStartIndex: pieceStartIndex,
		}
		/*
			If the offset for the next file is not '0' it means the final piece for this file was truncated.
			Given this assertion, we can copy all the parsed pieces except the last piece, seeing as it will be copied
			as the first piece for the next file, unless the current file is the last file in the list.
		*/
		if result.nextFileOffset != 0 && !isLastFile {
			piecesArr = append(piecesArr, result.pieces[:len(result.pieces)-1]...)
		} else {
			piecesArr = append(piecesArr, result.pieces...)
		}

		fileOffset = result.nextFileOffset
		piecesIndex += result.nextPieceStartIndex
	}

	return &torrentInfo{
		files:  files,
		pieces: piecesArr,
	}, nil
}

func parseInfoDict(infoDict map[string]any, tr Torrent) (*torrentInfo, error) {
	for key, value := range map[string]any{"name": "", "piece length": 0, "pieces": ""} {
		if _, exists := infoDict[key]; !exists {
			return nil, fmt.Errorf("metainfo 'info' dictionary is missing required property '%s'", key)
		}

		expectedType := reflect.TypeOf(value)
		receivedType := reflect.TypeOf(infoDict[key])

		if receivedType != expectedType {
			return nil, fmt.Errorf("expected the '%s' property to be of type '%v', but received '%v'", key, expectedType, receivedType)
		}
	}

	if _, ok := infoDict["files"]; ok {
		info, err := parseFilesList(infoDict, tr)

		return info, err
	}

	if _, ok := infoDict["length"]; !ok {
		return nil, fmt.Errorf("metainfo 'info' dictionary must contain a 'files' or 'length' property")
	}

	fileLength, ok := infoDict["length"].(int)

	if !ok {
		return nil, fmt.Errorf("'length' property of metainfo info dictionary must be an integer not %T", fileLength)
	}

	pieceLength := infoDict["piece length"].(int)
	pieceOffset := 0
	piecesHashes := infoDict["pieces"].(string)

	result, err := parsePiecesHashes(fileLength, pieceLength, pieceOffset, piecesHashes)

	if err != nil {
		return nil, fmt.Errorf("failed to parse pieces hashes: %w", err)
	}

	files := []file{{
		torrent:         &tr,
		Length:          fileLength,
		Name:            infoDict["name"].(string),
		Offset:          0,
		pieceEndIndex:   fileLength / pieceLength,
		pieceStartIndex: 0,
	}}

	return &torrentInfo{
		files:  files,
		pieces: result.pieces,
	}, nil
}

func parseMetaInfo(data []byte) (Torrent, error) {
	var torrent Torrent

	decodedValue, _, err := bencode.DecodeValue(data)

	if err != nil {
		return torrent, fmt.Errorf("failed to decode metainfo file: %w", err)
	}

	metainfo, ok := decodedValue.(map[string]any)

	if !ok {
		return torrent, fmt.Errorf("expected metainfo to be a bencoded dictionary, but received '%T'", metainfo)
	}

	for key, value := range map[string]any{"announce": "string", "info": make(map[string]any)} {
		if _, exists := metainfo[key]; !exists {
			return torrent, fmt.Errorf("metainfo dictionary is missing required property '%s'", key)
		}

		expectedType := reflect.TypeOf(value)
		receivedType := reflect.TypeOf(metainfo[key])

		if receivedType != expectedType {
			return torrent, fmt.Errorf("expected the '%s' property to be of type '%v', but received '%v'", key, expectedType, receivedType)
		}
	}

	var announceListErr error
	trackers := utils.NewSet()

	if announceList, ok := metainfo["announce-list"]; ok {
		trackers, announceListErr = parseAnnounceList(announceList)
	} else {
		trackers.Add(metainfo["announce"].(string))
	}

	if announceListErr != nil {
		return torrent, fmt.Errorf("failed to parse announce list: %w", announceListErr)
	}

	torrentInfo, err := parseInfoDict(metainfo["info"].(map[string]any), torrent)

	if err != nil {
		return torrent, fmt.Errorf("failed to parse metainfo 'info' dictionary %w", err)
	}

	bencodedValue, err := bencode.EncodeValue(metainfo["info"])

	if err != nil {
		return torrent, fmt.Errorf("failed to encode metainfo 'info' dictionary")
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	torrent.ctx = ctx
	torrent.cancelFunc = cancelFunc

	torrent.info = torrentInfo
	torrent.infoHash = sha1.Sum([]byte(bencodedValue))

	torrent.incomingPeersCh = make(chan []Peer, 1)
	torrent.maxPeerConnections = 10
	torrent.metadataPeersCh = make(chan PeerConnection, 10)
	torrent.peerConnections = map[string]PeerConnection{}
	torrent.peers = make(map[string]Peer)
	torrent.failingPeers = make(map[string]Peer)
	torrent.statusCh = make(chan torrentStatus, 1)
	torrent.trackers = *trackers

	return torrent, nil
}
