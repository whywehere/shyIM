# :heart: ShyIM 

使用Golang实现简易的IM聊天服务

## 1. 初始化

### 解析配置文件

使用`github.com/spf13/viper`库解析`.yaml`配置文件

```go
var GlobalConfig *Config
// Config 配置映射结构体
type Config struct {
	MySQL struct {
		DNS string `mapstructure:"dns"` 
	} `mapstructure:"mysql"`

	Redis struct {
		Address  string `mapstructure:"address"`  
		Password string `mapstructure:"password"` 
	} `mapstructure:"redis"`

	Etcd struct {
		Endpoints []string      `mapstructure:"endpoints"` 
		Timeout   time.Duration `mapstructure:"timeout"`
	} `mapstructure:"etcd"`

	APP struct {
		IP                  string `mapstructure:"ip"` //localmachine ip
		HttpServerPort      string `mapstructure:"http_server_port"`
		RPCPort             string `mapstructure:"rpc_port"`
		Salt                string `mapstructure:"salt"`// 密码加盐
		WorkerPoolSize      int    `mapstructure:"worker_pool_size"` // worker 队列数量
		MaxWorkerTask       int    `mapstructure:"max_worker_task"` // worker 对应负责的任务队列最大任务存储数量
		HeartbeatTimeout    int    `mapstructure:"heartbeat_time"`
		HeartbeatInterval   int    `mapstructure:"heartbeat_interval"`
		WebSocketServerPort string `mapstructure:"websocket_server_port"`
	} `mapstructure:"app"`
	JWT struct {
		SignKey    string        `mapstructure:"sign_key"`
		ExpireTime time.Duration `mapstructure:"expire_time"`
	} `mapstructure:"jwt"`
	RabbitMQ struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"rabbitmq"`
}
```

### 连接mysql、redis

使用`gorm.io/gorm`、`gorm.io/driver/mysql`连接mysql 、`github.com/go-redis/redis/v8`连接redis

```go
var (
	DB  *gorm.DB
	RDB *redis.Client
)
// mysql
DB, err = gorm.Open(mysql.Open(config.GlobalConfig.MySQL.DNS), &gorm.Config{})

// redis
RDB = redis.NewClient(&redis.Options{
		Addr:         config.GlobalConfig.Redis.Address,
		DB:           0,
		Password:     config.GlobalConfig.Redis.Password,
		PoolSize:     30,
		MinIdleConns: 30,
	})
```

### Model

该项目共有五张表**user**,  **friend**,  **group** , **message**,  **group_user **

```go
// User
type User struct {
	ID          uint64    `gorm:"primary_key;auto_increment;comment:'自增主键'" json:"id"`
	PhoneNumber string    `gorm:"not null;unique;comment:'手机号'" json:"phone_number"`
	Nickname    string    `gorm:"not null;comment:'昵称'" json:"nickname"`
	Password    string    `gorm:"not null;comment:'密码'" json:"-"`
	CreateTime  time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;comment:'创建时间'" json:"create_time"`
	UpdateTime  time.Time `gorm:"not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'" json:"update_time"`
}

// Friend
type Friend struct {
	ID         uint64    `gorm:"primary_key;auto_increment;comment:'自增主键'" json:"id"`
	UserID     uint64    `gorm:"not null;comment:'用户id'" json:"user_id"`
	FriendID   uint64    `gorm:"not null;comment:'好友id'" json:"friend_id"`
	CreateTime time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;comment:'创建时间'" json:"create_time"`
	UpdateTime time.Time `gorm:"not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'" json:"update_time"`
}

// Group
type Group struct {
	ID         uint64    `gorm:"primary_key;auto_increment;comment:'自增主键'" json:"id"`
	Name       string    `gorm:"not null;comment:'群组名称'" json:"name"`
	OwnerID    uint64    `gorm:"not null;comment:'群主id'" json:"owner_id"`
	CreateTime time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;comment:'创建时间'" json:"create_time"`
	UpdateTime time.Time `gorm:"not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'" json:"update_time"`
}

// Message
type Message struct {
	ID          uint64    `gorm:"primary_key;auto_increment;comment:'自增主键'" json:"id"`
	UserID      uint64    `gorm:"not null;comment:'用户id，指接受者用户id'" json:"user_id"`
	SenderID    uint64    `gorm:"not null;comment:'发送者用户id'"`
	SessionType int8      `gorm:"not null;comment:'聊天类型，群聊/单聊'" json:"session_type"`
	ReceiverId  uint64    `gorm:"not null;comment:'接收者id，群聊id/用户id'" json:"receiver_id"`
	MessageType int8      `gorm:"not null;comment:'消息类型,语言、文字、图片'" json:"message_type"`
	Content     []byte    `gorm:"not null;comment:'消息内容'" json:"content"`
	Seq         uint64    `gorm:"not null;comment:'消息序列号'" json:"seq"`
	SendTime    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;comment:'消息发送时间'" json:"send_time"`
	CreateTime  time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;comment:'创建时间'" json:"create_time"`
	UpdateTime  time.Time `gorm:"not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'" json:"update_time"`
}

// Group_User
type GroupUser struct {
	ID         uint64    `gorm:"primary_key;auto_increment;comment:'自增主键'" json:"id"`
	GroupID    uint64    `gorm:"not null;comment:'组id'" json:"group_id"`
	UserID     uint64    `gorm:"not null;comment:'用户id'" json:"user_id"`
	CreateTime time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;comment:'创建时间'" json:"create_time"`
	UpdateTime time.Time `gorm:"not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'" json:"update_time"`
}
```

### proto文件

```protobuf
								// ------conn.proto--------//
service Connect {
  // 私聊消息投递
  rpc DeliverMessage (DeliverMessageReq) returns (google.protobuf.Empty);
  // 群聊消息投递
  rpc DeliverMessageAll(DeliverMessageAllReq) returns (google.protobuf.Empty);
}

message DeliverMessageReq {
  uint64 receiver_id = 1;   //  消息接收者
  bytes data = 2;  // 要投递的消息
}

message DeliverMessageAllReq{
  map<uint64, bytes> receiver_id_2_data = 1; // 消息接受者到要投递的消息的映射
}

							// ------message.proto--------//
// 会话类型
enum SessionType {
  ST_UnKnow = 0;  // 未知
  ST_Single = 1; // 单聊
  ST_Group = 2;  // 群聊
}

// 枚举发送的消息类型
enum MessageType {
  MT_UnKnow = 0;  // 未知
  MT_Text = 1;  // 文本类型消息
  MT_Picture = 2;  // 图片类型消息
  MT_Voice = 3;  // 语音类型消息
}

// ACK 消息类型，先根据 Input/Output 的 type 解析出是 ACK，再根据 ACKType 判断是 ACK 的是什么消息
enum ACKType {
  AT_UnKnow = 0;  // 未知
  AT_Up = 1 ;   // 服务端回复客户端发来的消息
  AT_Push = 2;  // 客户端回复服务端发来的消息
  AT_Login = 3;  // 登录
}

// 所有 websocket 的消息类型
enum CmdType {
  CT_UnKnow = 0;  // 未知
  CT_Login = 1;   // 连接注册，客户端向服务端发送，建立连接
  CT_Heartbeat = 2;  // 心跳，客户端向服务端发送，连接保活
  CT_Message = 3;  // 消息投递，可能是服务端发给客户端，也可能是客户端发给服务端
  CT_ACK = 4;   // ACK
  CT_Sync = 5;   // 离线消息同步
}

/*
	客户端发送给服务端 [消息]
	客户端发送前：先组装出下层消息[data] 例如 HeartBeatMsg，序列化作为 Input 的 data 值，再填写 type 值，序列化 Input 发送给服务端
	服务端收到后：反序列化成 Input，根据 type 值调用不同类型 handler，在 handler 中将 data 解析成其他例如 LoginMsg类型消息，再做处理
*/
message Input {
  CmdType type = 1;   // 消息类型，根据不同消息类型，可以将 data 解析成下面其他类型
  bytes data = 2;    // 数据
}

/*
	服务端发送[消息]给客户端
	服务端：组装出下层消息例如 Message，序列化作为 Output 的 data 值，再填写其他值，序列化 Output 发送给客户端
	客户端：反序列化成 Output，根据 type 值调用不同类型 handler，在 handler 中将 data 解析成其他例如 Message 类型消息，再做处理
 */
message Output {
  CmdType type = 1;  // 消息类型，根据不同的消息类型，可以将 data 解析成下面其他类型
  int32 code = 2;  // 错误码
  string CodeMsg = 3;  // 错误码信息
  bytes data = 4;  // 数据
}

// 下行消息批处理
message OutputBatch {
  repeated bytes outputs = 1;
}

// 登录
message LoginMsg {
  bytes token = 1;    // token
}

// 心跳
message HeartbeatMsg {}

// 上行消息
message UpMsg {
  Message msg = 1; // 消息内容
  uint64 clientId = 2;  // 保证上行消息可靠性
}

// 下行消息
message PushMsg {
  Message msg = 1;  // 消息内容
}

// 上行离线消息同步
message SyncInputMsg {
  uint64 seq = 1;  // 客户端已经同步的序列号
}

// 下行离线消息同步
message SyncOutputMsg {
  repeated Message messages = 1;  // 消息列表
  bool has_more = 2;   // 是否还有更多数据
}

// 消息投递
// 上行、下行
message Message {
  SessionType session_type = 1;  // 会话类型 单聊、群聊
  uint64 receiver_id = 2;  // 接收者id 用户id/群组id
  uint64 sender_id = 3;  // 发送者id
  MessageType message_type = 4;  // 消息类型 文本、图片、语音
  bytes content = 5;  // 实际用户所发数据
  uint64 seq = 6;   // 客户端的最大消息同步序号
  int64 send_time = 7; // 消息发送时间戳，ms
}

// ACK 回复
// 根据顶层消息 type 解析得到
// 客户端中发送场景：
// 1. 客户端中收到 PushMsg 类型消息，向服务端回复 AT_Push 类型的 ACK，表明已收到 TODO
// 服务端中发送场景：
// 1. 服务端收到 CT_Login 消息，向客户端回复 AT_Login 类型的 ACK
// 2. 服务端收到 UpMsg 类型消息， 向客户端回复 AT_Up 类型的 ACK 和 clientId，表明已收到该 ACK，无需超时重试
message ACKMsg {
  ACKType type = 1;   // 收到的是什么类型的 ACK
  uint64 clientId = 2;
  uint64 seq = 3;   // 上行消息推送时回复最新 seq
}

								// ------mq_msg.proto--------//
message MQMessages {
  repeated MQMessage messages = 1;
}

message MQMessage {
  uint64 id = 1;
  uint64 user_id = 2;
  uint64 sender_id = 3;
  int32 session_type = 4;
  uint64 receiver_id = 5;
  int32 message_type = 6;
  bytes content = 7;
  uint64 seq = 8;
  google.protobuf.Timestamp send_time = 9;
  google.protobuf.Timestamp create_time = 10;
  google.protobuf.Timestamp update_time = 11;
}					
```



## 2. 启动Kafka

`github.com/IBM/sarama`

### 初始化Kafka

```go
const (
	MessageGroupID = "message.group"
	MessageTopic   = "message.topic"
)

type Conn struct {
	producer sarama.SyncProducer
	consumer sarama.ConsumerGroup
	topic    string
	groupID  string
}

func InitKafka(brokers []string) *Conn {
	config := sarama.NewConfig()
	config.Producer.Return.Errors = true    // 设定是否需要返回错误信息
	config.Producer.Return.Successes = true // 设定是否需要返回成功信息
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		logger.Slog.Error("Failed to create Kafka producer", "err", err)
		return nil
	}

	consumer, err := sarama.NewConsumerGroup(brokers, MessageGroupID, config)
	if err != nil {
		logger.Slog.Error("Failed to create Kafka consumerGroup", "err", err)
		return nil
	}

	return &Conn{
		producer: producer,
		consumer: consumer,
		topic:    MessageTopic,
		groupID:  MessageGroupID,
	}
}
```

### 生产者生产消息

```go
func (c *Conn) Publish(data []byte) error {
	if data == nil || len(data) == 0 {
		fmt.Println("Failed to publish: data is empty")
		return nil
	}
	msg := &sarama.ProducerMessage{
		Topic: c.topic,
		Value: sarama.ByteEncoder(data),
	}
	_, _, err := c.producer.SendMessage(msg)
	return err
}
```

### 消费者消费消息

```go
// ConsumeMessage Conn 开一个go进程一直运行
func (c *Conn) ConsumeMessage() {
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := c.consumer.Consume(context.Background(), []string{c.topic}, ConsumerGroupHandlerKafka{}); err != nil {
				logger.Slog.Error("consumer consume failed", "err", err)
				return
			}
		}
	}()
}
// ConsumerGroupHandlerKafka 实现 ConsumerGroupHandler 接口来处理 Kafka 消息
type ConsumerGroupHandlerKafka struct{}

func (h ConsumerGroupHandlerKafka) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h ConsumerGroupHandlerKafka) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h ConsumerGroupHandlerKafka) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		// 处理从 Kafka 接收到的消息
		MessageCreateHandler(msg)

		// 标记消息已经被处理
		session.MarkMessage(msg, "")
	}
	return nil
}

// MessageCreateHandler 用于处理从 Kafka 中接收到的消息
func MessageCreateHandler(msg *sarama.ConsumerMessage) {
	messageModels := model.ProtoMarshalToMessage(msg.Value) // 使用消息的值
	if messageModels == nil {
		// 处理消息为空的情况，这里可以根据实际需求返回不同的处理结果
		return
	}

	err := model.CreateMessage(messageModels...)
	if err != nil {
		fmt.Println("[MessageCreateHandlerKafka] model.CreateMessage 失败，err:", err)
		// 根据业务逻辑返回不同的处理结果，可能是 Nack 或其他操作
		return
	}

	// 根据成功处理后的情况返回 Ack
	return
}
```

## 3. 启动ETCD

`go.etcd.io/etcd/client/v3`

### 服务注册

```go
// Registry  服务注册
type Registry struct {
	client        *clientV3.Client                        // etcd client
	leaseID       clientV3.LeaseID                        //租约ID
	keepAliveChan <-chan *clientV3.LeaseKeepAliveResponse // 租约 KeepAlive 相应chan
	key           string                                  // key
	val           string
}

func NewRegistry(key, value string, lease int64) error {
	client, err := clientV3.New(clientV3.Config{
		Endpoints:   config.GlobalConfig.Etcd.Endpoints,
		DialTimeout: time.Duration(config.GlobalConfig.Etcd.Timeout) * time.Second,
	})
	if err != nil {
		return err
	}
	regServer := &Registry{
		client: client,
		key:    key,
		val:    value,
	}
	if err = regServer.putKeyWithLease(lease); err != nil {
		return err
	}

	go regServer.ListenLeaseRespChan()

	return nil
}

// ListenLeaseRespChan 监听续租情况
func (r *Registry) ListenLeaseRespChan() {
	defer r.close()

	for range r.keepAliveChan {
	}

}
```

### 服务发现

```go
// Discovery 服务发现
type Discovery struct {
	client    *clientV3.Client //    etcd client
	serverMap sync.Map
}

func NewDiscovery() (*Discovery, error) {
	client, err := clientV3.New(clientV3.Config{
		Endpoints:   config.GlobalConfig.Etcd.Endpoints,
		DialTimeout: time.Duration(config.GlobalConfig.Etcd.Timeout) * time.Second,
	})
	if err != nil {
		logger.Slog.Error("Failed to create etcd client", "[ERROR]", err)
		return nil, err
	}
	logger.Slog.Info("Creating Discovery succeeded")
	return &Discovery{client: client}, nil
}
```

### 监听更新

```go
// 获得以prefix为前缀的key的所有value， 将所有value(service)加入serverMap
func (d *Discovery) WatchServices(prefix string) {
	resp, err := d.client.Get(context.TODO(), prefix, clientV3.WithPrefix())
	if err != nil {
		return
	}
	for i := range resp.Kvs {
		if v := resp.Kvs[i]; v != nil {
			d.serverMap.Store(string(v.Key), string(v.Value))
		}
	}
	d.watcher(prefix)
	return
}

//监听以prefix为前缀的所有key的变化, 并对变化做出相应的处理，增加服务或移除服务
func (d *Discovery) watcher(prefix string) {
	watchChan := d.client.Watch(context.TODO(), prefix, clientV3.WithPrefix())
	for wResp := range watchChan {
		for _, event := range wResp.Events {
			switch event.Type {
			case mvccpb.PUT:
				d.serverMap.Store(string(event.Kv.Key), string(event.Kv.Value))
			case mvccpb.DELETE:
				d.serverMap.Delete(string(event.Kv.Key))
			}

		}
	}
}
```

## 4. HttpService

> 使用`github.com/gin-gonic/gin` 
>
> http的实现逻辑较简单 故不分析

```go
// 用户注册
r.POST("/register", service.RegisterHandler)
// 用户登录
r.POST("/login", service.LoginHandler)
// 添加 AuthCheck() 验证中间件
auth := r.Group("", middlewares.AuthCheck())
{
	// 添加好友
	auth.POST("/friend/add", service.AddFriendHandler)
	// 创建群聊
	auth.POST("/group/create", service.CreateGroupHandler)
	// 获取群成员列表
	auth.GET("/group_user/list", service.GroupUserList)
}
```

## 5. WebsocketService

>  使用`github.com/gorilla/websocket`

## 升级协议  http -> websocket

```go
var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

r.GET("/ws", func(c *gin.Context) {
		// 升级协议  http -> websocket
		WsConn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Slog.Error("[WebSocket Connect Failed]", "[ERROR]", err)
			return
		}

		// 初始化连接
		conn := ws.NewConnection(server, WsConn, connID)
		connID++

		// 开启读写线程
		go conn.Start()
	})
```

## SeverManager

```go
// Server 连接管理
// 1. 连接管理
// 将所有的客户端连接对象全部存入connMap中， 方便后续操作
// 2. 工作队列
// 通过taskQueue 提高消息处理的速度
type Server struct {
	connMap   sync.Map    // 登录的用户连接 k: userid, v: 连接对象
	taskQueue []chan *Req // 工作池
}

// GetServer 返回一个ServerManager单例
func GetServer() *Server {
	once.Do(func() {
		ConnManager = &Server{
            // 初始worker队列
			taskQueue: make([]chan *Req, config.GlobalConfig.APP.WorkerPoolSize), 
		}
	})
	return ConnManager
}
```

### ConnMap Operations

```go
// ServerMap Operations:

// AddConn 添加连接
func (cm *Server) AddConn(userId uint64, conn *Conn) {
	cm.connMap.Store(userId, conn)
	fmt.Printf("UserId=%d 已上线\n", userId)
	logger.Slog.Info(fmt.Sprintf("userId=%d has added to ServerMap", userId))
}

// RemoveConn 删除连接
func (cm *Server) RemoveConn(userId uint64) {
	cm.connMap.Delete(userId)
	fmt.Printf("UserId=%d 已下线\n", userId)
	logger.Slog.Info(fmt.Sprintf("userId=%d has removeed from ServerMap", userId))
}

// GetConn 根据userid获取相应的连接
func (cm *Server) GetConn(userId uint64) *Conn {
	value, ok := cm.connMap.Load(userId)
	if ok {
		return value.(*Conn)
	}
	return nil
}

// GetAllConn 获取全部连接
func (cm *Server) GetAllConn() []*Conn {
	connects := make([]*Conn, 0)
	cm.connMap.Range(func(key, value interface{}) bool {
		conn := value.(*Conn)
		connects = append(connects, conn)
		return true
	})
	return connects
}

// SendMessageAll 进行本地推送
func (cm *Server) SendMessageAll(userId2Msg map[uint64][]byte) {
	var wg sync.WaitGroup
	ch := make(chan struct{}, 5) // 限制并发数
	for userId, data := range userId2Msg {
		ch <- struct{}{}
		wg.Add(1)
		go func(userId uint64, data []byte) {
			defer func() {
				<-ch
				wg.Done()
			}()
			conn := ConnManager.GetConn(userId)
			if conn != nil {
				conn.SendMsg(userId, data)
			}
		}(userId, data)
	}
	close(ch)
	wg.Wait()
}
```

### Websocket Conn

```go
// Conn 连接实例
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

func (c *Conn) Start() {
	// 开启从客户端读取数据流程的 goroutine
	go c.StartReader()

	// 开启用于写回客户端数据流程的 goroutine
	go c.StartWriter()
	go c.StartWriterWithBuffer()
}
```

### 开启从客户端读取数据流程的 goroutine

> 数据格式[客户端 --> 服务端]

```go
type Input struct {
	// ...
    
    // 消息类型，根据不同消息类型，可以将 data 解析成下面其他类型
	Type CmdType `protobuf:"varint,1,opt,name=type,proto3,enum=pb.CmdType" json:"type,omitempty"` 
    
    // 对应具体消息类型的结构体数据
	Data []byte  `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`                  
}
```

服务端接收客户端消息

```go
// StartReader 用于从客户端中读取数据
func (c *Conn) StartReader() {
	defer c.Stop()
	for {
		// 阻塞读
		_, data, err := c.Socket.ReadMessage()
		if err != nil {
			logger.Slog.Error("websocket ReadMessage failed")
			return
		}
		// 消息处理
		c.HandlerMessage(data)
	}
}
```

服务端处理消息

```go
/*
    // 枚举消息类型
    enum CmdType {
      CT_UnKnow = 0;  // 未知
      CT_Login = 1;   // 连接注册，客户端向服务端发送，建立连接
      CT_Heartbeat = 2;  // 心跳，客户端向服务端发送，连接保活
      CT_Message = 3;  // 消息投递，可能是服务端发给客户端，也可能是客户端发给服务端
      CT_ACK = 4;   // ACK
      CT_Sync = 5;   // 离线消息同步
    }
    // ==== Req ==== //
	type Req struct {
		conn *Conn   // 连接
		data []byte  // 客户端发送的请求数据
		f    Handler // 该请求需要执行的路由函数
	}
*/

// HandlerMessage 消息处理
func (c *Conn) HandlerMessage(bytes []byte) {
	// 将bytes解析绑定到input
    input := new(pb.Input)
	if err := proto.Unmarshal(bytes, input); err != nil {
		return
	}
    
	// 对未登录用户进行拦截
	if input.Type != pb.CmdType_CT_Login && c.GetUserId() == 0 {
		return
	}
	
	req := &Req{
		conn: c,
		data: input.Data,
		f:    nil,
	}
    
	//客户端消息类型，服务端根据不同的**cmdType**进行不同的处理
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
			logger.Slog.Info("Unknown command type")
	}

	// 更新心跳时间
	c.KeepLive()
	// 根据不同的input.Type， 得到不同的处理函数， 生成req实体对象，将req存入taskQueue中， 依次执行 workers同时监听taskQueue, 并发执行逻辑函数
	// 送入worker队列等待调度执行 
	c.server.SendMsgToTaskQueue(req)
}

								//======= taskQueue operations ======//
// SendMsgToTaskQueue 将消息交给 taskQueue，由 worker 调度处理
func (cm *Server) SendMsgToTaskQueue(req *Req) {
	// 根据ConnID来分配当前的连接应该由哪个worker负责处理，保证同一个连接的消息处理串行
	// 轮询的平均分配法则 得到需要处理此条连接的workerID
	workerID := req.conn.ConnId % uint64(len(cm.taskQueue))

	// 将消息发给对应的 taskQueue
	cm.taskQueue[workerID] <- req

}

// StartWorkerPool 启动 worker 工作池
func (cm *Server) StartWorkerPool() {
	// 初始化并启动 worker 工作池
	for i := 0; i < len(cm.taskQueue); i++ {
		// 初始化每个worker的缓存大小
		cm.taskQueue[i] = make(chan *Req, config.GlobalConfig.APP.MaxWorkerTask) 
		// 启动worker
		go cm.StartOneWorker(i, cm.taskQueue[i])
	}
}

// StartOneWorker 启动 worker 的工作流程
func (cm *Server) StartOneWorker(workerID int, taskQueue chan *Req) {
	fmt.Println("Worker ID = ", workerID, " is started.")
	for {
		select {
		case req := <-taskQueue:
			req.f()
		}
	}
}
```

Req 逻辑处理函数Login

```go
func (r *Req) Login() {
	// 检查用户是否已登录 只能防止同一个连接多次调用 Login
	if r.conn.GetUserId() != 0 {
		fmt.Println("[用户登录] 用户已登录")
		return
	}
	
    /*
    type LoginMsg struct {
		// ...
		Token []byte `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"` // token
	}
	*/
    
	// 消息解析 
	loginMsg := new(pb.LoginMsg)
	err := proto.Unmarshal(r.data, loginMsg)
	if err != nil {
		fmt.Println("[用户登录] unmarshal error,err:", err)
		return
	}
    
	// 登录校验
	userClaims, err := utils.AnalyseToken(string(loginMsg.Token))
	if err != nil {
		fmt.Println("[用户登录] AnalyseToken err:", err)
		return
	}
    
	// 检查用户是否已经在其他连接登录
	onlineAddr, err := cache.GetUserOnline(userClaims.UserId)
	if onlineAddr != "" {
		// TODO 更友好的提示
		fmt.Println("[用户登录] 用户已经在其他连接登录")
		r.conn.Stop()
		return
	}
	

	// Redis 存储用户数据 k: userId,  v: grpc地址，方便用户能直接通过这个地址进行 rpc 方法调用
	grpcServerAddr := fmt.Sprintf("%s:%s", config.GlobalConfig.APP.IP, config.GlobalConfig.APP.RPCPort)
    
	err = cache.SetUserOnline(userClaims.UserId, grpcServerAddr)
	if err != nil {
		fmt.Println("[用户登录] 系统错误")
		return
	}

	// 设置 user_id
	r.conn.SetUserId(userClaims.UserId)

	// 加入到 connMap 中
	r.conn.server.AddConn(userClaims.UserId, r.conn)
	/*
	enum ACKType {
  		AT_UnKnow = 0;  // 未知
  		AT_Up = 1 ;   // 服务端回复客户端发来的消息
  		AT_Push = 2;  // 客户端回复服务端发来的消息
  		AT_Login = 3;  // 登录
	}
	
	type ACKMsg struct {
		// ...
		Type     ACKType `protobuf:"varint,1,opt,name=type,proto3,enum=pb.ACKType" json:"type,omitempty"` // 		 收到的是什么类型的 ACK
		ClientId uint64  `protobuf:"varint,2,opt,name=clientId,proto3" json:"clientId,omitempty"`
		Seq      uint64  `protobuf:"varint,3,opt,name=seq,proto3" json:"seq,omitempty"` // 上行消息推送时回复最		 新 seq
	}
	*/
    
	// 回复ACK
	bytes, err := GetOutputMsg(pb.CmdType_CT_ACK, int32(common.OK), &pb.ACKMsg{Type: pb.ACKType_AT_Login})
	if err != nil {
		return
	}

	// 回复发送 Login 请求的客户端
	r.conn.SendMsg(userClaims.UserId, bytes)
}
```

Req 逻辑处理函数HeartBeat

```go
func (r *Req) Heartbeat() {
	// TODO 更新当前用户状态，不做回复
}
```

Req 逻辑处理函数MessageHandler

```go
/*
	MessageHandler 消息处理，处理客户端发送给服务端的消息
	A客户端发送消息给服务端，服务端收到消息处理后发给B客户端
	包括：单聊、群聊
*/
func (r *Req) MessageHandler() {
    /*
    type UpMsg struct {
		// ...
		Msg      *Message `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`  // 消息内容
		ClientId uint64   `protobuf:"varint,2,opt,name=clientId,proto3" json:"clientId,omitempty"` // 保证上行消息可靠性
	}
	*/
	msg := new(pb.UpMsg)
	err := proto.Unmarshal(r.data, msg)

	if err != nil {
		logger.Slog.Error(err.Error())
		return
	}

	// 实现消息可靠性
	if !r.conn.CompareAndIncrClientID(msg.ClientId) {
		return
	}

	if msg.Msg.SenderId != r.conn.GetUserId() {
		return
	}

	// 单聊不能发给自己
	if msg.Msg.SessionType == pb.SessionType_ST_Single && msg.Msg.ReceiverId == r.conn.GetUserId() {
		fmt.Println("[消息处理] 不能将消息发送给自己")
		return
	}

	// 给自己发一份，消息落库但是不推送
	seq, err := SendToUser(msg.Msg, msg.Msg.SenderId)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// 单聊、群聊
	switch msg.Msg.SessionType {
	case pb.SessionType_ST_Single:
		_, err = SendToUser(msg.Msg, msg.Msg.ReceiverId)
	case pb.SessionType_ST_Group:
		err = SendToGroup(msg.Msg)
	default:
		return
	}

	if err != nil {
		fmt.Println("[消息发送] 系统错误")
		return
	}

	// 回复发送上行消息的客户端 ACK
	ackBytes, err := GetOutputMsg(pb.CmdType_CT_ACK, common.OK, &pb.ACKMsg{
		Type:     pb.ACKType_AT_Up,
		ClientId: msg.ClientId, // 回复客户端，当前已 ACK 的消息
		Seq:      seq,          // 回复客户端当前其 seq
	})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// 回复发送 Message 请求的客户端 A
	r.conn.SendMsg(msg.Msg.SenderId, ackBytes)
}
```

组装 [ 服务端 --> 客户端 ] 消息

```go
// GetOutputMsg 组装出下行消息
func GetOutputMsg(cmdType pb.CmdType, code int32, message proto.Message) ([]byte, error) {
	output := &pb.Output{
		Type:    cmdType,
		Code:    code,
		CodeMsg: common.GetErrorMessage(uint32(code), ""),
		Data:    nil,
	}

	if message != nil {
		msgBytes, err := proto.Marshal(message)
		if err != nil {
			logger.Slog.Error("proto Marshal failed", "err", err)
			return nil, err
		}
		output.Data = msgBytes
	}

	bytes, err := proto.Marshal(output)
	if err != nil {
		logger.Slog.Error("proto Marshal failed", "err", err)
		return nil, err
	}
	return bytes, nil
}
```



发送消息 [user --> user] 私聊

```go
// SendToUser 发送消息到好友
func SendToUser(msg *pb.Message, userId uint64) (uint64, error) {
	// 获取接受者 seqId
	seq, err := service.GetUserNextSeq(userId)
	if err != nil {
		logger.Slog.Error("get next seq failed", "err", err)
		return 0, err
	}
	msg.Seq = seq

	// 发给MQ
	if err = kafka.MQ.Publish(model.MessageToProtoMarshal(&model.Message{
		UserID:      userId,
		SenderID:    msg.SenderId,
		SessionType: int8(msg.SessionType),
		ReceiverId:  msg.ReceiverId,
		MessageType: int8(msg.MessageType),
		Content:     msg.Content,
		Seq:         seq,
		SendTime:    time.UnixMilli(msg.SendTime),
	})); err != nil {
		logger.Slog.Error(err.Error())
		return 0, err
	}

	// 如果发给自己的，只落库不进行发送
	if userId == msg.SenderId {
		return seq, nil
	}

	// 组装消息
	bytes, err := GetOutputMsg(pb.CmdType_CT_Message, int32(common.OK), &pb.PushMsg{Msg: msg})
	if err != nil {
		logger.Slog.Error(err.Error())
		return 0, err
	}

	// 进行推送
	return 0, Send(userId, bytes)
}
```

[user] -> [group] 群聊

```go
// SendToGroup 发送消息到群
func SendToGroup(msg *pb.Message) error {
	// 获取群成员信息
	userIds, err := service.GetGroupUser(msg.ReceiverId)
	if err != nil {
		logger.Slog.Error("[SendToGroup] 查询失败", "err", err)
		return err
	}

	// userId set
	m := make(map[uint64]struct{}, len(userIds))
	for _, userId := range userIds {
		m[userId] = struct{}{}
	}

	// 检查当前用户是否属于该群
	if _, ok := m[msg.SenderId]; !ok {
		fmt.Printf("发送消息者: %v 不属于该群: %v\n", msg.ReceiverId, msg.SenderId)
		return nil
	}

	// 自己不再进行推送
	delete(m, msg.SenderId)

	sendUserIds := make([]uint64, 0, len(m))
	for userId := range m {
		sendUserIds = append(sendUserIds, userId)
	}

	// 批量获取 seqId
	sequences, err := service.GetUserNextSeqBatch(sendUserIds)
	if err != nil {
		logger.Slog.Error("[批量获取 sequences 失败]", "err", err)
		return err
	}

	//  k:userid v:该userId的seq
	sendUserSet := make(map[uint64]uint64, len(sequences))
	for i, userId := range sendUserIds {
		sendUserSet[userId] = sequences[i]
	}

	// 创建 Message 对象
	messages := make([]*model.Message, 0, len(m))

	for userId, seq := range sendUserSet {
		messages = append(messages, &model.Message{
			UserID:      userId,
			SenderID:    msg.SenderId,
			SessionType: int8(msg.SessionType),
			ReceiverId:  msg.ReceiverId,
			MessageType: int8(msg.MessageType),
			Content:     msg.Content,
			Seq:         seq,
			SendTime:    time.UnixMilli(msg.SendTime),
		})
	}

	// 发给MQ
	err = kafka.MQ.Publish(model.MessageToProtoMarshal(messages...))
	//err = mq.MessageMQ.Publish(model.MessageToProtoMarshal(messages...))
	if err != nil {
		logger.Slog.Error(err.Error())
		return err
	}
	// 组装消息，进行推送
	userId2Msg := make(map[uint64][]byte, len(m))
	for userId, seq := range sendUserSet {
		msg.Seq = seq
		bytes, err := GetOutputMsg(pb.CmdType_CT_Message, int32(common.OK), &pb.PushMsg{Msg: msg})
		if err != nil {
			fmt.Println("[消息处理] GetOutputMsg Marshal error,err:", err)
			return err
		}
		userId2Msg[userId] = bytes
	}

	// 获取全部网关服务，进行消息推送
	services := etcd.DiscoverySer.GetServices()
	local := fmt.Sprintf("%s:%s", config.GlobalConfig.APP.IP, config.GlobalConfig.APP.RPCPort)
	for _, addr := range services {
		// 如果是本机，进行本地推送
		if local == addr {
			GetServer().SendMessageAll(userId2Msg)
		} else {
			// 如果不是本机，进行远程 RPC 调用
			_, err = rpc.GetServerClient(addr).DeliverMessageAll(context.Background(), &pb.DeliverMessageAllReq{
				ReceiverId_2Data: userId2Msg,
			})

			if err != nil {
				logger.Slog.Error("[DeliverMessageAll]", "err", err)
				return err
			}
		}
	}

	return nil
}
```

>Send 消息转发
>是否在线 ---否---> 不进行推送
>   |
>   是
>   ↓
> 是否在本地 --否--> RPC 调用
>   |
>   是
>   ↓
> 消息发送

```go
func Send(receiverId uint64, bytes []byte) error {
	// 查询是否在线
	rpcAddr, err := cache.GetUserOnline(receiverId)
	if err != nil {
		return err
	}

	// 不在线
	if rpcAddr == "" {
		fmt.Println("用户不在线: ", receiverId)
		return nil
	}
	fmt.Println("用户在线: ", receiverId, "rpcAddr = ", rpcAddr)

	// 查询是否在本地
	conn := ConnManager.GetConn(receiverId)
	if conn != nil {
		// 发送本地消息
		conn.SendMsg(receiverId, bytes)
		fmt.Println("发送消息给本地用户, ReceiverId: ", receiverId)
		return nil
	}

	// rpc 调用
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = rpc.GetServerClient(rpcAddr).DeliverMessage(ctx, &pb.DeliverMessageReq{
		ReceiverId: receiverId,
		Data:       bytes,
	})

	if err != nil {
		logger.Slog.Error("DeliverMessage Failed", "err", err)
		return err
	}

	return nil
}
```









































Req 逻辑处理函数Sync

```go
// Sync  消息同步，拉取离线消息
func (r *Req) Sync() {
    /*
    type SyncInputMsg struct {
		// ...
		Seq uint64 `protobuf:"varint,1,opt,name=seq,proto3" json:"seq,omitempty"` // 客户端已经同步的序列号
	}
	*/
	msg := new(pb.SyncInputMsg)
	err := proto.Unmarshal(r.data, msg)
	if err != nil {
		fmt.Println("[离线消息] unmarshal error,err:", err)
		return
	}

	// 根据 seq 查询，得到比 seq 大的用户消息
	messages, hasMore, err := model.ListByUserIdAndSeq(r.conn.GetUserId(), msg.Seq, model.MessageLimit)
	if err != nil {
        logger.Slog.Error("[model.ListByUserIdAndSeq]", "err", err)
		return
	}
    
	pbMessage := model.MessagesToPB(messages)

	ackBytes, err := GetOutputMsg(pb.CmdType_CT_Sync, int32(common.OK), &pb.SyncOutputMsg{
		Messages: pbMessage,
		HasMore:  hasMore,
	})
	if err != nil {
		fmt.Println("[离线消息] proto.Marshal err:", err)
		return
	}
	// 回复
	r.conn.SendMsg(r.conn.GetUserId(), ackBytes)
}
```



服务端发送消息给客户端

```go
// SendMsg 根据 userId 给相应 socket 发送消息
func (c *Conn) SendMsg(userId uint64, bytes []byte) {
    c.isCloseMutex.RLock()
    defer c.isCloseMutex.RUnlock()

    // 已关闭
    if c.isClose {
       logger.Slog.Info("connection closed when send msg")
       return
    }

    // 根据 userId 找到对应 socket
    conn := c.server.GetConn(userId)
    if conn == nil {
       return
    }

    // 发送
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
```



