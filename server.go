package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int
	//在线用户列表
	OnlineMap map[string]*User
	mapLock   sync.RWMutex
	//消息广播的channel
	Message chan string
}

// 创建一个server的接口
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}
	return server
}

// 启动服务器的接口
func (this *Server) Start() {
	//socket listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
	if err != nil {
		fmt.Println("net.listen err:", err)
		return
	}
	//close listen socket
	defer listener.Close()
	//监听Message的goroutine
	go this.ListenMessage()

	for {
		//accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener.Accept err:", err)
			continue
		}
		//do handler
		go this.Handler(conn)
	}
}

// 根据连接conn new一个user，user中有conn，channel c等待消息
// 然后广播到server的channel Message
func (this *Server) Handler(conn net.Conn) {
	//当前连接的业务
	// fmt.Println("链接建立成功")
	user := NewUser(conn, this)
	user.Online() //用户上线

	//监听用户是否活跃
	isLive := make(chan bool)

	go func() {
		buf := make([]byte, 4096)
		for {
			//n是buf的长度，读到0则代表已下线
			n, err := conn.Read(buf)
			if n == 0 {
				user.Offline()
			}
			if err != nil {
				fmt.Println("conn read err:", err)
				return
			}
			//提取用户的消息(去除'\n')
			msg := string(buf[:n-1])
			//将得到的消息广播处理
			user.DoMessage(msg)

			//用户任意消息，代表当前用户是一个活跃的
			isLive <- true
		}
	}()
	//当前handler阻塞
	for {
		select {
		case <-isLive:
			//当前用户是活跃的，重置定时器
			//不做任何事情，为了激活下面定时器，更新下面定时器
		case <-time.After(time.Second * 300):
			//已经超时，将当前user强制关闭
			user.SendMsg("连接超时，您被踢出")

			//销毁用的资源
			//close(user.C)
			//关闭连接
			conn.Close()

			//退出当前handler
			return //runtim.Goexit()
		}
	}

}

// 广播消息写道server的channel中
func (this *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	this.Message <- sendMsg
}

// 监听Message广播消息的goroutine，一旦有消息就发送给全部在线User
// 相当于将server管道的东西写到每个user的管道中
func (this *Server) ListenMessage() {
	for {
		msg := <-this.Message
		//将message
		this.mapLock.Lock()
		for _, cli := range this.OnlineMap {
			cli.C <- msg
		}
		this.mapLock.Unlock()
	}
}
