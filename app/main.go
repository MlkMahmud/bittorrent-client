package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/app/client"
	"github.com/codecrafters-io/bittorrent-starter-go/app/torrent"
)

func main() {
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	command := os.Args[1]

	switch command {
	case "decode":
		{
			bencodedValue := os.Args[2]
			decoded, _, err := bencode.DecodeValue([]byte(bencodedValue))

			if err != nil {
				log.Fatal(err)
				return
			}

			jsonString, err := json.Marshal(decoded)

			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(jsonString))
			return
		}

	case "download":
		{
			dest := os.Args[3]
			torrentFilePath := os.Args[4]

			if err := client.Download(torrentFilePath, dest); err != nil {
				log.Fatal(err)
			}

			return
		}

	case "download_piece":
		{
			dest := os.Args[3]
			torrentFilePath := os.Args[4]
			pieceIndex, err := strconv.Atoi(os.Args[5])

			if err != nil {
				log.Fatal(err)
			}

			trrnt, err := torrent.NewTorrent(torrentFilePath)

			if err != nil {
				log.Fatal(err)
			}

			peers, err := trrnt.GetPeers()

			if err != nil {
				log.Fatal(err)
			}

			peerConnection := torrent.NewPeerConnection(torrent.PeerConnectionConfig{Peer: peers[0]})

			if err := peerConnection.InitConnection(); err != nil {
				log.Fatal(err)
			}

			downloadedPiece, err := peerConnection.DownloadPiece(trrnt.Info.Pieces[pieceIndex])

			if err != nil {
				log.Fatal(err)
			}

			if err := os.WriteFile(dest, downloadedPiece.Data, 0644); err != nil {
				log.Fatal(err)
			}

			return
		}

	case "handshake":
		{

			trrnt, err := torrent.NewTorrent(os.Args[2])

			if err != nil {
				log.Fatal(err)
			}

			peerAddress := os.Args[3]
			addressParts := strings.Split(peerAddress, ":")
			host := addressParts[0]

			portNum, err := strconv.ParseUint(addressParts[1], 10, 16)

			if err != nil {
				log.Fatal(err)
			}

			port := uint16(portNum)
			peer := torrent.Peer{InfoHash: trrnt.InfoHash, IpAddress: host, Port: port}

			peerConnection := torrent.NewPeerConnection(torrent.PeerConnectionConfig{Peer: peer})

			if err := peerConnection.InitConnection(); err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Peer ID: %x\n", peerConnection.PeerId)
		}

	case "info":
		{
			torrentFilePath := os.Args[2]
			trrnt, err := torrent.NewTorrent(torrentFilePath)

			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Tracker URL: %s\n", trrnt.TrackerUrl)
			fmt.Printf("Length: %d\n", trrnt.Info.Length)
			fmt.Printf("Info Hash: %x\n", trrnt.InfoHash)
			fmt.Printf("Piece Length: %d\n", trrnt.Info.Pieces[0].Length)
			fmt.Println("Piece Hashes:")
			for _, piece := range trrnt.Info.Pieces {
				fmt.Printf("%x\n", piece.Hash)
			}

			return
		}

	case "magnet_download":
		{
			dest := os.Args[3]
			torrentFilePath := os.Args[4]

			if err := client.Download(torrentFilePath, dest); err != nil {
				log.Fatal(err)
			}

			return
		}

	case "magnet_download_piece":
		{
			dest := os.Args[3]
			torrentFilePath := os.Args[4]
			pieceIndex, err := strconv.Atoi(os.Args[5])

			if err != nil {
				log.Fatal(err)
			}

			trrnt, err := torrent.NewTorrent(torrentFilePath)

			if err != nil {
				log.Fatal(err)
			}

			peers, err := trrnt.GetPeers()

			if err != nil {
				log.Fatal(err)
			}

			if err := trrnt.DownloadMetadata(); err != nil {
				log.Fatal(err)
			}

			peerConnection := torrent.NewPeerConnection(torrent.PeerConnectionConfig{Peer: peers[0]})

			if err := peerConnection.InitConnection(); err != nil {
				log.Fatal(err)
			}

			downloadedPiece, err := peerConnection.DownloadPiece(trrnt.Info.Pieces[pieceIndex])

			if err != nil {
				log.Fatal(err)
			}

			if err := os.WriteFile(dest, downloadedPiece.Data, 0644); err != nil {
				log.Fatal(err)
			}

			return
		}

	case "magnet_handshake":
		{
			magnetLink := os.Args[2]

			trrnt, err := torrent.NewTorrent(magnetLink)

			if err != nil {
				log.Fatal(err)
			}

			peers, err := trrnt.GetPeers()

			if err != nil {
				log.Fatal(err)
			}

			peerConnection := torrent.NewPeerConnection(torrent.PeerConnectionConfig{Peer: peers[0]})
			initErr := peerConnection.InitConnection()

			if peerConnection.Conn != nil {
				defer peerConnection.Conn.Close()
			}

			if initErr != nil {
				log.Fatal(initErr)
			}

			fmt.Printf("Peer ID: %x\n", peerConnection.PeerId)

			if peerConnection.SupportsExtensions {
				fmt.Printf("Peer Metadata Extension ID: %d\n", peerConnection.PeerExtensions[torrent.Metadata])
			}

			return
		}

	case "magnet_info":
		{
			magnetLink := os.Args[2]

			trrnt, err := torrent.NewTorrent(magnetLink)

			if err != nil {
				log.Fatal(err)
			}

			if err := trrnt.DownloadMetadata(); err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Tracker URL: %s\n", trrnt.TrackerUrl)
			fmt.Printf("Length: %d\n", trrnt.Info.Length)
			fmt.Printf("Info Hash: %x\n", trrnt.InfoHash)
			fmt.Printf("Piece Length: %d\n", trrnt.Info.Pieces[0].Length)
			fmt.Println("Piece Hashes:")
			for _, piece := range trrnt.Info.Pieces {
				fmt.Printf("%x\n", piece.Hash)
			}

			return
		}

	case "magnet_parse":
		{
			magnetLink := os.Args[2]

			trrnt, err := torrent.NewTorrent(magnetLink)

			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Tracker URL: %s\n", trrnt.TrackerUrl)
			fmt.Printf("Info Hash: %x\n", trrnt.InfoHash)

			return
		}

	case "peers":
		{
			torrentFilePath := os.Args[2]
			trrnt, err := torrent.NewTorrent(torrentFilePath)

			if err != nil {
				log.Fatal(err)
			}

			peers, err := trrnt.GetPeers()

			if err != nil {
				log.Fatal(err)
			}

			for _, peer := range peers {
				fmt.Printf("%s:%d\n", peer.IpAddress, peer.Port)
			}

			return
		}

	default:
		{
			fmt.Println("Unknown command: " + command)
			os.Exit(1)
		}
	}
}
