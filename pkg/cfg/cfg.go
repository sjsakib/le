package cfg

import (
	"errors"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type StaticSiteMode string

const (
	StaticSiteModeOff  StaticSiteMode = "off"
	StaticSiteModeAuto StaticSiteMode = "auto"
)

func (m *StaticSiteMode) String() string {
	return string(*m)
}
func (m *StaticSiteMode) Set(s string) error {
	switch s {
	case "false":
		*m = StaticSiteModeOff
	case "auto":
		*m = StaticSiteModeAuto
	default:
		return errors.New("invalid static site mode")
	}
	return nil
}

func (m *StaticSiteMode) Type() string {
	return "enum"
}

type Config struct {
	Dir  string
	Port int

	StaticSiteMode StaticSiteMode
	IsSPA          bool

	LogPath string
}

func Load() (*Config, error) {
	viper.SetConfigName("leconf")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.le")

	viper.ReadInConfig()

	f := flag.CommandLine
	f.StringP("dir", "d", ".", "Directory to serve files from")
	f.IntP("port", "p", 8080, "Port to run the file server on")

	var staticSiteMode StaticSiteMode = "auto"
	f.Var(&staticSiteMode, "static-site", "Static site mode: auto|false")

	f.StringP("log-path", "l", "", "If a path is provided, logs will be written to that directory")

	err := viper.BindPFlags(f)

	flag.Parse()

	if err != nil {
		return nil, err
	}

	c := &Config{
		Dir:            viper.GetString("dir"),
		Port:           viper.GetInt("port"),
		StaticSiteMode: StaticSiteMode(viper.GetString("static-site")),
		IsSPA:          viper.GetBool("spa"),
		LogPath:        viper.GetString("log-path"),
	}

	return c, nil
}
