package livestream

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/yutopp/go-flv"
	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

type RTMPServerHandler struct {
	rtmp.DefaultHandler
	flvFile *os.File
	flvEnc  *flv.Encoder
}

func (h *RTMPServerHandler) OnServe(conn *rtmp.Conn)  {
}

func (h *RTMPServerHandler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Printf("RTMP connection established from %s", cmd)
	return nil
}

func (h *RTMPServerHandler) OnPublish(_ *rtmp.StreamContext, timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
	log.Printf("RTMP publish from %s", cmd)
	if cmd.PublishingName == "" {
		return errors.New("publishing name is required")
	}
	p := filepath.Join(
		os.TempDir(),
		filepath.Clean(filepath.Join("/", fmt.Sprintf("%s.flv", cmd.PublishingName))),
	)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return errors.Wrap(err, "failed to open file")
	}
	defer f.Close()

	h.flvFile = f

	enc, err := flv.NewEncoder(f, flv.FlagsAudio|flv.FlagsVideo)
	if err != nil {
		_ = f.Close()
		return errors.Wrap(err, "Failed to create flv encoder") 
	}
	h.flvEnc = enc
	return nil
}

func (h *RTMPServerHandler) OnPlay(timestamp uint32, cmd *rtmpmsg.NetStreamPlay) error {
	log.Printf("RTMP play from %s", cmd)
	return nil
}

func (h *RTMPServerHandler) OnCreateStream(timestamp uint32, cmd *rtmpmsg.NetConnectionCreateStream) error {
	log.Printf("RTMP create stream from %s", cmd)
	return nil
}

func (h *RTMPServerHandler) OnSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {
	r := bytes.NewReader(data.Payload)

	var script flvtag.ScriptData
	if err := flvtag.DecodeScriptData(r, &script); err != nil {
		return errors.Wrap(err, "failed to decode script data")
	}
	log.Printf("RTMP script data: %+v", script)

	if err := h.flvEnc.Encode(&flvtag.FlvTag{
		TagType: flvtag.TagTypeScriptData,
		Timestamp: timestamp,
		Data: &script,
	}); err != nil {
		return errors.Wrap(err, "failed to write script data tag")
	}

	return nil
}
