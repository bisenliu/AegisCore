package metrics

import "github.com/prometheus/client_golang/prometheus/collectors"

// registerRuntimeCollectors 注册系统底层运行指标（如内存占用、垃圾回收耗时和活跃协程数）到监控系统中。
func (p *Provider) registerRuntimeCollectors() error {
	if err := p.Register(collectors.NewGoCollector()); err != nil {
		return err
	}
	return p.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}
