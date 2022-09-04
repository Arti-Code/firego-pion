package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"github.com/pion/webrtc/v3"
	"google.golang.org/api/option"
)

type Session struct {
	offer  string
	answer string
}

func clearSession(db *db.Client, ctx *context.Context, device string) {
	refSession := db.NewRef("signaling/" + device)
	var emptySession Session = Session{offer: "", answer: ""}
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

func main() {
	var deviceName string = "biesior"
	ctx, conf, opt := initFirebase(deviceName, "https://firego-pion-default-rtdb.firebaseio.com", "key.json")
	app := initApp(ctx, conf, opt)
	db := initDataBase(app, ctx)
	pc := initPeerConnection()
	listener := initStreamListener()

	defer func() {
		var err error
		if err = listener.Close(); err != nil {
			panic(err)
		}
	}()

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		panic(err)
	}
	rtpSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		panic(err)
	}

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
	//signal.Decode(signal.MustReadStdin(), &offer)

	if err = pc.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	if err = pc.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	<-gatherComplete

	//fmt.Println(signal.Encode(*pc.LocalDescription()))

	inboundRTPPacket := make([]byte, 1600) // UDP MTU
	for {
		n, _, err := listener.ReadFrom(inboundRTPPacket)
		if err != nil {
			panic(fmt.Sprintf("error during read: %s", err))
		}

		if _, err = videoTrack.Write(inboundRTPPacket[:n]); err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				return
			}

			panic(err)
		}
	}

	refSdl := db.NewRef("signaling/" + deviceName + "/offer")
	var sdl string = ""
	var offer_accepted bool = false
	for !offer_accepted {
		err = refSdl.Get(ctx, &sdl)
		if err != nil {
			e := fmt.Errorf("error getting sdl: %v", err)
			fmt.Println(e)
		}
		fmt.Println("checking offer...")
		if sdl != "" {
			offer_accepted = true
		} else {
			time.Sleep(time.Second * 5)
		}
	}
	fmt.Println("OFFER:")
	fmt.Println(sdl)
}
