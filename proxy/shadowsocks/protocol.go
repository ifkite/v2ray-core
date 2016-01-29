package shadowsocks

import (
	"io"

	"github.com/v2ray/v2ray-core/common/alloc"
	"github.com/v2ray/v2ray-core/common/log"
	v2net "github.com/v2ray/v2ray-core/common/net"
	"github.com/v2ray/v2ray-core/common/serial"
	"github.com/v2ray/v2ray-core/transport"
)

const (
	AddrTypeIPv4   = 1
	AddrTypeIPv6   = 4
	AddrTypeDomain = 3
)

type Request struct {
	Address v2net.Address
	Port    v2net.Port
	OTA     bool
}

func ReadRequest(reader io.Reader, auth *Authenticator) (*Request, error) {
	buffer := alloc.NewSmallBuffer()
	defer buffer.Release()

	_, err := io.ReadFull(reader, buffer.Value[:1])
	if err != nil {
		log.Error("Shadowsocks: Failed to read address type: ", err)
		return nil, transport.CorruptedPacket
	}
	lenBuffer := 1

	request := new(Request)

	addrType := (buffer.Value[0] & 0x0F)
	if (buffer.Value[0] & 0x10) == 0x10 {
		request.OTA = true
	}
	switch addrType {
	case AddrTypeIPv4:
		_, err := io.ReadFull(reader, buffer.Value[lenBuffer:lenBuffer+4])
		if err != nil {
			log.Error("Shadowsocks: Failed to read IPv4 address: ", err)
			return nil, transport.CorruptedPacket
		}
		request.Address = v2net.IPAddress(buffer.Value[lenBuffer : lenBuffer+4])
		lenBuffer += 4
	case AddrTypeIPv6:
		_, err := io.ReadFull(reader, buffer.Value[lenBuffer:lenBuffer+16])
		if err != nil {
			log.Error("Shadowsocks: Failed to read IPv6 address: ", err)
			return nil, transport.CorruptedPacket
		}
		request.Address = v2net.IPAddress(buffer.Value[lenBuffer : lenBuffer+16])
		lenBuffer += 16
	case AddrTypeDomain:
		_, err := io.ReadFull(reader, buffer.Value[lenBuffer:lenBuffer+1])
		if err != nil {
			log.Error("Shadowsocks: Failed to read domain lenth: ", err)
			return nil, transport.CorruptedPacket
		}
		domainLength := int(buffer.Value[lenBuffer])
		lenBuffer++
		_, err = io.ReadFull(reader, buffer.Value[lenBuffer:lenBuffer+domainLength])
		if err != nil {
			log.Error("Shadowsocks: Failed to read domain: ", err)
			return nil, transport.CorruptedPacket
		}
		request.Address = v2net.DomainAddress(string(buffer.Value[lenBuffer : lenBuffer+domainLength]))
		lenBuffer += domainLength
	default:
		log.Error("Shadowsocks: Unknown address type: ", addrType)
		return nil, transport.CorruptedPacket
	}

	_, err = io.ReadFull(reader, buffer.Value[lenBuffer:lenBuffer+2])
	if err != nil {
		log.Error("Shadowsocks: Failed to read port: ", err)
		return nil, transport.CorruptedPacket
	}

	request.Port = v2net.PortFromBytes(buffer.Value[lenBuffer : lenBuffer+2])
	lenBuffer += 2

	if request.OTA {
		authBytes := buffer.Value[lenBuffer : lenBuffer+auth.AuthSize()]
		_, err = io.ReadFull(reader, authBytes)
		if err != nil {
			log.Error("Shadowsocks: Failed to read OTA: ", err)
			return nil, transport.CorruptedPacket
		}

		actualAuth := auth.Authenticate(nil, buffer.Value[0:lenBuffer])
		if !serial.BytesLiteral(actualAuth).Equals(serial.BytesLiteral(authBytes)) {
			log.Error("Shadowsocks: Invalid OTA: ", actualAuth)
			return nil, transport.CorruptedPacket
		}
	}

	return request, nil
}