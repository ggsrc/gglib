# GGLib - 灵活的 gRPC Server 基础库

GGLib 是一个设计灵活的 gRPC server 基础库，支持动态管理多种资源类型。该库提供了统一的资源管理接口，可以轻松添加和管理数据库、Redis、Kafka 等各种资源。

## 特性

- 🚀 **灵活的 gRPC Server**: 基于 Google gRPC 框架，支持高性能的 RPC 通信
- 🔧 **统一资源管理**: 提供统一的资源接口，支持任意类型的资源
- ⚙️ **函数式配置选项**: 使用 `WithXXX` 模式的配置选项，提供灵活的资源配置
- 🏥 **健康检查**: 内置资源健康检查机制
- 🔄 **生命周期管理**: 自动管理资源的启动、停止和清理
- 🛡️ **线程安全**: 所有操作都是线程安全的
- 📦 **易于扩展**: 简单的接口设计，易于添加新的资源类型
- 🏷️ **标签和元数据**: 支持为资源添加标签和元数据，便于管理和监控
- 🧪 **完整测试**: 包含完整的单元测试

## 架构设计

### 核心接口

```go
// Resource 定义了所有资源必须实现的接口
type Resource interface {
    Name() string                    // 资源唯一名称
    Start(ctx context.Context) error // 启动资源
    Stop(ctx context.Context) error  // 停止资源
    HealthCheck(ctx context.Context) error // 健康检查
    IsRunning() bool                 // 运行状态
}
```

### 资源管理器

```go
// ResourceManager 管理多个资源的生命周期
type ResourceManager interface {
    AddResource(resource Resource) error
    RemoveResource(name string) error
    GetResource(name string) (Resource, bool)
    ListResources() []Resource
    StartAll(ctx context.Context) error
    StopAll(ctx context.Context) error
    HealthCheckAll(ctx context.Context) map[string]HealthStatus
}
```

## 快速开始

### 安装依赖

```bash
go mod tidy
```

### 基本使用

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/gglib/gglib/pkg/resource"
    "github.com/gglib/gglib/pkg/server"
)

func main() {
    // 创建服务器配置选项
    serverOpts, err := resource.NewServerOptions(
        resource.WithName("my-grpc-server"),
        resource.WithPort(8080),
        resource.WithAddress("0.0.0.0"),
        resource.WithMetrics(true),
        resource.WithRecovery(true),
    )
    if err != nil {
        log.Fatalf("Failed to create server options: %v", err)
    }
    
    // 创建 gRPC 服务器
    srv := server.NewServer(serverOpts)
    
    // 创建数据库配置选项
    dbOpts, err := resource.NewDatabaseOptions(
        resource.WithName("main-database"),
        resource.WithDriverName("mysql"),
        resource.WithDataSourceName("user:password@tcp(localhost:3306)/testdb"),
        resource.WithMaxOpenConns(25),
        resource.WithMaxIdleConns(5),
        resource.WithConnMaxLifetime(5*time.Minute),
        resource.WithTag("type", "database"),
    )
    if err != nil {
        log.Fatalf("Failed to create database options: %v", err)
    }
    
    // 添加数据库资源
    dbResource := resource.NewDatabaseResource(dbOpts)
    srv.AddResource(dbResource)
    
    // 创建 Redis 配置选项
    redisOpts, err := resource.NewRedisOptions(
        resource.WithName("cache-redis"),
        resource.WithAddr("localhost:6379"),
        resource.WithRedisPoolSize(10),
        resource.WithDialTimeout(5*time.Second),
        resource.WithTag("type", "cache"),
    )
    if err != nil {
        log.Fatalf("Failed to create Redis options: %v", err)
    }
    
    // 添加 Redis 资源
    redisResource := resource.NewRedisResource(redisOpts)
    srv.AddResource(redisResource)
    
    // 创建 Kafka 配置选项
    kafkaOpts, err := resource.NewKafkaOptions(
        resource.WithName("message-queue"),
        resource.WithBrokers([]string{"localhost:9092"}),
        resource.WithKafkaDialTimeout(10*time.Second),
        resource.WithTag("type", "message-queue"),
    )
    if err != nil {
        log.Fatalf("Failed to create Kafka options: %v", err)
    }
    
    // 添加 Kafka 资源
    kafkaResource := resource.NewKafkaResource(kafkaOpts)
    srv.AddResource(kafkaResource)
    
    // 启动服务器
    ctx := context.Background()
    if err := srv.Start(ctx); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
    
    // 检查健康状态
    health := srv.HealthCheck(ctx)
    for name, status := range health {
        log.Printf("%s: %s", name, status.Status)
    }
}
```

## 内置资源类型

### 数据库资源

```go
// 创建数据库资源
dbResource := resource.NewDatabaseResource("main-db", "user:password@tcp(localhost:3306)/testdb")
srv.AddResource(dbResource)

// 获取数据库连接
if db, ok := srv.GetResource("main-db"); ok {
    if dbResource, ok := db.(*resource.DatabaseResource); ok {
        sqlDB := dbResource.GetDB()
        // 使用 sqlDB 进行数据库操作
    }
}
```

### Redis 资源

```go
// 从 URL 创建 Redis 资源
redisResource := resource.NewRedisResourceFromURL("cache-redis", "redis://localhost:6379")
srv.AddResource(redisResource)

// 或者从配置创建
options := &redis.Options{
    Addr: "localhost:6379",
    DB:   0,
}
redisResource := resource.NewRedisResource("cache-redis", options)
srv.AddResource(redisResource)

// 获取 Redis 客户端
if redis, ok := srv.GetResource("cache-redis"); ok {
    if redisResource, ok := redis.(*resource.RedisResource); ok {
        client := redisResource.GetClient()
        // 使用 client 进行 Redis 操作
    }
}
```

### Kafka 资源

```go
// 创建 Kafka 资源
kafkaResource := resource.NewKafkaResource("message-queue", []string{"localhost:9092"}, "events")
srv.AddResource(kafkaResource)

// 获取 Kafka 连接
if kafka, ok := srv.GetResource("message-queue"); ok {
    if kafkaResource, ok := kafka.(*resource.KafkaResource); ok {
        conn := kafkaResource.GetConn()
        // 使用 conn 进行 Kafka 操作
    }
}
```

## 自定义资源

你可以轻松创建自定义资源类型：

```go
type CustomResource struct {
    name    string
    running bool
    mu      sync.RWMutex
}

func (c *CustomResource) Name() string {
    return c.name
}

func (c *CustomResource) Start(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.running = true
    return nil
}

func (c *CustomResource) Stop(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.running = false
    return nil
}

func (c *CustomResource) HealthCheck(ctx context.Context) error {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if !c.running {
        return fmt.Errorf("resource is not running")
    }
    return nil
}

func (c *CustomResource) IsRunning() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.running
}

// 使用自定义资源
customResource := &CustomResource{name: "my-custom-resource"}
srv.AddResource(customResource)
```

## 健康检查

库提供了完整的健康检查机制：

```go
// 检查所有资源的健康状态
health := srv.HealthCheck(ctx)
for name, status := range health {
    fmt.Printf("Resource: %s, Status: %s, Message: %s\n", 
        name, status.Status, status.Message)
}
```

健康状态包含以下信息：
- `Name`: 资源名称
- `Status`: 状态（healthy/unhealthy/unknown）
- `Message`: 状态描述
- `Timestamp`: 检查时间戳
- `Details`: 详细信息（可选）

## 运行示例

### 基本示例

```bash
cd examples/basic
go run main.go
```

### 自定义资源示例

```bash
cd examples/custom_resource
go run main.go
```

## 测试

运行所有测试：

```bash
go test ./...
```

运行特定包的测试：

```bash
go test ./pkg/resource
go test ./pkg/server
```

## 项目结构

```
gglib/
├── pkg/
│   ├── resource/
│   │   ├── interface.go      # 核心资源接口定义
│   │   ├── manager.go        # 资源管理器接口定义
│   │   ├── database.go       # 数据库资源接口定义
│   │   ├── redis.go          # Redis 资源接口定义
│   │   ├── kafka.go          # Kafka 资源接口定义
│   │   ├── options.go        # 配置选项定义
│   │   └── interface_test.go # 接口测试文件
│   └── server/
│       └── interface.go      # gRPC 服务器接口定义
├── examples/
│   ├── basic/                # 基本使用示例
│   └── options/              # 配置选项使用示例
├── docs/
│   └── API.md                # API 文档
├── go.mod                    # Go 模块文件
└── README.md                 # 项目说明文档
```

## 接口设计

### 核心接口

1. **Resource 接口**: 所有资源的基础接口，定义了 `Start`、`Stop`、`HealthCheck`、`IsRunning` 等基本方法
2. **Manager 接口**: 资源管理器接口，用于管理多个资源的生命周期
3. **DatabaseResource 接口**: 数据库资源接口，扩展了基础资源接口，提供数据库特定功能
4. **RedisResource 接口**: Redis 资源接口，提供 Redis 连接和操作功能
5. **KafkaResource 接口**: Kafka 资源接口，提供消息队列功能
6. **Server 接口**: gRPC 服务器接口，集成了资源管理功能

### 扩展接口

- **ManagerWithStats**: 带统计信息的资源管理器
- **ManagerWithEvents**: 支持事件监听的资源管理器
- **ManagerWithDependencies**: 支持资源依赖管理的资源管理器
- **DatabaseResourceWithMigration**: 支持数据库迁移的数据库资源
- **DatabaseResourceWithBackup**: 支持数据库备份的数据库资源
- **RedisResourceWithPubSub**: 支持发布订阅的 Redis 资源
- **RedisResourceWithLua**: 支持 Lua 脚本的 Redis 资源
- **RedisResourceWithPipeline**: 支持管道的 Redis 资源
- **KafkaResourceWithAdmin**: 支持管理功能的 Kafka 资源

## 依赖

- `google.golang.org/grpc`: gRPC 框架
- `github.com/redis/go-redis/v9`: Redis 客户端
- `github.com/segmentio/kafka-go`: Kafka 客户端
- `github.com/stretchr/testify`: 测试框架

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
