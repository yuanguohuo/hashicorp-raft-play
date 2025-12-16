# Create a Raft Cluster

- node0

```
./bin/raft_kv_store --raft-advertise-addrs r0@127.0.0.1:7890,r1@127.0.0.1:7891,r2@127.0.0.1:7892 --local-index 0 --raft-bind-addr 0.0.0.0:7890 --http-bind-addr 0.0.0.0:8890 --mode create
```

- node1

```
./bin/raft_kv_store --raft-advertise-addrs r0@127.0.0.1:7890,r1@127.0.0.1:7891,r2@127.0.0.1:7892 --local-index 1 --raft-bind-addr 0.0.0.0:7891 --http-bind-addr 0.0.0.0:8891 --mode create
```


- node2

```
./bin/raft_kv_store --raft-advertise-addrs r0@127.0.0.1:7890,r1@127.0.0.1:7891,r2@127.0.0.1:7892 --local-index 2 --raft-bind-addr 0.0.0.0:7892 --http-bind-addr 0.0.0.0:8892  --mode create
```


# Add a Member

- node3

```
./bin/raft_kv_store --local-advertise-addr r3@127.0.0.1:7893 --raft-bind-addr 0.0.0.0:7893 --http-bind-addr 0.0.0.0:8893 --leader-http-bind-addr 0.0.0.0:8891 --mode join
```

进程启动Raft之后，会向Leader发HTTP请求，请求Leader把自己加到Raft Cluster中。为了简单，我们用参数`--leader-http-bind-addr`告诉进程Leader HTTP地址。当然，也可以轮训。


# Remove a Member

管理员向Leader发HTTP请求

```
curl -X GET "http://0.0.0.0:8891/evict?id=r1"
```

被remove的可以是任何一个member，包括Leader。


# Raft Advertise Addr vs. Raft Bind Addr


- Bind Addr：节点在本地网络接口上监听的地址，用于接收来自其他节点的连接请求。

- Advertise Addr：节点对外公布的地址，其他节点会用这个地址来连接本节点。

两者为什么会不同呢？在 NAT 环境下（如云服务器、家庭网络），节点的内网地址（bindAddr）对其他节点不可见。例如：节点在 AWS EC2 实例中，bindAddr 是内网IP 172.31.16.10，但 advertiseAddr 应该是 EC2 的公网IP 54.200.100.200。**当然，两者也可以相同**。

在本例中，为了区分两者，使用`0.0.0.0:{port}`作bindAddr；使用`127.0.0.1:{port}`作advertiseAddr。
