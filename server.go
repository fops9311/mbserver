// Package mbserver implments a Modbus server (slave).
package mbserver

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/tarm/serial"
)

const VERSION = "0.0.11"

// Server is a Modbus slave with allocated memory for discrete inputs, coils, etc.
type Server struct {
	// Debug enables more verbose messaging.
	Debug            bool
	listeners        []net.Listener
	ports            []*serial.Port
	portsWG          sync.WaitGroup
	portsCloseChan   chan struct{}
	requestChan      chan *Request
	function         [256](func(*Server, Framer) ([]byte, *Exception))
	DiscreteInputs   []byte
	Coils            []byte
	HoldingRegisters []uint16
	InputRegisters   []uint16
}

// Request contains the connection and Modbus frame.
type Request struct {
	conn     io.ReadWriteCloser
	frame    Framer
	responce chan Framer
}

// NewServer creates a new Modbus server (slave).
func NewServer() *Server {
	s := &Server{}

	// Allocate Modbus memory maps.
	s.DiscreteInputs = make([]byte, 65536)
	s.Coils = make([]byte, 65536)
	s.HoldingRegisters = make([]uint16, 65536)
	s.InputRegisters = make([]uint16, 65536)

	// Add default functions.
	s.function[1] = ReadCoils
	s.function[2] = ReadDiscreteInputs
	s.function[3] = ReadHoldingRegisters
	s.function[4] = ReadInputRegisters
	s.function[5] = WriteSingleCoil
	s.function[6] = WriteHoldingRegister
	s.function[15] = WriteMultipleCoils
	s.function[16] = WriteHoldingRegisters

	s.requestChan = make(chan *Request)
	s.portsCloseChan = make(chan struct{})

	go s.handler()

	return s
}

// RegisterFunctionHandler override the default behavior for a given Modbus function.
func (s *Server) RegisterFunctionHandler(funcCode uint8, function func(*Server, Framer) ([]byte, *Exception)) {
	s.function[funcCode] = function
}

func (s *Server) handle(request *Request) Framer {
	var exception *Exception
	var data []byte

	response := request.frame.Copy()

	function := request.frame.GetFunction()
	if s.function[function] != nil {
		data, exception = s.function[function](s, request.frame)
		response.SetData(data)
	} else {
		exception = &IllegalFunction
	}

	if exception != &Success {
		response.SetException(exception)
	}

	return response
}

// All requests are handled synchronously to prevent modbus memory corruption.
func (s *Server) handler() {
	var i byte = 0
	for {
		if s.Debug {
			log.Printf("handle #%d; start----\n", i)
		}
		request := <-s.requestChan

		if s.Debug {
			log.Printf("handle #%d; got request: %v\n", i, request.frame.Bytes())
		}
		response := s.handle(request)
		if s.Debug {
			log.Printf("handle #%d; finish\n", i)
		}
		if s.Debug {
			log.Printf("handle #%d; send responce\n", i)
		}
		request.responce <- response
		if s.Debug {
			log.Printf("handle #%d; end----\n", i)
		}
		i++
	}
}

// Close stops listening to TCP/IP ports and closes serial ports.
func (s *Server) Close() {
	for _, listen := range s.listeners {
		listen.Close()
	}

	close(s.portsCloseChan)
	s.portsWG.Wait()

	for _, port := range s.ports {
		port.Close()
	}
}
