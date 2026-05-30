package common

import (
	"context"
	"fmt"
	"math"

	"github.com/bytedance/gopkg/util/gopool"
)

var relayGoPool gopool.Pool

func init() {
	relayGoPool = gopool.NewPool("gopool.RelayPool", math.MaxInt32, gopool.NewConfig())
	relayGoPool.SetPanicHandler(func(ctx context.Context, i interface{}) {
		// stop_chan 现承载一个幂等的停止函数（关闭广播 stopChan），取代原 chan bool +
		// SafeSendBool 写法，避免与 stream_scanner 中的 close 产生数据竞争。
		if stop, ok := ctx.Value("stop_chan").(func()); ok {
			stop()
		}
		SysError(fmt.Sprintf("panic in gopool.RelayPool: %v", i))
	})
}

func RelayCtxGo(ctx context.Context, f func()) {
	relayGoPool.CtxGo(ctx, f)
}
