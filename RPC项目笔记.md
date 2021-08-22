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
   `func (server *Server) Accept(lis net.Listener)`
2. 开启一个goroutine，处理conn。  
   `func (server *Server) ServeConn(conn io.ReadWriteCloser)`
3. 先用json解析Option，通过解析出的内容判断是否是rpc，使用什么解码方法。然后开始解码：  
   `func (server *Server) serveCodec(cc codec.Codec, opt *Option)`
4. 开始根据编解码实例解析后序字段，由于可以发多个请求方法，所以需要一个一个解析请求，然后处理，直到解析出的请求是nil。处理请求也是用新的goroutine进行处理，由于反馈响应时需要避免多个结果混合到一起，所以需要在编码阶段每个结果单独进行。也就是说处理请求可以并发进行，但是对处理后的结果进行编码和发送的过程需要串行。
   * 解析请求：`func (server *Server) readRequest(cc codec.Codec) (*request, error)`
   * 处理请求：`func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration)`
   * 对结果进行编码并反馈响应：`func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex)`

到此为止，服务端在监听socket时收到rpc请求、解析、处理请求、编码反馈，整个过程都完成了。

# 客户端

## 1. 承载1次rpc调用需要的信息的结构Call

* 服务和方法名
* 方法的入参
* 服务端反馈的该方法的返回值
* 有可能产生的错误信息
* 需要一个管道字段在rpc处理后通知调用方
* 请求的编号，为了区分是哪个请求

```go
type Call struct {
	Seq           uint64      // 每个请求的唯一编号
	ServiceMethod string      // 服务和方法名：<service>.<method>
	Args          interface{} // 请求方法的入参
	Reply         interface{} // 请求方法的返回值
	Error         error       // 请求的错误信息
	Done          chan *Call  // 请求结束后，用该字段通知调用方。
}
```

## 2. 客户端的数据结构Client

```go
type Client struct {
	cc       codec.Codec      // 编解码
	opt      *server.Option   // 与服务端协商的控制信息
	sending  sync.Mutex       // 保证请求的串行发送，避免多个请求混在一起
	header   codec.Header     // 请求头
	mu       sync.Mutex       // 保证请求有序发送
	seq      uint64           // 请求编号，每个请求唯一
	pending  map[uint64]*Call // 未处理完的请求
	closing  bool             // 用户主动关闭
	shutdown bool             // 错误发送导致
}
```

Call是每次请求承载的信息，Client是保证正确发送请求和接收响应的客户端。

## 3. 发送请求需要的方法

### (1) 关闭客户端

就是正常关闭一个socket。需要注意的是要保证多个客户端正常关闭，需要并发安全。closing标识和socket的close()都执行完成才算关闭了一个客户端。  
`func (c *Client) Close() error`

### (2) 判断客户端是否可用

根据客户端的closing和shutdown字段判断客户端的状态，同样保证客户端某个时刻状态不发生改变，需要并发控制。  
`func (c *Client) IsAvailable() bool`

### (3) 创建客户端

1. 接收用户的socket和option，将option信息发送给服务端协商好编解码方式。  
   `func NewClient(conn net.Conn, opt *server.Option) (*Client, error)`
2. 新建1个客户端实例，然后开启一个goroutine接受响应。  
   `func newClientCodec(cc codec.Codec, opt *server.Option) *Client`

### (4) 接收响应：receive方法

1. 先解析head，然后根据解析出的seq编号将请求从pending中移除，并返回存请求信息的call。
2. 将响应结果解析到call中，然后将call通过管道发送给调用者。

以上过程完成了解析head、将该请求实例(call)从pending中移除、将结果解析出来存在call中，再将这个call发给调用方。
`func (c *Client) receive()`

### (5) 发送请求的接口

1. Go 和 Call 是客户端暴露给用户的两个 RPC 服务调用接口，Go 是一个异步接口，返回 call 实例。发送出去就返回，不需要等待响应。
   `func (c *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call`
2. Call 是对 Go 的封装，阻塞 call.Done，等待响应返回，是一个同步接口。
   `func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error`
3. Go接口中创建1个call实例，然后将这个实例发送通过调用 send 方法发送出去。
4. Go接口在发送过程中是阻塞的，需要等待发送成功后才能执行后序。Call接口的阻塞是阻塞到了收到这个请求的响应。

### (6) 发送请求：send方法

1. 发送过程保证每个请求有序发送，不发生混淆，所以需要并发安全。发送的并发控制都是使用名为sending的锁，其他操作使用mu锁。
2. 将call实例添加到Client中的pending中(注册call实例)
3. 构造header和args。然后他们编码发送给服务端。

`func (c *Client) send(call *Call)`

### (7) 与Call相关

#### (a) 注册call实例，将Call添加到pending

1. 应用在send过程
2. 将call实例添加到pending中
3. 更新请求序列号seq

`func (c *Client) registerCall(call *Call) (uint64, error)`

#### (b) 将Call移除到pending，并返回结果

* 接收到响应，请求发送失败，清空请求列表，只要需要将call从pending中移除的场合都需要。
`func (c *Client) removeCall(seq uint64) *Call`

#### (c) 移除所有calls，清空pending

* 移除所有的calls
* 并且通知调用者
* shutdown状态改为true
`func (c *Client) terminateCalls(err error)`

# 服务注册

通过反射，获取某个结构体的所有方法；并且通过方法，获取到该方法所有的参数类型与返回值。

## 1. 方法的所有信息：methodType

```go
type methodType struct {
	method    reflect.Method // 方法名
	ArgType   reflect.Type   // 第一个参数类型
	ReplyType reflect.Type   // 第二个参数类型，也是反馈结果的类型
	numCalls  uint64
}
```

## 2. 服务的信息：service

```go
type service struct {
	name   string                 // 服务的结构体名
	typ    reflect.Type           // 服务的结构体类型
	rcv    reflect.Value          // 服务的结构体实例本身，实例值
	method map[string]*methodType // 这个服务的所有方法。
}
```

method存的就是上面methodType的方法名反射后的字符串。

### (1) 初始化

`func newService(rcv interface{}) *service`  
在初始化的过程中，将传入的rcv进行反射，rcv存了需要注册的方法，初始化过程中将格式正确的方法都反射后转存到service中。

### (2) 过滤不符合格式的方法

`func (s *service) registerMethods()`  
过滤条件就是入参2个，范围值1个(类型为 error)

### (3) call方法，通过反射值调用方法

`func (s *service) call(m *methodType, argv, replyVal reflect.Value) error`  
call相当于所有调用方法的模板：m为方法反射前的实例；argv、replyVal为两个参数。通过名称可以发现，一般第一个为入参，第二个为返回值，因为整个call的返回值为error，所以只能通过入参将地址传进去，然后将结果存在这个地址中。

## 3. 将服务集成到服务端

已经实现了如何将方法映射成服务。收到请求到回复，相关的步骤有：

1. 根据入参类型，解码请求的 body ；
2. 调用 service.call，完成方法调用；
3. 将 reply 序列化为字节流，构造响应报文

### (1) 注册服务端

`func (server *Server) Register(rcvr interface{}) error`  

* 需要创建一个并发安全的map实例(全局的服务端实例serviceMap)，保存服务器有的各个服务，key：服务名；value：服务的实例.
* 传入的rcvr就是1个服务的实例，将其进行反射，存到服务端的map实例中。

### (2) 服务发现

`func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error)`

1. 已知收到的请求名称是 service.method 的字符串格式。可以通过字符串解析出服务名service和方法名method。
2. 根据服务名service在全局的服务端实例serviceMap中查找对应的服务。
3. 根据方法名在服务中找需要调用的方法。

以上过程就实现了服务发现。

### (3) readRequest方法中补全请求中的内容

```go
type request struct {
	h              *codec.Header // 请求的head
	argv, replyVal reflect.Value
	mtype          *methodType
	svc            *service
}
```
收到请求后，将请求的内容存到request实例中，比如方法名、方法的参数类型等，可以通过服务发现后找到的方法填进去。

### (4) handleRequest实现对请求的处理

1. 通过readRequest已知了request的所有内容，包括方法method、参数等。
2. 调用存在request中的服务实例中的call方法，就可以实现对请求的业务处理，得到结果，这个过程相当于普通的函数调用一样。


学习<https://geektutu.com/post/geerpc.html>，做的一些总结。
