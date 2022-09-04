package main

import (
	"context"
	"encoding/json"

	//	"errors"
	"fmt"
	//	"io"
	"net"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"github.com/pion/webrtc/v3"
	"google.golang.org/api/option"
)

type Session struct {
	Offer  string `json: "offer"`
	Answer string `json: "answer"`
}

type SDP struct {
	Type string `json: "type"`
	Sdp  string `json: "sdp"`
}

//func (s *Session) getOffer() string {
//	return s.Offer
//}
//
//func (s *Session) getAnswer() string {
//	return s.Answer
//}

func clearSession(db *db.Client, ctx *context.Context, device string) {
	refSession := db.NewRef("signaling/" + device)
	var emptySession Session = Session{Offer: "", Answer: ""}
	err := refSession.Set(*ctx, &emptySession)
	if err != nil {
		e := fmt.Errorf("error during session room clearing: %v", err)
		fmt.Println(e)
	}
}

func initFirebase(device string, dbURL string, privKey string) (ctx context.Context, conf *firebase.Config, opt option.ClientOption) {
	ctx = context.Background()
	conf = &firebase.Config{DatabaseURL: dbURL}
	opt = option.WithCredentialsFile(privKey)
	return
}

func initApp(ctx context.Context, conf *firebase.Config, opt option.ClientOption) *firebase.App {
	app, _ := firebase.NewApp(ctx, conf, opt)
	return app
}

func initDataBase(app *firebase.App, ctx context.Context) *db.Client {
	db, _ := app.Database(ctx)
	return db
}

func initPeerConnection() *webrtc.PeerConnection {
	peerConnection, _ := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}})
	return peerConnection
}

func initStreamListener() *net.UDPConn {
	listener, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5004})
	return listener
}

func waitForOffer(db *db.Client, ctx context.Context, device string) string {
	refSdp := db.NewRef("signaling/" + device)
	//	var sdp map[string]interface{}
	var sdp2 Session
	var offer_accepted bool = false
	for !offer_accepted {
		_ = refSdp.Get(ctx, &sdp2)
		fmt.Println("checking offer...")
		//fmt.Println(sdp2.Offer)
		if sdp2.Offer != "" {
			fmt.Println("new offer founded")
			offer_accepted = true
		} else {
			time.Sleep(time.Second * 5)
		}
	}
	return sdp2.Offer
}

func initVideoTrack() *webrtc.TrackLocalStaticRTP {
	videoTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		panic(err)
	}
	return videoTrack
}

func main() {
	var deviceName string = "biesior"
	ctx, conf, opt := initFirebase(deviceName, "https://firego-pion-default-rtdb.firebaseio.com", "key.json")
	app := initApp(ctx, conf, opt)
	db := initDataBase(app, ctx)
	pc := initPeerConnection()
	listener := initStreamListener()
	//	clearSession(db, &ctx, deviceName)
	defer func() {
		var err error
		if err = listener.Close(); err != nil {
			panic(err)
		}
	}()

	var videoTrack *webrtc.TrackLocalStaticRTP = initVideoTrack()
	rtpSender, _ := pc.AddTrack(videoTrack)

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateFailed {
			if closeErr := pc.Close(); closeErr != nil {
				panic(closeErr)
			}
		}
	})

	offer := webrtc.SessionDescription{}
	//var s SDP
	//var json_offer []byte
	data := waitForOffer(db, ctx, deviceName)
	_ = json.Unmarshal([]byte(data), &offer)
	fmt.Println("OFFER:")
	fmt.Println(offer.Type)
	fmt.Println(offer.SDP)
	//signal.Decode(signal.MustReadStdin(), &offer)

	//	if err = pc.SetRemoteDescription(offer); err != nil {
	//		panic(err)
	//	}
	//
	//	answer, err := pc.CreateAnswer(nil)
	//	if err != nil {
	//		panic(err)
	//	}
	//
	//	gatherComplete := webrtc.GatheringCompletePromise(pc)
	//
	//	if err = pc.SetLocalDescription(answer); err != nil {
	//		panic(err)
	//	}
	//
	//	<-gatherComplete
	//
	//	fmt.Println(signal.Encode(*pc.LocalDescription()))
	//
	//	inboundRTPPacket := make([]byte, 1600) // UDP MTU
	//	for {
	//		n, _, err := listener.ReadFrom(inboundRTPPacket)
	//		if err != nil {
	//			panic(fmt.Sprintf("error during read: %s", err))
	//		}
	//
	//		if _, err = videoTrack.Write(inboundRTPPacket[:n]); err != nil {
	//			if errors.Is(err, io.ErrClosedPipe) {
	//				return
	//			}
	//
	//			panic(err)
	//		}
	//	}
}
