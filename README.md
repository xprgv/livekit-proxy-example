# Livekit proxy example

Simple app for demonstrating how to proxy webrtc tracks between 2 livekit rooms

## Usage
```sh
go run main.go \
    --room_name=<room_name> \
    --subscriber_url=<some_remote_url> \
    --subscriber_api_key=<some_sub_api_key> \
    --subscriber_api_secret=<some_sub_api_secret> \
    --publisher_url=<some_local_url> \
    --publisher_api_key=<some_pub_api_key> \
    --publisher_api_secret=<some_pub_api_secret> \
```