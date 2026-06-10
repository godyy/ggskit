package mongobd

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/godyy/glog"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// BDConfig BD 配置.
type BDConfig struct {
	// Client MongoDB 客户端
	Client *mongo.Client

	// Workers 后台worker数量
	// 默认值为 runtime.NumCPU().
	Wokers int

	// MaxWorkerOps 每个后台worker最大操作数.
	// 默认值为 1000.
	MaxWorkerOps int

	// DefExecTimeout 默认执行超时时间.
	// 当操作符携带的上下文没有指定超时时间时, 在执行该操作符时
	// 将使用 DefExecTimeout 作为超时时间.
	// 默认值为 5 秒.
	DefExecTimeout time.Duration

	// Logger 日志记录器
	Logger glog.Logger
}

func (c *BDConfig) check() error {
	if c.Client == nil {
		return errors.New("client is nil")
	}
	if c.Wokers <= 0 {
		c.Wokers = runtime.NumCPU()
	}
	if c.MaxWorkerOps <= 0 {
		c.MaxWorkerOps = 1000
	}
	if c.DefExecTimeout <= 0 {
		c.DefExecTimeout = 5 * time.Second
	}
	if c.Logger == nil {
		return errors.New("logger is nil")
	}
	return nil
}

// BD MongoDB 操作后台.
// 负责接收操作符(OP)并将其分发给对应的后台worker处理.
type BD struct {
	client         *mongo.Client  // MongoDB 客户端
	defExecTimeout time.Duration  // 默认执行超时时间
	wks            []*worker      // 后台worker列表
	wg             sync.WaitGroup // 等待组, 用于等待所有worker完成
	logger         glog.Logger    // 日志记录器
}

// NewBD 构造BD.
func NewBD(cfg BDConfig) (*BD, error) {
	// 检查配置.
	if err := cfg.check(); err != nil {
		return nil, err
	}

	// 构造BD
	bd := &BD{
		client:         cfg.Client,
		defExecTimeout: cfg.DefExecTimeout,
		wks:            make([]*worker, cfg.Wokers),
		logger:         cfg.Logger,
	}

	// 启动后台worker.
	bd.wg.Add(cfg.Wokers)
	for i := 0; i < cfg.Wokers; i++ {
		wk := &worker{
			BD:  bd,
			ops: make(chan Op, cfg.MaxWorkerOps),
		}
		bd.wks[i] = wk
		go wk.run()
	}

	return bd, nil
}

// Add 添加操作符到后台队列.
// op 将被分发给对应的后台worker处理, op 处理完成后会通过 done 通道返回.
// 回调函数 callback 会被添加到 op 中, 但需要在通过 done 通道获取 op 后调用.
func (bd *BD) Add(hashKey any, op Op, callback func(Op), done chan Op) error {
	if op == nil {
		return errors.New("op is nil")
	}
	if done == nil {
		return errors.New("done is nil")
	}
	opBase := op.base()
	opBase.callback = callback
	opBase.done = done
	hc := getHashCode(hashKey)
	wk := bd.wks[hc%uint64(len(bd.wks))]
	select {
	case wk.ops <- op:
	case <-opBase.ctx.Done():
		return opBase.ctx.Err()
	}
	return nil
}

// Exec 同步执行操作符.
// 等待操作符处理完成并返回结果.
func (bd *BD) Exec(hashKey any, op Op) error {
	done := make(chan Op, 1)
	if err := bd.Add(hashKey, op, nil, done); err != nil {
		return err
	}
	<-done
	return op.Err()
}

// Stop 停止所有后台worker.
// 等待所有操作符处理完成后返回.
func (bd *BD) Stop() {
	for _, wk := range bd.wks {
		wk.stop()
	}
	bd.wks = nil
	bd.wg.Wait()
}

// getHashCode 通过哈希键计算哈希值.
func getHashCode(hashKey any) uint64 {
	switch o := hashKey.(type) {
	case int8:
		return uint64(o)
	case int16:
		return uint64(o)
	case int32:
		return uint64(o)
	case int64:
		return uint64(o)
	case int:
		return uint64(o)
	case uint8:
		return uint64(o)
	case uint16:
		return uint64(o)
	case uint32:
		return uint64(o)
	case uint64:
		return o
	case uint:
		return uint64(o)
	case string:
		// 使用 FNV-1a 算法高效计算字符串哈希
		var h uint64 = 14695981039346656037 // FNV 偏移基础
		for i := 0; i < len(o); i++ {
			h ^= uint64(o[i])
			h *= 1099511628211 // FNV 质数
		}
		return h
	case interface{ HashCode() uint64 }:
		return o.HashCode()
	default:
		panic(fmt.Errorf("invalid hashKey type: %s", reflect.TypeOf(hashKey).String()))
	}
}
