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

二、服务端

三、客户端

四、服务注册

学习<https://geektutu.com/post/geerpc.html>，做的一些总结。
