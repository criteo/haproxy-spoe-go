package spoe

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	helloKeyMaxFrameSize      = "max-frame-size"
	helloKeySupportedVersions = "supported-versions"
	helloKeyVersion           = "version"
	helloKeyCapabilities      = "capabilities"
	helloKeyHealthcheck       = "healthcheck"
	helloKeyEngineID          = "engine-id"

	capabilityAsync      = "async"
	capabilityPipelining = "pipelining"
)

func (c *conn) handleHello(frame Frame) (Frame, map[string]bool, bool, error) {
	data, _, err := decodeKVs(frame.data, -1)
	if err != nil {
		return frame, nil, false, errors.Wrap(err, "hello")
	}

	log.Debugf("spoe: hello from %s: %+v", c.Conn.RemoteAddr(), data)

	remoteFrameSize, ok := data[helloKeyMaxFrameSize].(uint)
	if !ok {
		return frame, nil, false, fmt.Errorf("hello: expected %s", helloKeyMaxFrameSize)
	}

	connFrameSize := remoteFrameSize
	if connFrameSize > maxFrameSize {
		connFrameSize = maxFrameSize
	}

	c.frameSize = int(connFrameSize)

	remoteSupportedVersions, ok := data[helloKeySupportedVersions].(string)
	if !ok {
		return frame, nil, false, fmt.Errorf("hello: expected %s", helloKeyVersion)
	}

	versionOK := false
	for _, supportedVersion := range strings.Split(remoteSupportedVersions, ",") {
		remoteVersion, err := parseVersion(supportedVersion)
		if err != nil {
			return frame, nil, false, errors.Wrap(err, "hello")
		}

		if remoteVersion[0] == 2 {
			versionOK = true
		}
	}

	if !versionOK {
		return frame, nil, false, fmt.Errorf("hello: incompatible version %s, need %s", remoteSupportedVersions, version)
	}

	healthcheck, _ := data[helloKeyHealthcheck].(bool)

	remoteCapabilitiesStr, ok := data[helloKeyCapabilities].(string)
	if !ok {
		return frame, nil, false, fmt.Errorf("hello: expected %s", helloKeyCapabilities)
	}
	remoteCapabilities := parseCapabilities(remoteCapabilitiesStr)
	if !remoteCapabilities[capabilityPipelining] && !healthcheck {
		// HAProxy never sends pipelining capability on healthcheck hellos
		return frame, nil, false, fmt.Errorf("hello: expected pipelining capability")
	}

	c.engineID, _ = data[helloKeyEngineID].(string)
	if len(c.engineID) == 0 && !healthcheck {
		// HAProxy never sends engine-id on healthcheck hellos
		return frame, nil, false, fmt.Errorf("hello: engine-id not found")
	}

	frame.ftype = frameTypeAgentHello
	frame.flags = frameFlagFin
	frame.data = frame.originalData

	off := 0
	n, err := encodeKV(frame.data[off:], helloKeyVersion, version)
	if err != nil {
		return frame, nil, false, errors.Wrap(err, "hello")
	}
	off += n

	n, err = encodeKV(frame.data[off:], helloKeyMaxFrameSize, connFrameSize)
	if err != nil {
		return frame, nil, false, errors.Wrap(err, "hello")
	}
	off += n

	localCapabilities := []string{capabilityPipelining}
	if remoteCapabilities[capabilityAsync] {
		localCapabilities = append(localCapabilities, capabilityAsync)
	}

	n, err = encodeKV(frame.data[off:], helloKeyCapabilities, strings.Join(localCapabilities, ","))
	if err != nil {
		return frame, nil, false, errors.Wrap(err, "hello")
	}
	off += n

	frame.data = frame.data[:off]

	return frame, remoteCapabilities, healthcheck, nil
}

func parseCapabilities(capas string) map[string]bool {
	caps := map[string]bool{}
	for _, s := range strings.Split(capas, ",") {
		caps[s] = true
	}
	return caps
}

func checkCapabilities(capas string) bool {
	hasPipelining := false

	for _, s := range strings.Split(capas, ",") {
		switch s {
		case capabilityPipelining:
			hasPipelining = true
		}
	}

	return hasPipelining
}

func parseVersion(v string) ([]int, error) {
	res := []int{}

	v = strings.TrimSpace(v)
	s, e := 0, 0

	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == '.' {
			n, err := strconv.Atoi(v[s:e])
			if err != nil {
				return nil, errors.Wrap(err, "version parse")
			}
			res = append(res, n)
			s = i + 1
			e = s
			continue
		}
		if v[i] >= '0' && v[i] <= '9' {
			e++
			continue
		}
		return nil, fmt.Errorf("version parse: unexpected char %s", string(v[i]))
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("version parse: expected at least one digit")
	}

	return res, nil
}
