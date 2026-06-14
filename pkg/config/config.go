package config

import "github.com/spf13/viper"
import flag "github.com/spf13/pflag"

type Config struct {
	Dir  string
	Port int
}

func Load() (*Config, error) {
	viper.SetConfigName("leconf")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.le")

	viper.ReadInConfig()

	f := flag.CommandLine
	f.String("dir", ".", "Directory to serve files from")
	f.Int("port", 8080, "Port to run the file server on")

	err := viper.BindPFlags(f)

	flag.Parse()

	if err != nil {
		return nil, err
	}

	c := &Config{
		Dir:  viper.GetString("dir"),
		Port: viper.GetInt("port"),
	}

	return c, nil
}
