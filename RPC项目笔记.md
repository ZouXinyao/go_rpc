# 协议与编解码

## 1. RPC调用过程

* 客户端发送请求，这个请求包括了：服务名Arith，方法名Multiply，方法使用的参数args；
* 请求到达服务端，经过处理，服务端做出响应，包括：错误信息error，返回值reply。

## 2. 根据调用过程可以确定通信协议

* head部分：请求的服务名；请求的方法名；响应的错误信息。
* body部分：请求的参数；响应的返回值。

## 3. head部分的结构

```go
type Header struct {
	ServiceMethod string // 服务名和方法名，一般是 service.method 的格式
	Seq           uint64 // 请求序列号，区分不同的请求。
	Error         string // 反馈的错误信息
}
```

## 4. 编解码

Codec是用于编解码的接口，根据这个接口，可以实现不同的编解码方式
```go
type Codec interface {
	io.Closer
	ReadHeader(*Header) error   // 解码head
	ReadBody(interface{}) error // 解码body
	Write(*Header, interface{}) error
}
```

所有的编解码的方式采用键值对的形式存在一个全局的map中，key是编解码方式的名称；val是其构造函数。

## 5. 编解码的具体实现：GobCodec

```go
type GobCodec struct {
	conn io.ReadWriteCloser // 建立socket时的链接实例
	buf  *bufio.Writer      // 写缓冲，防止阻塞
	dec  *gob.Decoder       // 解码
	enc  *gob.Encoder       // 编码
}
```

1. 构造函数NewGobCodec，在创建GobCodec实例时就指定好从哪个socket取数据进行解码；将编码后的数据存放在内部创建的buffer中。

```go
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn), // 从conn中取数据进行解码
		enc:  gob.NewEncoder(buf),  // 编码后的数据存入buf，Encode(*)内部实现了存逻辑
	}
}
```

2. 在实现的Codec接口的方法中看不到将数据存入的过程是因为Decode()和Encoder()方法中已经将read和write的逻辑包含了：

   * 解码逻辑：将conn中的内容解码到传入的实例中。
   * 编码逻辑：将数据编码到buffer(内存)中，然后再存入其他结构，比如磁盘中。

# 服务端

## 1. 控制信息相关协议

* 1条rpc请求包含的内容：1条控制信息+n个具体的请求(每个请求方法都是1个head和1个body)
* 控制信息用Option表示，采用JSON编码格式。
* head就是上文提到的Header
* body应该为interface{}类型。
* Option包括MagicNumber(rpc的标识，表示这个请求是一个rpc)和CodecType(这个RPC请求中所有请求方法的编码方式)
* 服务端处理请求时，先用JSON解析Option，确定这是一个RPC请求，及其后序的解码方式，使用CodecType解码剩余内容。
* 1个请求的格式一般是Option在最前面，Header 和 Body 可以有多个：`Option Header1 Body1 Header2 Body2 ...`

```go
type Option struct {
	MagicNumber    int
	CodecType      string // 内容是NewCodecFuncMap全局map中的key。
}
```

## 2. 服务端收到rpc请求后的处理流程

### 从监听socket开始介绍

1. 如果收到rpc请求就创建1个socket(名称为conn)，这时accept处理的，这里和socket编程中的一样。
   * `func (server *Server) Accept(lis net.Listener)`
2. 开启一个goroutine，处理conn。
   * `func (server *Server) ServeConn(conn io.ReadWriteCloser)`
3. 先用json解析Option，通过解析出的内容判断是否是rpc，使用什么解码方法。然后开始解码：
   * `func (server *Server) serveCodec(cc codec.Codec, opt *Option)`
4. 开始根据编解码实例解析后序字段，由于可以发多个请求方法，所以需要一个一个解析请求，然后处理，直到解析出的请求是nil。处理请求也是用新的goroutine进行处理，由于反馈响应时需要避免多个结果混合到一起，所以需要在编码阶段每个结果单独进行。也就是说处理请求可以并发进行，但是对处理后的结果进行编码和发送的过程需要串行。
   * 解析请求：`func (server *Server) readRequest(cc codec.Codec) (*request, error)`
   * 处理请求：`func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration)`
   * 对结果进行编码并反馈响应：`func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex)`

到此为止，服务端在监听socket时收到rpc请求、解析、处理请求、编码反馈，整个过程都完成了。

三、客户端

四、服务注册

学习<https://geektutu.com/post/geerpc.html>，做的一些总结。
