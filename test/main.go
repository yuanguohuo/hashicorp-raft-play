package main

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/raft"

	"goraft/test/node"
)

func main() {
	log.Println("=== Raft KV Store 演示 ===")

	members := []raft.Server{
		{ID: "node0", Address: "127.0.0.1:8090"},
		{ID: "node1", Address: "127.0.0.1:8091"},
		{ID: "node2", Address: "127.0.0.1:8092"},
	}

	rnode0, err := node.NewRaftNode(members, 0, "0.0.0.0:8090", false)
	if err != nil {
		log.Fatalf("创建 rnode0 失败: %v", err)
	}

	rnode1, err := node.NewRaftNode(members, 1, "0.0.0.0:8091", false)
	if err != nil {
		log.Fatalf("创建 rnode1 失败: %v", err)
	}

	rnode2, err := node.NewRaftNode(members, 2, "0.0.0.0:8092", false)
	if err != nil {
		log.Fatalf("创建 rnode2 失败: %v", err)
	}

	log.Println("等待选举...")
	var leaderNode *node.RaftNode = nil

	for range 10 {
		if rnode0.IsLeader() {
			leaderNode = rnode0
			break
		} else if rnode1.IsLeader() {
			leaderNode = rnode1
			break
		} else if rnode2.IsLeader() {
			leaderNode = rnode2
			break
		}

		log.Println("waiting for election ...")
		time.Sleep(3 * time.Second)
	}

	if leaderNode == nil {
		log.Fatal("election failed")
	}

	// 演示基本操作
	log.Println("\n=== 演示基本操作 ===")

	// 设置一些键值对
	log.Println("\n1. 设置键值对:")
	log.Println("   Set key1=value1")
	if err := leaderNode.Set("key1", "value1"); err != nil {
		log.Printf("   失败: %v", err)
	}

	log.Println("   Set key2=value2")
	if err := leaderNode.Set("key2", "value2"); err != nil {
		log.Printf("   失败: %v", err)
	}

	log.Println("   Set key3=value3")
	if err := leaderNode.Set("key3", "value3"); err != nil {
		log.Printf("   失败: %v", err)
	}

	// 等待数据复制
	time.Sleep(500 * time.Millisecond)

	// 从所有节点读取值
	log.Println("\n2. 从所有节点读取值:")
	stores := []*node.RaftNode{rnode0, rnode1, rnode2}
	for i, store := range stores {
		log.Printf("   Node%d状态: %s", i+1, store.State())
		val, ok := store.Get("key1")
		if ok {
			log.Printf("   Node%d: key1=%s", i+1, val)
		}
		val, ok = store.Get("key2")
		if ok {
			log.Printf("   Node%d: key2=%s", i+1, val)
		}
		val, ok = store.Get("key3")
		if ok {
			log.Printf("   Node%d: key3=%s", i+1, val)
		}
	}

	// 删除键
	log.Println("\n3. 删除key1:")
	if err := leaderNode.Delete("key1"); err != nil {
		log.Printf("   失败: %v", err)
	}

	// 等待删除操作复制
	time.Sleep(500 * time.Millisecond)

	// 验证删除
	log.Println("\n4. 验证删除:")
	for i, store := range stores {
		val, ok := store.Get("key1")
		if ok {
			log.Printf("   Node%d: key1仍然存在: %s", i+1, val)
		} else {
			log.Printf("   Node%d: key1已删除", i+1)
		}
	}

	// 显示所有数据
	log.Println("\n5. 所有节点的完整数据:")
	for i, store := range stores {
		all := store.GetAll()
		log.Printf("   Node%d: %v", i+1, all)
	}

	for i := range 100 {
		leaderNode.Set(fmt.Sprintf("zhuhai_key_%d", i), fmt.Sprintf("zhuhai_val_%d", i))
	}

	time.Sleep(time.Second * 5)

	log.Println("\n=== 演示完成 ===")
	log.Println("这展示了Raft如何保证所有节点的一致性！")
}
