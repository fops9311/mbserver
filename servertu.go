package mbserver

import (
	"io"
	"log"
	"time"

	"github.com/goburrow/serial"
)

// ListenRTU starts the Modbus server listening to a serial device.
// For example:  err := s.ListenRTU(&serial.Config{Address: "/dev/ttyUSB0"})
func (s *Server) ListenRTU(serialConfig *serial.Config) (err error) {
	port, err := serial.Open(serialConfig)
	if err != nil {
		log.Fatalf("failed to open %s: %v\n", serialConfig.Address, err)
	}
	s.ports = append(s.ports, port)

	s.portsWG.Add(1)
	go func() {
		defer s.portsWG.Done()
		s.acceptSerialRequests(port)
	}()

	return err
}

func (s *Server) acceptSerialRequests(port serial.Port) {
SkipFrameError:
	for {
		select {
		case <-s.portsCloseChan:
			return
		default:
		}

		getMessageBuffer := func(port serial.Port) ([]byte, error) {
			var result []byte = make([]byte, 0)
			for len(result) < 4 {
				readbuffer := make([]byte, 512)
				bytesRead, err := port.Read(readbuffer)
				if err != nil {
					return result, err
				}
				<-time.NewTimer(time.Millisecond * 20).C
				for _, v := range readbuffer[:bytesRead] {
					result = append(result, v)
				}
			}
			return result, nil
		}

		buffer, err := getMessageBuffer(port)
		if err != nil {
			if err != io.EOF {
				log.Printf("serial read error %v\n", err)
			}
			return
		}
		bytesRead := len(buffer)

		if bytesRead != 0 {

			// Set the length of the packet to the number of read bytes.
			packet := buffer[:bytesRead]

			frame, err := NewRTUFrame(packet)
			if err != nil {
				log.Printf("bad serial frame error %v\n", err)
				//The next line prevents RTU server from exiting when it receives a bad frame. Simply discard the erroneous
				//frame and wait for next frame by jumping back to the beginning of the 'for' loop.
				log.Printf("Keep the RTU server running!!\n")
				continue SkipFrameError
				//return
			}

			resp := make(chan Framer)
			request := &Request{port, frame, resp}

			s.requestChan <- request
			//time dependancy
			//<-time.NewTimer(time.Millisecond * 20).C
			response := <-resp
			request.conn.Write(append(response.Bytes()))
		}
	}
}
