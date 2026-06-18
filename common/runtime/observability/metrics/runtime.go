package metrics

import "github.com/prometheus/client_golang/prometheus/collectors"

func (p *Provider) registerRuntimeCollectors() error {
	if err := p.Register(collectors.NewGoCollector()); err != nil {
		return err
	}
	return p.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}
