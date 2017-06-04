// Code generated by protoc-gen-gogo.
// source: github.com/evanphx/mesh/protocol/pipe/message.proto
// DO NOT EDIT!

/*
	Package pipe is a generated protocol buffer package.

	It is generated from these files:
		github.com/evanphx/mesh/protocol/pipe/message.proto

	It has these top-level messages:
		Message
*/
package pipe

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

import strconv "strconv"

import bytes "bytes"

import strings "strings"
import github_com_gogo_protobuf_proto "github.com/gogo/protobuf/proto"
import sort "sort"
import reflect "reflect"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type Message_Type int32

const (
	PIPE_OPEN     Message_Type = 0
	PIPE_CLOSE    Message_Type = 1
	PIPE_DATA     Message_Type = 2
	PIPE_UNKNOWN  Message_Type = 3
	PIPE_OPENED   Message_Type = 4
	PIPE_DATA_ACK Message_Type = 5
)

var Message_Type_name = map[int32]string{
	0: "PIPE_OPEN",
	1: "PIPE_CLOSE",
	2: "PIPE_DATA",
	3: "PIPE_UNKNOWN",
	4: "PIPE_OPENED",
	5: "PIPE_DATA_ACK",
}
var Message_Type_value = map[string]int32{
	"PIPE_OPEN":     0,
	"PIPE_CLOSE":    1,
	"PIPE_DATA":     2,
	"PIPE_UNKNOWN":  3,
	"PIPE_OPENED":   4,
	"PIPE_DATA_ACK": 5,
}

func (Message_Type) EnumDescriptor() ([]byte, []int) { return fileDescriptorMessage, []int{0, 0} }

type Message struct {
	Session   uint64       `protobuf:"varint,1,opt,name=session,proto3" json:"session,omitempty"`
	SeqId     uint64       `protobuf:"varint,2,opt,name=seq_id,json=seqId,proto3" json:"seq_id,omitempty"`
	Type      Message_Type `protobuf:"varint,3,opt,name=type,proto3,enum=pipe.Message_Type" json:"type,omitempty"`
	PipeName  string       `protobuf:"bytes,4,opt,name=pipe_name,json=pipeName,proto3" json:"pipe_name,omitempty"`
	Data      []byte       `protobuf:"bytes,5,opt,name=data,proto3" json:"data,omitempty"`
	Encrypted bool         `protobuf:"varint,6,opt,name=encrypted,proto3" json:"encrypted,omitempty"`
}

func (m *Message) Reset()                    { *m = Message{} }
func (*Message) ProtoMessage()               {}
func (*Message) Descriptor() ([]byte, []int) { return fileDescriptorMessage, []int{0} }

func init() {
	proto.RegisterType((*Message)(nil), "pipe.Message")
	proto.RegisterEnum("pipe.Message_Type", Message_Type_name, Message_Type_value)
}
func (x Message_Type) String() string {
	s, ok := Message_Type_name[int32(x)]
	if ok {
		return s
	}
	return strconv.Itoa(int(x))
}
func (this *Message) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
	}

	that1, ok := that.(*Message)
	if !ok {
		that2, ok := that.(Message)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	if this.Session != that1.Session {
		return false
	}
	if this.SeqId != that1.SeqId {
		return false
	}
	if this.Type != that1.Type {
		return false
	}
	if this.PipeName != that1.PipeName {
		return false
	}
	if !bytes.Equal(this.Data, that1.Data) {
		return false
	}
	if this.Encrypted != that1.Encrypted {
		return false
	}
	return true
}
func (this *Message) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 10)
	s = append(s, "&pipe.Message{")
	s = append(s, "Session: "+fmt.Sprintf("%#v", this.Session)+",\n")
	s = append(s, "SeqId: "+fmt.Sprintf("%#v", this.SeqId)+",\n")
	s = append(s, "Type: "+fmt.Sprintf("%#v", this.Type)+",\n")
	s = append(s, "PipeName: "+fmt.Sprintf("%#v", this.PipeName)+",\n")
	s = append(s, "Data: "+fmt.Sprintf("%#v", this.Data)+",\n")
	s = append(s, "Encrypted: "+fmt.Sprintf("%#v", this.Encrypted)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringMessage(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func extensionToGoStringMessage(m github_com_gogo_protobuf_proto.Message) string {
	e := github_com_gogo_protobuf_proto.GetUnsafeExtensionsMap(m)
	if e == nil {
		return "nil"
	}
	s := "proto.NewUnsafeXXX_InternalExtensions(map[int32]proto.Extension{"
	keys := make([]int, 0, len(e))
	for k := range e {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	ss := []string{}
	for _, k := range keys {
		ss = append(ss, strconv.Itoa(k)+": "+e[int32(k)].GoString())
	}
	s += strings.Join(ss, ",") + "})"
	return s
}
func (m *Message) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Message) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Session != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintMessage(dAtA, i, uint64(m.Session))
	}
	if m.SeqId != 0 {
		dAtA[i] = 0x10
		i++
		i = encodeVarintMessage(dAtA, i, uint64(m.SeqId))
	}
	if m.Type != 0 {
		dAtA[i] = 0x18
		i++
		i = encodeVarintMessage(dAtA, i, uint64(m.Type))
	}
	if len(m.PipeName) > 0 {
		dAtA[i] = 0x22
		i++
		i = encodeVarintMessage(dAtA, i, uint64(len(m.PipeName)))
		i += copy(dAtA[i:], m.PipeName)
	}
	if len(m.Data) > 0 {
		dAtA[i] = 0x2a
		i++
		i = encodeVarintMessage(dAtA, i, uint64(len(m.Data)))
		i += copy(dAtA[i:], m.Data)
	}
	if m.Encrypted {
		dAtA[i] = 0x30
		i++
		if m.Encrypted {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i++
	}
	return i, nil
}

func encodeFixed64Message(dAtA []byte, offset int, v uint64) int {
	dAtA[offset] = uint8(v)
	dAtA[offset+1] = uint8(v >> 8)
	dAtA[offset+2] = uint8(v >> 16)
	dAtA[offset+3] = uint8(v >> 24)
	dAtA[offset+4] = uint8(v >> 32)
	dAtA[offset+5] = uint8(v >> 40)
	dAtA[offset+6] = uint8(v >> 48)
	dAtA[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32Message(dAtA []byte, offset int, v uint32) int {
	dAtA[offset] = uint8(v)
	dAtA[offset+1] = uint8(v >> 8)
	dAtA[offset+2] = uint8(v >> 16)
	dAtA[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintMessage(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *Message) Size() (n int) {
	var l int
	_ = l
	if m.Session != 0 {
		n += 1 + sovMessage(uint64(m.Session))
	}
	if m.SeqId != 0 {
		n += 1 + sovMessage(uint64(m.SeqId))
	}
	if m.Type != 0 {
		n += 1 + sovMessage(uint64(m.Type))
	}
	l = len(m.PipeName)
	if l > 0 {
		n += 1 + l + sovMessage(uint64(l))
	}
	l = len(m.Data)
	if l > 0 {
		n += 1 + l + sovMessage(uint64(l))
	}
	if m.Encrypted {
		n += 2
	}
	return n
}

func sovMessage(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozMessage(x uint64) (n int) {
	return sovMessage(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *Message) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Message{`,
		`Session:` + fmt.Sprintf("%v", this.Session) + `,`,
		`SeqId:` + fmt.Sprintf("%v", this.SeqId) + `,`,
		`Type:` + fmt.Sprintf("%v", this.Type) + `,`,
		`PipeName:` + fmt.Sprintf("%v", this.PipeName) + `,`,
		`Data:` + fmt.Sprintf("%v", this.Data) + `,`,
		`Encrypted:` + fmt.Sprintf("%v", this.Encrypted) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringMessage(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *Message) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMessage
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Message: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Message: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Session", wireType)
			}
			m.Session = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Session |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SeqId", wireType)
			}
			m.SeqId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SeqId |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			m.Type = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Type |= (Message_Type(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PipeName", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthMessage
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.PipeName = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthMessage
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Data = append(m.Data[:0], dAtA[iNdEx:postIndex]...)
			if m.Data == nil {
				m.Data = []byte{}
			}
			iNdEx = postIndex
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Encrypted", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Encrypted = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipMessage(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMessage
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipMessage(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMessage
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMessage
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthMessage
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowMessage
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipMessage(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthMessage = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMessage   = fmt.Errorf("proto: integer overflow")
)

func init() {
	proto.RegisterFile("github.com/evanphx/mesh/protocol/pipe/message.proto", fileDescriptorMessage)
}

var fileDescriptorMessage = []byte{
	// 360 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x4c, 0x90, 0xcd, 0x4e, 0xea, 0x40,
	0x18, 0x86, 0x3b, 0x50, 0x7e, 0xfa, 0x1d, 0xe0, 0xf4, 0x4c, 0x72, 0x92, 0x46, 0xcd, 0xa4, 0x61,
	0x61, 0xba, 0xd0, 0x36, 0x91, 0x2b, 0x40, 0xe8, 0x82, 0xa0, 0x85, 0x54, 0x8c, 0xcb, 0xa6, 0xd0,
	0xb1, 0xd4, 0xd8, 0x1f, 0x68, 0x31, 0xb2, 0xf3, 0x12, 0xbc, 0x01, 0xf7, 0x5e, 0x8a, 0x4b, 0x96,
	0x2e, 0x65, 0xdc, 0xb8, 0xe4, 0x12, 0x4c, 0x07, 0x45, 0x77, 0xdf, 0xfb, 0xbc, 0x79, 0xde, 0x4c,
	0x06, 0x5a, 0x7e, 0x90, 0x4d, 0x17, 0x63, 0x7d, 0x12, 0x87, 0x06, 0xbd, 0x73, 0xa3, 0x64, 0x7a,
	0x6f, 0x84, 0x34, 0x9d, 0x1a, 0xc9, 0x3c, 0xce, 0xe2, 0x49, 0x7c, 0x6b, 0x24, 0x41, 0x42, 0x73,
	0x94, 0xba, 0x3e, 0xd5, 0x39, 0xc5, 0x62, 0xce, 0xf6, 0x8e, 0x7f, 0xa9, 0x7e, 0xec, 0xc7, 0x5b,
	0x65, 0xbc, 0xb8, 0xe6, 0x89, 0x07, 0x7e, 0x6d, 0xa5, 0xe6, 0x53, 0x01, 0x2a, 0xe7, 0xdb, 0x19,
	0xac, 0x40, 0x25, 0xa5, 0x69, 0x1a, 0xc4, 0x91, 0x82, 0x54, 0xa4, 0x89, 0xf6, 0x77, 0xc4, 0xff,
	0xa1, 0x9c, 0xd2, 0x99, 0x13, 0x78, 0x4a, 0x81, 0x17, 0xa5, 0x94, 0xce, 0x7a, 0x1e, 0x3e, 0x04,
	0x31, 0x5b, 0x26, 0x54, 0x29, 0xaa, 0x48, 0x6b, 0x9c, 0x60, 0x3d, 0x7f, 0x80, 0xfe, 0xb5, 0xa6,
	0x8f, 0x96, 0x09, 0xb5, 0x79, 0x8f, 0xf7, 0x41, 0xca, 0x2b, 0x27, 0x72, 0x43, 0xaa, 0x88, 0x2a,
	0xd2, 0x24, 0xbb, 0x9a, 0x03, 0xcb, 0x0d, 0x29, 0xc6, 0x20, 0x7a, 0x6e, 0xe6, 0x2a, 0x25, 0x15,
	0x69, 0x35, 0x9b, 0xdf, 0xf8, 0x00, 0x24, 0x1a, 0x4d, 0xe6, 0xcb, 0x24, 0xa3, 0x9e, 0x52, 0x56,
	0x91, 0x56, 0xb5, 0x7f, 0x40, 0xf3, 0x06, 0xc4, 0x7c, 0x1c, 0xd7, 0x41, 0x1a, 0xf6, 0x86, 0xa6,
	0x33, 0x18, 0x9a, 0x96, 0x2c, 0xe0, 0x06, 0x00, 0x8f, 0x9d, 0xb3, 0xc1, 0x85, 0x29, 0xa3, 0x5d,
	0xdd, 0x6d, 0x8f, 0xda, 0x72, 0x01, 0xcb, 0x50, 0xe3, 0xf1, 0xd2, 0xea, 0x5b, 0x83, 0x2b, 0x4b,
	0x2e, 0xe2, 0xbf, 0xf0, 0x67, 0xe7, 0x9b, 0x5d, 0x59, 0xc4, 0xff, 0xa0, 0xbe, 0x33, 0x9c, 0x76,
	0xa7, 0x2f, 0x97, 0x4e, 0x8f, 0x56, 0x6b, 0x22, 0xbc, 0xae, 0x89, 0xb0, 0x59, 0x13, 0xf4, 0xc0,
	0x08, 0x7a, 0x66, 0x04, 0xbd, 0x30, 0x82, 0x56, 0x8c, 0xa0, 0x37, 0x46, 0xd0, 0x07, 0x23, 0xc2,
	0x86, 0x11, 0xf4, 0xf8, 0x4e, 0x84, 0x71, 0x99, 0x7f, 0x6a, 0xeb, 0x33, 0x00, 0x00, 0xff, 0xff,
	0x4e, 0x84, 0x1c, 0xdf, 0xc0, 0x01, 0x00, 0x00,
}