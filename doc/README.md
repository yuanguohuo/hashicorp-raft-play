# Raft KV Store 示例 (AI 生成)

这是一个使用 HashiCorp Raft 库实现的分布式一致性键值存储示例。

## Raft 核心概念

### 1. Raft 协议概述

Raft 是一个用于管理复制日志的一致性算法。它通过选举一个 leader 来管理日志复制，确保所有节点的一致性。

### 2. 核心组件

#### FSM (Finite State Machine - 有限状态机)
- **作用**: 应用层状态机，负责实际的数据操作
- **接口**:
  - `Apply(*Log) interface{}`: 当日志条目被提交时调用，必须是确定性的
  - `Snapshot() (FSMSnapshot, error)`: 创建快照，用于日志压缩
  - `Restore(io.ReadCloser) error`: 从快照恢复状态

#### LogStore
- **作用**: 持久化存储 Raft 日志条目
- **实现**: 可以使用内存存储（测试）或持久化存储（生产）

#### StableStore
- **作用**: 存储 Raft 的元数据（如当前任期、投票信息等）
- **实现**: 可以使用内存存储（测试）或持久化存储（生产）

#### SnapshotStore
- **作用**: 存储 FSM 的快照
- **作用**: 用于快速恢复和日志压缩

#### Transport
- **作用**: 节点之间的网络通信层
- **实现**: TCP 传输（生产）或内存传输（测试）

### 3. Raft 节点状态

- **Follower**: 初始状态，接收 leader 的日志条目
- **Candidate**: 选举状态，请求其他节点投票
- **Leader**: 领导状态，处理客户端请求并复制日志
- **Shutdown**: 关闭状态

### 4. 关键流程

#### 选举流程
1. Follower 在超时后变为 Candidate
2. Candidate 向其他节点请求投票
3. 获得多数票的 Candidate 成为 Leader

#### 日志复制流程
1. 客户端向 Leader 发送写请求
2. Leader 将操作追加到本地日志
3. Leader 并行向所有 Follower 复制日志
4. 当多数节点确认后，日志被提交
5. Leader 将日志应用到 FSM
6. Leader 通知 Follower 提交日志

#### 快照流程
1. FSM 创建快照
2. 快照被持久化
3. 旧的日志条目被删除（日志压缩）

## 代码结构

### KVStore (FSM 实现)
- `Apply()`: 处理 set/delete 命令
- `Snapshot()`: 创建当前状态的快照
- `Restore()`: 从快照恢复状态

### RaftKVStore (Raft 包装器)
- 封装 Raft 实例和 FSM
- 提供 Set/Get/Delete 接口
- 处理集群管理（添加节点等）

## 运行示例

### 简单示例（内存传输，3个节点）

```bash
cd examples/kvstore
go run simple_kvstore.go
```

这个示例会：
1. 创建3个节点（使用内存传输，仅用于演示）
2. 引导集群
3. 添加节点到集群
4. 演示基本的 set/get/delete 操作
5. 展示数据在所有节点间的一致性

**输出示例**：
- 显示每个节点的状态（Leader/Follower）
- 展示写操作如何复制到所有节点
- 验证删除操作的一致性

**注意**：此示例使用内存存储和内存传输，仅用于演示和学习。生产环境应使用持久化存储（如 BoltDB）和 TCP 传输。

## 关键特性演示

### 1. 一致性保证
- 所有写操作都通过 Leader
- 操作被复制到多数节点后才提交
- 所有节点最终看到相同的数据

### 2. 容错性
- 3节点集群可以容忍1个节点故障
- 5节点集群可以容忍2个节点故障
- Leader 故障时会自动选举新 Leader

### 3. 线性一致性
- 所有操作都有全局顺序
- 读操作可以看到所有已提交的写操作

## 注意事项

1. **生产环境**: 本示例使用内存存储，仅用于演示。生产环境应使用持久化存储（如 BoltDB）
2. **网络分区**: 在网络分区情况下，只有多数派可以继续服务
3. **性能**: Raft 的性能受网络延迟和磁盘 I/O 影响
4. **配置**: 需要合理配置超时时间、心跳间隔等参数

## 扩展阅读

- [Raft 论文](https://raft.github.io/raft.pdf)
- [HashiCorp Raft 文档](https://godoc.org/github.com/hashicorp/raft)
- [Raft 可视化](https://raft.github.io/)

