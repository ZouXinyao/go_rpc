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
# 一、编解码

二、服务端

三、客户端

四、服务注册

学习<https://geektutu.com/post/geerpc.html>，做的一些总结。
