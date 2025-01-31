package mbserver

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tarm/serial"
)

// ListenRTU starts the Modbus server listening to a serial device.
// For example:  err := s.ListenRTU(&serial.Config{Address: "/dev/ttyUSB0"})
func (s *Server) ListenRTU(serialConfig *serial.Config) (err error) {
	port, err := serial.OpenPort(serialConfig)
	if err != nil {
		log.Fatalf("failed to open %s: %v\n", serialConfig.Name, err)
	}
	s.ports = append(s.ports, port)

	s.portsWG.Add(1)
	go func() {
		defer s.portsWG.Done()
		s.acceptSerialRequests(port)
	}()
	return err
}

var wg sync.WaitGroup

func (s *Server) acceptSerialRequests(port *serial.Port) {
	cominput := scanCom(port, s.Debug)
	for {
		select {
		case <-s.portsCloseChan:
			return
		case buffer := <-cominput:

			if s.Debug {
				log.Printf("acceptSerialRequests got input...\n")
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
					wg.Done()
					continue
					//return
				}

				resp := make(chan Framer)
				request := &Request{port, frame, resp}

				s.requestChan <- request
				response := <-resp

				_, err = request.conn.Write((response.Bytes()))
				wg.Done()

				if s.Debug {
					log.Printf("response data: %v,error: %v\n", response.Bytes(), err)
				}
			}
		}
	}
}

var s = time.Now()

func scanCom(port *serial.Port, debug bool) chan []byte {
	var c chan []byte = make(chan []byte, 20)
	byteChan := make(chan []byte, 20)
	go func() {
		for {
			readbuffer := make([]byte, 1)
			_, err := port.Read(readbuffer)
			if err != nil {
				log.Println(err)
				continue
			}
			byteChan <- readbuffer
		}
	}()
	go func() {
		var result []byte = make([]byte, 0)
		i := 0
		for {
			select {
			case readbuffer := <-byteChan:
				if i == 0 {
					if debug {
						fmt.Println()
					}
				}
				f := time.Now()
				r := f.Sub(s)

				s = f
				result = append(result, readbuffer...)
				if debug {
					log.Print(i, ":", readbuffer, r, "")
				}
				i++
				continue
			case <-time.NewTimer(time.Millisecond * 5).C:

				if debug {
					fmt.Printf(".")
				}
				if len(result) > 0 && result[0] == 1 {
					c <- result
					wg.Add(1)
					wg.Wait()
				} else {
					if debug && len(result) > 0 {
						fmt.Println()
						log.Println("BAD REQUEST", result)
					}
				}
				result = make([]byte, 0)
				i = 0
			}
		}
	}()
	return c
}
