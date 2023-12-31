package ws

import (
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"shyIM/config"
	"shyIM/model/cache"
	"shyIM/pkg/logger"
	"shyIM/pkg/protocol/pb"
	"sync"
	"time"
)

// Conn 连接实例
// 1. 启动读写线程
// 2. 读线程读到数据后，根据数据类型获取处理函数，交给 worker 队列调度执行
type Conn struct {
	ConnId           uint64          // 连接编号，通过对编号取余，能够让 Conn 始终进入同一个 worker，保持有序性
	server           *Server         // 当前连接属于哪个 server
	UserId           uint64          // 连接所属用户id
	UserIdMutex      sync.RWMutex    // 保护 userId 的锁
	Socket           *websocket.Conn // 用户连接
	sendCh           chan []byte     // 用户要发送的数据
	isClose          bool            // 连接状态
	isCloseMutex     sync.RWMutex    // 保护 isClose 的锁
	exitCh           chan struct{}   // 通知 writer 退出
	maxClientId      uint64          // 该连接收到的最大 clientId，确保消息的可靠性
	maxClientIdMutex sync.Mutex      // 保护 maxClientId 的锁

	lastHeartBeatTime time.Time  // 最后活跃时间
	heartMutex        sync.Mutex // 保护最后活跃时间的锁
}

func NewConnection(server *Server, wsConn *websocket.Conn, ConnId uint64) *Conn {
	return &Conn{
		ConnId:            ConnId,
		server:            server,
		UserId:            0, // 此时用户未登录， userID 为 0
		Socket:            wsConn,
		sendCh:            make(chan []byte, 10),
		isClose:           false,
		exitCh:            make(chan struct{}, 1),
		lastHeartBeatTime: time.Now(), // 刚连接时初始化，避免正好遇到清理执行，如果连接没有后续操作，将会在下次被心跳检测踢出
		maxClientId:       0,
	}
}

func (c *Conn) Start() {
	// 开启从客户端读取数据流程的 goroutine
	go c.StartReader()

	// 开启用于写回客户端数据流程的 goroutine
	//go c.StartWriter()
	go c.StartWriterWithBuffer()
}

// StartReader 用于从客户端中读取数据
func (c *Conn) StartReader() {
	defer c.Stop()

	for {
		// 阻塞读
		_, data, err := c.Socket.ReadMessage()
		if err != nil {
			logger.Slog.Error("[websocket ReadMessage Failed]", "err", err)
			return
		}
		// 消息处理
		c.HandlerMessage(data)
	}
}

// HandlerMessage 消息处理
func (c *Conn) HandlerMessage(bytes []byte) {
	// TODO 所有错误都需要写回给客户端

	input := new(pb.Input)
	if err := proto.Unmarshal(bytes, input); err != nil {
		logger.Slog.Error(err.Error())
		return
	}
	// 对未登录用户进行拦截
	if input.Type != pb.CmdType_CT_Login && c.GetUserId() == 0 {
		return
	}

	// 构造请求体(包含消息体、连接对象、处理函数)
	req := &Req{
		conn: c,
		data: input.Data,
		f:    nil,
	}

	// 针对不同的消息类型, 选择不同的逻辑函数
	switch input.Type {
	case pb.CmdType_CT_Login: // 登录
		req.f = req.Login
	case pb.CmdType_CT_Heartbeat: // 心跳
		req.f = req.Heartbeat
	case pb.CmdType_CT_Message: // 上行消息
		req.f = req.MessageHandler
	case pb.CmdType_CT_ACK: // ACK TODO

	case pb.CmdType_CT_Sync: // 离线消息同步
		req.f = req.Sync
	default:
		fmt.Println("[unknown cmdType]")
	}

	if req.f == nil {
		return
	}

	// 更新心跳时间
	c.KeepLive()

	// 送入worker队列等待调度执行
	c.server.SendMsgToTaskQueue(req)
}

// SendMsg 根据 userId 给相应 socket 发送消息
func (c *Conn) SendMsg(userId uint64, bytes []byte) {
	c.isCloseMutex.RLock()
	defer c.isCloseMutex.RUnlock()

	// 已关闭
	if c.isClose {
		fmt.Println(fmt.Sprintf("userID: %d 已下线 不能发送消息", c.UserId))
		return
	}

	// 根据 userId 找到对应 socket
	conn := c.server.GetConn(userId)
	if conn == nil {
		return
	}

	// 每个客户端都对应一个conn conn.sendCh 用来监听是否有消息发送过来
	conn.sendCh <- bytes

	return
}

// StartWriter 向客户端写数据
func (c *Conn) StartWriter() {
	logger.Slog.Info("[WebSocket Writer Goroutine is running]")
	defer logger.Slog.Info(fmt.Sprintf(c.RemoteAddr() + "[WebSocket Writer exit!]"))

	var err error
	for {
		select {
		case data := <-c.sendCh:
			if err = c.Socket.WriteMessage(websocket.BinaryMessage, data); err != nil {
				logger.Slog.Info("[WebSocket WriteMessage Failed]", "[ERROR]", err)
				return
			}
			// 更新心跳时间
			c.KeepLive()
		case <-c.exitCh:
			return
		}
	}
}

// StartWriterWithBuffer 向客户端写数据
// 由延迟优先调整为吞吐优先，使得消息的整体吞吐提升，但是单条消息的延迟会有所上升
func (c *Conn) StartWriterWithBuffer() {
	logger.Slog.Info("[WebSocket WriterWithBuffer Goroutine is running]")
	defer logger.Slog.Info(fmt.Sprintf(c.RemoteAddr() + "[WebSocket WriterWithBuffer exit!]"))

	// 每 100ms 或者当 buffer 中存够 50 条数据时，进行发送
	tickerInterval := 100
	ticker := time.NewTicker(time.Millisecond * time.Duration(tickerInterval))
	bufferLimit := 50
	buffer := &pb.OutputBatch{Outputs: make([][]byte, 0, bufferLimit)}

	send := func() {
		if len(buffer.Outputs) == 0 {
			return
		}

		sendData, err := proto.Marshal(buffer)
		if err != nil {
			logger.Slog.Error("proto marshal: ", "[ERROR]", err)
			return
		}
		if err = c.Socket.WriteMessage(websocket.BinaryMessage, sendData); err != nil {
			fmt.Println("Send Data error:, ", err, " Conn Writer exit")
			return
		}
		buffer.Outputs = make([][]byte, 0, bufferLimit)
		// 更新心跳时间
		c.KeepLive()
	}

	for {
		select {
		case buff := <-c.sendCh:
			buffer.Outputs = append(buffer.Outputs, buff)
			if len(buffer.Outputs) == bufferLimit {
				send()
			}
		case <-ticker.C:
			send()
		case <-c.exitCh:
			return
		}
	}
}

func (c *Conn) Stop() {
	c.isCloseMutex.Lock()
	defer c.isCloseMutex.Unlock()

	if c.isClose {
		return
	}

	// 关闭 socket 连接
	_ = c.Socket.Close()
	// 关闭 writer
	c.exitCh <- struct{}{}

	if c.GetUserId() != 0 {
		// 将连接从connMap中移除
		c.server.RemoveConn(c.GetUserId())
		// 用户下线
		_ = cache.DelUserOnline(c.GetUserId())
	}

	c.isClose = true

	// 关闭管道
	close(c.exitCh)
	close(c.sendCh)
	logger.Slog.Info("Conn is closed", "UserId", c.GetUserId())
}

// KeepLive 更新心跳
func (c *Conn) KeepLive() {
	c.heartMutex.Lock()
	defer c.heartMutex.Unlock()
	c.lastHeartBeatTime = time.Now()
}

// IsAlive 是否存活
func (c *Conn) IsAlive() bool {
	c.heartMutex.Lock()
	c.isCloseMutex.RLock()
	defer c.isCloseMutex.RUnlock()
	defer c.heartMutex.Unlock()

	if c.isClose || time.Now().Sub(c.lastHeartBeatTime) > time.Duration(config.GlobalConfig.APP.HeartbeatTimeout)*time.Second {
		return false
	}
	return true
}

// GetUserId 获取 userId
func (c *Conn) GetUserId() uint64 {
	c.UserIdMutex.RLock()
	defer c.UserIdMutex.RUnlock()

	return c.UserId
}

// SetUserId 设置 UserId
func (c *Conn) SetUserId(userId uint64) {
	c.UserIdMutex.Lock()
	defer c.UserIdMutex.Unlock()

	c.UserId = userId
}

func (c *Conn) CompareAndIncrClientID(newMaxClientId uint64) bool {
	c.maxClientIdMutex.Lock()
	defer c.maxClientIdMutex.Unlock()
	if c.maxClientId+1 == newMaxClientId {
		c.maxClientId++
		return true
	}
	return false
}

// RemoteAddr 获取远程客户端地址
func (c *Conn) RemoteAddr() string {
	return c.Socket.RemoteAddr().String()
}
