package debugprobe

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ProbeConfig is used to configure how the probe will respond
type ProbeConfig struct {
	// If failNext > 0, then fail next probe and decrement.  If failNext < 0, then
	// fail forever.
	FailNext int `json:"failNext" mapstructure:"fail-next"`
}

func (p *Probe) SetConfig(c ProbeConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.c = c
}

func (p *Probe) BindConfig(prefix string, v *viper.Viper, fs *pflag.FlagSet) {
	fs.Int(prefix+"-fail-next", 0, "Fail the next N probes. 0 is succeed forever. <0 is fail forever.")
	v.BindPFlag(prefix+".fail-next", fs.Lookup(prefix+"-fail-next"))
}
