package codec

import "io"

type Header struct {
	ServiceMethod string // 服务名和方法名，一般是 service.method 的格式
	Seq           uint64 // 请求序列号，区分不同的请求。
	Error         string // 反馈的错误信息
}

// Codec 对消息编解码的接口，可以实现不同的编解码的实例
type Codec interface {
	io.Closer
	ReadHeader(*Header) error   // 解码head
	ReadBody(interface{}) error // 解码body
	Write(*Header, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec

const (
	GobType string = "application/gob"
)

var NewCodecFuncMap map[string]NewCodecFunc

// init 不同的编解码实例的名称(GobType)下存了对应的构造函数。
func init() {
	NewCodecFuncMap = make(map[string]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
