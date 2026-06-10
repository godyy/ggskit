package config

import (
	"github.com/BurntSushi/toml"
	pkgerrors "github.com/pkg/errors"
)

// LoadFile 读取配置文件.
func LoadFile(cfg any, file string) error {
	if _, err := toml.DecodeFile(file, cfg); err != nil {
		return pkgerrors.WithMessage(err, "decode file")
	}

	if i, ok := cfg.(ConfigWithFlags); ok {
		if err := i.ApplyFlags(); err != nil {
			return err
		}
	}

	return nil
}
