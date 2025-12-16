# HashiCorp Raft 核心概念详解 (AI 生成)

## 1. Raft 协议概述

Raft 是一个用于管理复制日志的一致性算法，它将一致性问题分解为三个相对独立的子问题：
- **Leader 选举**：当现有 leader 失效时，必须选出一个新的 leader
- **日志复制**：leader 必须接收来自客户端的日志条目，并复制到集群中的其他节点
- **安全性**：如果任何节点已将某个日志条目应用到其状态机，则其他节点不能对同一索引应用不同的日志条目

## 2. Raft 库的核心组件

### 2.1 FSM (Finite State Machine - 有限状态机)

FSM 是应用层的状态机，负责实际的数据操作。必须实现三个接口：

```go
type FSM interface {
    // Apply: 当日志条目被提交时调用，必须是确定性的
    // 在所有节点上，相同的输入必须产生相同的输出
    Apply(*Log) interface{}
    
    // Snapshot: 创建快照，用于日志压缩和快速恢复
    // 应该快速返回，实际持久化在 Persist 中进行
    Snapshot() (FSMSnapshot, error)
    
    // Restore: 从快照恢复状态
    // 恢复时会清空现有状态
    Restore(io.ReadCloser) error
}
```

**关键点**：
- `Apply` 必须是确定性的（deterministic）
- `Snapshot` 和 `Restore` 必须保证状态一致性
- FSM 的操作是串行的，但快照持久化可以并发

### 2.2 LogStore

**作用**：持久化存储 Raft 日志条目

**关键操作**：
- `StoreLog(log *Log)`: 存储日志条目
- `GetLog(index uint64, log *Log)`: 获取指定索引的日志
- `FirstIndex()` / `LastIndex()`: 获取日志范围
- `DeleteRange(min, max uint64)`: 删除日志范围（用于压缩）

**实现**：
- 测试：`InmemStore`（内存存储）
- 生产：`raft-boltdb` 或 `raft-mdb`（持久化存储）

### 2.3 StableStore

**作用**：存储 Raft 的元数据

**存储内容**：
- 当前任期（CurrentTerm）
- 投票信息（LastVoteTerm, LastVoteCand）

**实现**：
- 测试：`InmemStore`（内存存储）
- 生产：与 LogStore 使用相同的持久化存储

### 2.4 SnapshotStore

**作用**：存储 FSM 的快照

**用途**：
- 日志压缩：删除已快照的旧日志
- 快速恢复：新节点或重启节点可以从快照快速恢复
- 状态同步：落后的节点可以通过快照快速追上

**实现**：
- 测试：`InmemSnapshotStore`（内存存储）
- 生产：`FileSnapshotStore`（文件系统存储）

### 2.5 Transport

**作用**：节点之间的网络通信层

**关键操作**：
- `AppendEntries()`: 复制日志条目
- `RequestVote()`: 请求投票
- `InstallSnapshot()`: 安装快照
- `Consumer()`: 接收 RPC 请求的通道

**实现**：
- 测试：`InmemTransport`（内存传输，用于测试）
- 生产：`TCPTransport`（TCP 网络传输）

## 3. Raft 节点状态

### 3.1 Follower（跟随者）
- **初始状态**：所有节点启动时都是 Follower
- **职责**：
  - 接收 Leader 的日志条目
  - 响应 Leader 的心跳
  - 参与选举投票

### 3.2 Candidate（候选者）
- **转换条件**：Follower 在超时后未收到 Leader 心跳
- **职责**：
  - 发起选举，请求其他节点投票
  - 如果获得多数票，成为 Leader

### 3.3 Leader（领导者）
- **转换条件**：Candidate 获得多数票
- **职责**：
  - 处理客户端请求
  - 复制日志到所有 Follower
  - 定期发送心跳保持领导地位

### 3.4 Shutdown（关闭）
- **转换条件**：调用 `Shutdown()` 方法
- **状态**：节点完全停止

## 4. 关键流程详解

### 4.1 选举流程

```
1. Follower 超时（ElectionTimeout）
   ↓
2. 转换为 Candidate，增加任期（Term）
   ↓
3. 向所有节点发送 RequestVote RPC
   ↓
4. 等待投票响应
   ↓
5a. 获得多数票 → 成为 Leader
5b. 收到更高任期的消息 → 转为 Follower
5c. 超时未获得多数票 → 重新选举
```

**关键点**：
- 每个任期最多只能有一个 Leader
- 需要多数票（quorum）才能成为 Leader
- 3节点集群需要2票，5节点集群需要3票

### 4.2 日志复制流程

```
1. 客户端向 Leader 发送写请求
   ↓
2. Leader 将操作追加到本地日志（未提交）
   ↓
3. Leader 并行向所有 Follower 发送 AppendEntries RPC
   ↓
4. Follower 验证日志，如果通过则追加到本地日志
   ↓
5. Follower 返回成功响应
   ↓
6. Leader 收到多数节点的成功响应
   ↓
7. Leader 提交日志（标记为已提交）
   ↓
8. Leader 将日志应用到 FSM
   ↓
9. Leader 通知 Follower 提交日志
   ↓
10. Follower 应用日志到 FSM
```

**关键点**：
- 只有 Leader 可以接受客户端写请求
- 日志必须复制到多数节点才能提交
- 提交的日志保证不会丢失（除非多数节点同时故障）

### 4.3 快照流程

```
1. FSM 状态增长到阈值
   ↓
2. Raft 调用 FSM.Snapshot()
   ↓
3. FSM 返回 FSMSnapshot（包含状态指针）
   ↓
4. Raft 调用 FSMSnapshot.Persist()
   ↓
5. FSM 将状态序列化到 SnapshotSink
   ↓
6. Raft 保存快照元数据
   ↓
7. Raft 删除已快照的旧日志（日志压缩）
```

**关键点**：
- 快照用于防止日志无限增长
- 快照必须包含完整的 FSM 状态
- 快照和日志应用可以并发进行

## 5. 一致性保证

### 5.1 线性一致性（Linearizability）

Raft 保证线性一致性：
- 所有操作都有全局顺序
- 读操作可以看到所有已提交的写操作
- 操作看起来是原子的

### 5.2 容错性

- **3节点集群**：可以容忍1个节点故障
- **5节点集群**：可以容忍2个节点故障
- **多数派原则**：只有多数节点可用时，集群才能继续服务

### 5.3 安全性保证

1. **选举安全性**：每个任期最多只有一个 Leader
2. **日志匹配**：如果两个日志条目有相同的索引和任期，则它们包含相同的命令
3. **领导者完整性**：如果某个日志条目在某个任期被提交，则它会在所有更高任期的 Leader 的日志中出现
4. **状态机安全性**：如果某个节点已将某个索引的日志应用到状态机，则其他节点不能对同一索引应用不同的日志条目

## 6. 使用示例

### 6.1 创建 Raft 节点

```go
// 1. 创建 FSM
fsm := NewKVStore()

// 2. 创建配置
config := raft.DefaultConfig()
config.LocalID = raft.ServerID("node1")

// 3. 创建存储
logStore := raft.NewInmemStore()
stableStore := raft.NewInmemStore()
snapshotStore := raft.NewInmemSnapshotStore()

// 4. 创建传输
transport := raft.NewInmemTransport("node1")

// 5. 创建 Raft 实例
r, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
```

### 6.2 引导集群

```go
configuration := raft.Configuration{
    Servers: []raft.Server{
        {ID: "node1", Address: "node1"},
    },
}
future := r.BootstrapCluster(configuration)
```

### 6.3 添加节点

```go
future := r.AddVoter("node2", "node2", 0, 10*time.Second)
```

### 6.4 应用操作

```go
// 只有 Leader 可以应用操作
if r.State() == raft.Leader {
    cmd := Command{Op: "set", Key: "key1", Value: "value1"}
    data, _ := json.Marshal(cmd)
    future := r.Apply(data, 10*time.Second)
    err := future.Error()
    response := future.Response()
}
```

## 7. 最佳实践

### 7.1 配置建议

- **集群大小**：推荐3或5个节点
- **超时设置**：
  - `HeartbeatTimeout`: 50-200ms
  - `ElectionTimeout`: 200-1000ms
  - `CommitTimeout`: 50ms

### 7.2 性能优化

- 使用批量应用（`BatchingFSM`）提高吞吐量
- 合理设置快照阈值和间隔
- 使用流水线复制提高性能

### 7.3 生产环境注意事项

1. **持久化存储**：必须使用持久化的 LogStore 和 StableStore
2. **网络分区**：在网络分区时，只有多数派可以继续服务
3. **监控**：监控 Leader 状态、日志复制延迟、快照大小等
4. **备份**：定期备份快照和日志

## 8. 常见问题

### Q: 为什么只有 Leader 可以接受写请求？
A: 为了保证一致性，所有写操作必须通过 Leader 复制到多数节点。如果允许 Follower 接受写请求，会导致数据不一致。

### Q: 如何处理网络分区？
A: 在网络分区时，只有包含多数节点的分区可以继续服务。少数派分区会停止服务，等待网络恢复。

### Q: 如何保证读操作的线性一致性？
A: 可以从 Leader 读取（保证看到最新数据），或者使用 ReadIndex/LeaseRead 优化（允许从 Follower 读取但保证一致性）。

### Q: 日志会无限增长吗？
A: 不会。通过快照机制，旧的日志会被压缩删除。快照包含完整的 FSM 状态，可以替代旧日志。

## 9. 参考资料

- [Raft 论文](https://raft.github.io/raft.pdf)
- [Raft 可视化](https://raft.github.io/)
- [HashiCorp Raft 文档](https://godoc.org/github.com/hashicorp/raft)

