package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/livekit/server-sdk-go/pkg/samplebuilder"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
)

var (
	roomName = flag.String("room_name", "", "")

	subscriberUrl       = flag.String("subscriber_url", "", "")
	subscriberApiKey    = flag.String("subscriber_api_key", "", "")
	subscriberApiSecret = flag.String("subscriber_api_secret", "", "")

	publisherUrl       = flag.String("publisher_url", "", "")
	publisherApiKey    = flag.String("publisher_api_key", "", "")
	publisherApiSecret = flag.String("publisher_api_secret", "", "")
)

func main() {
	flag.Parse()

	publisherRoom, err := lksdk.ConnectToRoom(
		*publisherUrl,
		lksdk.ConnectInfo{
			RoomName:            *roomName,
			APIKey:              *publisherApiKey,
			APISecret:           *publisherApiSecret,
			ParticipantName:     "publisher",
			ParticipantIdentity: "publisher-" + uuid.NewString(),
		},
		&lksdk.RoomCallback{},
	)
	if err != nil {
		log.Fatal("Failed to connect to publish room: ", err.Error())
	}

	subscriberRoom, err := lksdk.ConnectToRoom(
		*subscriberUrl,
		lksdk.ConnectInfo{
			RoomName:            *roomName,
			APIKey:              *subscriberApiKey,
			APISecret:           *subscriberApiSecret,
			ParticipantName:     "subscriber",
			ParticipantIdentity: "subscriber-" + uuid.NewString(),
		},
		&lksdk.RoomCallback{
			ParticipantCallback: lksdk.ParticipantCallback{
				OnTrackSubscribed: func(
					incomingTrack *webrtc.TrackRemote,
					incomingPublication *lksdk.RemoteTrackPublication,
					remoteParticipant *lksdk.RemoteParticipant,
				) {
					switch incomingTrack.Codec().MimeType {
					case webrtc.MimeTypeH264:
						log.Println("Proxy", incomingPublication.Name())

						outcomingTrack, err := lksdk.NewLocalSampleTrack(
							webrtc.RTPCodecCapability{MimeType: incomingTrack.Codec().MimeType},
							lksdk.WithRTCPHandler(func(p rtcp.Packet) {
								switch p.(type) {
								case *rtcp.PictureLossIndication:
									remoteParticipant.WritePLI(incomingTrack.SSRC())
								}
							}),
						)
						if err != nil {
							log.Fatal("Failed to create track: ", err.Error())
						}

						if _, err := publisherRoom.LocalParticipant.PublishTrack(outcomingTrack, &lksdk.TrackPublicationOptions{
							Name: incomingPublication.Name(),
						}); err != nil {
							log.Fatal("Failed to publish track: ", err.Error())
						}

						sb := samplebuilder.New(
							1000,
							&codecs.H264Packet{},
							incomingTrack.Codec().ClockRate,
							samplebuilder.WithPacketDroppedHandler(func() {
								remoteParticipant.WritePLI(incomingTrack.SSRC())
							}),
						)

						for {
							rtpPacket, _, err := incomingTrack.ReadRTP()
							if err != nil {
								if err == io.EOF {
									return
								}
								log.Fatal("Failed to read rtp packet from incoming webrtc track: ", err.Error())
							}

							sb.Push(rtpPacket)
							for {
								sample := sb.Pop()
								if sample == nil {
									break
								}
								if err := outcomingTrack.WriteSample(*sample, nil); err != nil {
									log.Fatal("Failed to write media samle: ", err.Error())
								}
							}
						}

					default:
						log.Println("Proxying track with codec ", incomingTrack.Codec().MimeType, " not supported")
					}
				},
			},
		},
	)
	if err != nil {
		log.Fatal("Failed to connect to subscriber room ", err.Error())
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Println("Shutting down due to signal: ", sig)

	publisherRoom.Disconnect()
	subscriberRoom.Disconnect()

	os.Exit(1)
}
