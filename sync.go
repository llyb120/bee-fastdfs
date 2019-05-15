package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)
var (
	syncMutex sync.Mutex
)

type SyncFileItem struct {
	Action int `json:"action"`
	FileId string `json:"file_id"`
	Path string `json:"path"`
}

func SyncLoop()  {
	lens := len(config.Peers)
	//只有一个的话，就没必要同步了
	if  lens <= 1 {
		return
	}

	//每隔一段时间随机挑选一个节点同步
	for {
		select {
		case <- time.After(time.Second * 1):
			//对每个节点都有1/2的同步概率
			for i := 0; i < lens; i++  {
				if i == config.Index {
					continue
				}
				if rand.Int() % 2 == 0 {
					//当前节点是否正在同步中
					syncMutex.Lock()
					if !syncing[i] {
						syncing[i] = true
						go syncSingleGroup(i)
					}
					syncMutex.Unlock()

				}
			}

		}
	}
}

func syncSingleGroup(group int)  {
	log.Printf("group %d will be synced", group)

	//读取最后一个同步ID
	id, err := GetLastSyncFileId(group)
	if err != nil {
		log.Printf("group %d sync failed", group)
	}

	getNeedToSyncFileList(group, id)

	log.Println(id)
}

func getNeedToSyncFileList(group int, lastId int)  {
	url := fmt.Sprintf("http://%s/syncList?fromGroupId=%d&lastId=%d", config.Peers[group], config.Index, lastId)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	bs,err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var li []SyncFileItem
	//var li *list.List = list.New()
	err = json.Unmarshal(bs, &li)
	if err != nil {
		return
	}
	log.Println(resp)
}

func setSyncing(i int, enable bool)  {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	if !syncing[i] {
		syncing[i] = enable
	}
}